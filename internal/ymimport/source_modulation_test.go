package ymimport

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSourceVibratoMatchesNativePeriodDeltasAndTurningPointHold(t *testing.T) {
	m := sourceModulation{}
	m.instrument([]byte{0, 0, 1, 2, 0, 1})
	m.vibrato(true)
	m.note(0, true)
	// Original 68000 calls on a 477-period tone, source note-table index 36.
	want := []int16{-2, -4, -4, -2, 0, 2, 4, 2, 0, -2, -4, -4, -2, 0, 2, 4}
	for frame, expected := range want {
		if got := m.periodDelta(36); got != expected {
			t.Fatalf("native triangle call %d: delta=%d, want %d", frame, got, expected)
		}
	}
}

func TestSourceVibratoDelayMatchesNativeFirstActiveCall(t *testing.T) {
	m := sourceModulation{}
	m.instrument([]byte{0, 0, 1, 2, 3, 1})
	m.vibrato(true)
	m.note(3, true)
	for frame, expected := range []int16{0, 0, 0, -2, -4, -4, -2, 0, 2, 4, 2, 0} {
		if got := m.periodDelta(36); got != expected {
			t.Fatalf("native delayed call %d: delta=%d, want %d", frame, got, expected)
		}
	}
}

func TestRepeatedZeroArpeggioRetainsNativeVibrato(t *testing.T) {
	score := SourceScore{Player: madMaxClassic, Rate: 50, Speed: 3, Frames: 30,
		Instruments: []SourceInstrument{{ID: 0, Settings: []byte{0, 0, 1, 3, 6, 1}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{Values: []int{0, 0, 0, 0}, StepFrames: 1, Repeat: 3}}},
		Events:      []SourceEvent{{Channel: 0, Frame: 0, Note: 40, Instrument: 0, Retrigger: true}},
		Controls:    []SourceControl{{Channel: 0, Frame: 0, Opcode: 0x82}, {Channel: 0, Frame: 0, Opcode: 0xc0}},
	}
	p, _, err := SourceProject(score, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	// Classic native calls on note 40: six delayed calls, then the triangle
	// with a low-end hold and octave-scaled 8-period steps.
	want := []int{0, 0, 0, 0, 0, 0, -8, -16, -24, -24, -16, -8, 0, 8, 16, 24, 16, 8, 0, -8}
	for frame, delta := range want {
		e.Tick()
		period := int(e.Registers[0]) | int(e.Registers[1])<<8
		if period != int(replay.TonePeriod(40))+delta {
			t.Fatalf("repeated-zero arpeggio lost native vibrato at frame %d: period=%d", frame, period)
		}
	}
}

func TestSourceSlideInitialDelayAndSignedAccumulatorMatchNativeCalls(t *testing.T) {
	for _, test := range []struct {
		delay byte
		want  []int16
	}{
		{1, []int16{-6, -12, -18, -24, -30, -36, -42, -48}},
		{3, []int16{0, 0, -6, -12, -18, -24, -30, -36}},
	} {
		m := sourceModulation{}
		m.pitchSlide(-6, test.delay)
		for frame, expected := range test.want {
			if got := m.periodDelta(36); got != expected {
				t.Fatalf("native slide delay %d call %d: %d, want %d", test.delay, frame, got, expected)
			}
		}
	}
}

func TestSourceControlsRetainOriginalExecutionFramesWithoutChangingNotes(t *testing.T) {
	b := sourceFixture()
	copy(b[0x2500:], []byte{0x82, 0xc3, 0xe1, 60, 0x84, 250, 1, 0x8e, 62, 0x81, 0x91, 3, 64, 0x87})
	s, err := DecodeSource(b, 0, 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Controls) != 12 {
		t.Fatalf("controls were not retained on all three voices: %v", s.Controls)
	}
	for channel := 0; channel < 3; channel++ {
		var controls []SourceControl
		for _, c := range s.Controls {
			if c.Channel == channel && c.Opcode < 0xc0 {
				controls = append(controls, c)
			}
		}
		if len(controls) != 3 || controls[0].Frame != 0 || controls[0].Opcode != 0x82 || controls[1].Frame != 6 || controls[1].Opcode != 0x84 || controls[2].Frame != 12 || controls[2].Opcode != 0x81 {
			t.Fatalf("original effect timing changed: %v", controls)
		}
	}
}

func TestSourceProjectRendersNativeModulationAcrossSequenceAndPatternBoundaries(t *testing.T) {
	score := SourceScore{Rate: 50, Speed: 3, Frames: 140,
		Instruments: []SourceInstrument{{ID: 0, Settings: []byte{0, 0, 1, 2, 0, 1}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{Values: []int{0}, StepFrames: 1, Repeat: 0}}},
		Events:      []SourceEvent{{Channel: 0, Frame: 0, Note: 60, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 64, Note: 60, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 90, Note: 60, Instrument: 0}},
		Controls:    []SourceControl{{Channel: 0, Frame: 0, Opcode: 0x82}, {Channel: 0, Frame: 0, Opcode: 0xc0}, {Channel: 0, Frame: 90, Opcode: 0x84, Operand: []byte{250, 1}}},
	}
	p, report, err := SourceProject(score, 0, 140)
	if err != nil {
		t.Fatal(err)
	}
	if report.ModulationSegments == 0 {
		t.Fatal("source effects never reached editable sequences")
	}
	raw, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	p.Bank, err = native.DecodeVoiceBank(raw)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	triangle := []int16{-2, -4, -4, -2, 0, 2, 4, 2, 0}
	for frame := 0; frame < 140; frame++ {
		e.Tick()
		delta := triangle[frame%len(triangle)]
		if frame >= 90 {
			delta -= int16((frame - 89) * 6)
		}
		want := int(replay.TonePeriod(60)) + int(delta)
		period := int(e.Registers[0]) | int(e.Registers[1])<<8
		if period != want {
			t.Fatalf("rendered source frame %d: period=%d, want %d", frame, period, want)
		}
	}
}
