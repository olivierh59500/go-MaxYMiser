package ymimport

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func fixedMixerFixture() SourceScore {
	return SourceScore{Player: madMaxClassic, Rate: 50, Speed: 3, Frames: 30,
		Instruments: []SourceInstrument{
			{ID: 0, Settings: []byte{2, 0, 0, 0, 0, 1}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}},
			{ID: 1, Settings: []byte{0, 0, 0, 0, 0, 1}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}},
		},
		Events:   []SourceEvent{{Channel: 2, Frame: 0, NativeNote: 16, Note: 28, Instrument: 0, Retrigger: true, FixedPitch: true}, {Channel: 2, Frame: 9, NativeNote: 16, Note: 28, Instrument: 0, FixedPitch: true}},
		Controls: []SourceControl{{Channel: 2, Frame: 0, Opcode: 0xc0}},
	}
}

func TestClassicFixedMixerKeepsNativePhaseOnLegatoAndAfterNativeReload(t *testing.T) {
	score := fixedMixerFixture()
	_, standalone, err := SourceVoiceBank(score)
	if err != nil || standalone.Unsupported[0] == "" {
		t.Fatal("a standalone bank claimed to encode a shared-noise program")
	}
	p, report, err := SourceProject(score, 0, 30)
	if err != nil || len(report.ScoreMixerInstruments) != 1 || report.ScoreMixerChanges == 0 || report.UnsupportedEvents != 0 {
		t.Fatalf("fixed-pitch mixer was not converted to editable commands: %+v %v", report, err)
	}
	song, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	p.Song, err = native.DecodeSong(song)
	if err != nil {
		t.Fatal(err)
	}
	p.Bank, err = native.DecodeVoiceBank(bank)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 30; frame++ {
		e.Tick()
		age := frame
		if frame >= 9 {
			age = frame - 9
		}
		mask := byte(4 | 32)
		want := byte(32)
		if age%2 == 0 {
			want = 4
		}
		if e.Registers[7]&mask != want || e.Registers[10] != 15 {
			t.Fatalf("native fixed mixer frame %d: mixer=%02x volume=%d", frame, e.Registers[7]&mask, e.Registers[10])
		}
		if age%2 == 0 && e.Registers[6] != 17 {
			t.Fatal("native alternating noise period was replaced by a constant default")
		}
	}
}

func TestClassicFixedMixerReadsTheNoiseShadowChangedByAnotherVoice(t *testing.T) {
	score := fixedMixerFixture()
	score.Events = append([]SourceEvent{{Channel: 0, Frame: 0, NativeNote: 45, Note: 57, Instrument: 1, Retrigger: true}}, score.Events...)
	score.Controls = append([]SourceControl{{Channel: 0, Frame: 0, Opcode: 0x8b}, {Channel: 0, Frame: 0, Opcode: 0xc1}}, score.Controls...)
	p, _, err := SourceProject(score, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 8; frame++ {
		e.Tick()
		if frame%2 == 0 && e.Registers[6] != 5 {
			t.Fatalf("fixed noise frame %d ignored the other voice's shadow period: %d", frame, e.Registers[6])
		}
	}
}

func TestClassicFixedMixerCropPreservesAlternatingPhase(t *testing.T) {
	score := fixedMixerFixture()
	p, _, err := SourceProject(score, 1, 8)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 1; frame < 8; frame++ {
		e.Tick()
		want := byte(32)
		if frame%2 == 0 {
			want = 4
		}
		if e.Registers[7]&(4|32) != want {
			t.Fatal("cropping restarted the source's mixer alternation")
		}
	}
}

func TestClassicFixedMixerKeepsUnverifiedNoiseSweepDefinitionsUnsupported(t *testing.T) {
	score := fixedMixerFixture()
	score.Controls = append(score.Controls, SourceControl{Channel: 0, Frame: 6, Opcode: 0x8f})
	p, report, err := SourceProject(score, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ScoreMixerInstruments) != 0 || report.Bank.Unsupported[0] == "" || p.Bank.Instruments[0][48] != 0 {
		t.Fatal("an unverified shared-noise sweep acquired a guessed sound program")
	}
}

func TestClassicMixerRejectsInvalidPositionsAndMissingDefinitions(t *testing.T) {
	for _, modify := range []func(*SourceScore){
		func(s *SourceScore) { s.Controls[0].Channel = 3 },
		func(s *SourceScore) { s.Events[0].Frame = -1 },
		func(s *SourceScore) { s.Controls[0].Opcode = 0xdf },
		func(s *SourceScore) { s.Events[0].Instrument = 30 },
	} {
		score := fixedMixerFixture()
		modify(&score)
		if _, err := classicMixerFrames(score); err == nil {
			t.Fatal("invalid source data reached the classic mixer model")
		}
	}
}

func TestSourceFinalBreakRetainsAnExistingLastFrameEffect(t *testing.T) {
	score := fixedMixerFixture()
	score.Frames = 5
	score.Events = []SourceEvent{{Channel: 0, Frame: 0, Note: 28, NativeNote: 16, Instrument: 0, Retrigger: true, FixedPitch: true}}
	score.Controls = []SourceControl{{Channel: 0, Frame: 0, Opcode: 0xc0}}
	p, _, err := SourceProject(score, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	cell := p.Song.Patterns[p.Song.Orders[0][0]][4]
	if cell.Effect1 != 'M' || cell.Effect2 != 'B' {
		t.Fatalf("excerpt termination overwrote a last-frame mixer command: %+v", cell)
	}
	e := replay.New(p)
	e.Play(false)
	for range 5 {
		e.Tick()
	}
	if e.Loops != 1 || e.Row != 0 || p.Song.State[49] != 0 || len(p.Song.Patterns) > model.MaxPatterns {
		t.Fatal("retaining the mixer command changed the native excerpt boundary")
	}
}
