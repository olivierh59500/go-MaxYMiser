package ymimport

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func longVolumeFixture() SourceScore {
	return SourceScore{Player: madMaxClassic, Rate: 50, Speed: 3, Frames: 220,
		Instruments: []SourceInstrument{{ID: 0, Settings: []byte{0, 0, 1, 2, 0, 31}, VolumeSequence: []byte{15, 14, 12, 0}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}}},
		Events:      []SourceEvent{{Channel: 0, Frame: 0, Note: 60, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 40, Note: 64, Instrument: 0}, {Channel: 0, Frame: 90, Note: 67, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 150, Instrument: 0, Rest: true}, {Channel: 0, Frame: 180, Note: 65, Instrument: 0, Retrigger: true}},
	}
}

func TestLongEnvelopeUsesExactNativePatternVolumeAndRetainsLegato(t *testing.T) {
	score := longVolumeFixture()
	settings := append([]byte(nil), score.Instruments[0].Settings...)
	volume := append([]byte(nil), score.Instruments[0].VolumeSequence...)
	_, standalone, err := SourceVoiceBank(score)
	if err != nil || standalone.Unsupported[0] == "" {
		t.Fatal("a standalone bank pretended to encode a long envelope in 63 words")
	}
	p, report, err := SourceProject(score, 0, 220)
	if err != nil || !reflect.DeepEqual(report.ScoreVolumeInstruments, []int{0}) || report.UnsupportedEvents != 0 || report.ScoreVolumeChanges == 0 {
		t.Fatalf("long envelope did not become editable score volume: %+v %v", report, err)
	}
	if !reflect.DeepEqual(score.Instruments[0].Settings, settings) || !reflect.DeepEqual(score.Instruments[0].VolumeSequence, volume) {
		t.Fatal("score-volume conversion altered the retained source definition")
	}
	rawSong, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	rawBank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	p.Song, err = native.DecodeSong(rawSong)
	if err != nil {
		t.Fatal(err)
	}
	p.Bank, err = native.DecodeVoiceBank(rawBank)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 220; frame++ {
		e.Tick()
		want := byte(15)
		switch {
		case frame >= 150 && frame < 180:
			want = 0
		case frame >= 211:
			want = 14
		case frame >= 121 && frame < 150:
			want = 14
		case frame >= 71 && frame < 90:
			want = 12
		case frame >= 31 && frame < 71:
			want = 14
		}
		if e.Registers[8] != want {
			t.Fatalf("native envelope frame %d: volume=%d, want %d", frame, e.Registers[8], want)
		}
	}
}

func TestCroppedLongEnvelopeKeepsTheSourceVolumePhase(t *testing.T) {
	score := longVolumeFixture()
	p, _, err := SourceProject(score, 50, 88)
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 50; frame < 88; frame++ {
		e.Tick()
		want := byte(14)
		if frame >= 71 {
			want = 12
		}
		if e.Registers[8] != want {
			t.Fatalf("cropped source frame %d restarted or shifted its long volume envelope", frame)
		}
	}
}

func TestClassicTimedDecayPausesTheEnvelopeUntilAnotherNote(t *testing.T) {
	score := longVolumeFixture()
	score.Frames = 90
	score.Events = []SourceEvent{{Channel: 0, Frame: 0, Note: 60, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 50, Note: 64, Instrument: 0}}
	score.Controls = []SourceControl{{Channel: 0, Frame: 40, Opcode: 0x90, Operand: []byte{1}}}
	p, report, err := SourceProject(score, 0, 90)
	if err != nil || report.ScoreVolumeChanges == 0 {
		t.Fatalf("classic timed decay did not become score volume: %v", err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 90; frame++ {
		e.Tick()
		want := byte(15)
		if frame >= 31 {
			want = 14
		}
		if frame >= 40 && frame < 50 {
			want -= byte((frame - 39) / 2)
		}
		if frame >= 81 {
			want = 12
		}
		if e.Registers[8] != want {
			t.Fatalf("timed-decay frame %d: volume=%d, want %d", frame, e.Registers[8], want)
		}
	}
}

func TestLongEnvelopeFallbackKeepsOtherUnsupportedProgramsSilent(t *testing.T) {
	score := longVolumeFixture()
	score.Instruments[0].Settings[0] = 0x40
	p, report, err := SourceProject(score, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ScoreVolumeInstruments) != 0 || report.Bank.Unsupported[0] == "" || p.Bank.Instruments[0][48] != 0 {
		t.Fatal("an unsupported hardware program acquired a replacement volume sound")
	}
	e := replay.New(p)
	e.Play(false)
	for range 100 {
		e.Tick()
		if e.Registers[8] != 0 {
			t.Fatal("an unsupported hardware definition became audible")
		}
	}
}

func TestLongEnvelopeBankPreparationSupportsSparseSourceIDs(t *testing.T) {
	score := longVolumeFixture()
	score.Instruments[0].ID = 3
	bank, report, ids, err := prepareSourceProjectBank(score)
	if err != nil || !reflect.DeepEqual(ids, []int{3}) || report.Unsupported[3] != "" || bank.Instruments[3] == (model.Instrument{}) {
		t.Fatalf("sparse source ID caused invalid bank preparation: %v", err)
	}
}

func TestLongEnvelopeRejectsInvalidVolumeControlReferences(t *testing.T) {
	score := longVolumeFixture()
	score.Controls = []SourceControl{{Channel: 0, Frame: 20, Opcode: 0xdf}}
	if _, _, err := SourceProject(score, 0, 100); err == nil {
		t.Fatal("a missing volume-control definition reached native conversion")
	}
	score.Controls[0] = SourceControl{Channel: 0, Frame: 20, Opcode: 0x90}
	if _, _, err := SourceProject(score, 0, 100); err == nil {
		t.Fatal("a truncated volume-decay command reached native conversion")
	}
}
