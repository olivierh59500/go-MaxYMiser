package ymimport

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSourceNoteExcerptRetainsVerifiedFramesAndLegato(t *testing.T) {
	score, err := DecodeSource(sourceFixture(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	p, report, err := SourceProject(score, 0, 25)
	if err != nil || report.UnsupportedEvents != 0 || report.StartFrame != 0 || report.EndFrame != 25 {
		t.Fatalf("source-note conversion failed: %+v %v", report, err)
	}
	pattern := p.Song.Patterns[p.Song.Orders[0][0]]
	if pattern[0].Note != 84 || pattern[0].Instrument != 4 || pattern[6].Note != 86 || pattern[6].Instrument != 0 || pattern[12].Note != 88 || pattern[12].Instrument != 4 {
		t.Fatal("native note timing, source instrument mapping or legato changed")
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 25; frame++ {
		e.Tick()
		want := byte(84)
		if frame >= 6 && frame < 12 {
			want = 86
		}
		if frame >= 12 && frame < 24 {
			want = 88
		}
		if e.Voices[0].Note != want {
			t.Fatalf("source frame %d: note=%d, want %d", frame, e.Voices[0].Note, want)
		}
	}
	if e.Loops != 1 || e.Row != 0 || p.Song.State[49] != 0 {
		t.Fatal("selected excerpt acquired extra empty rows or PCM sequencing")
	}
	raw, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := native.DecodeSong(raw); err != nil {
		t.Fatal(err)
	}
}

func TestSourceExcerptNamesUnsupportedSoundsAndReportsUntranslatedCommands(t *testing.T) {
	score, err := DecodeSource(sourceFixture(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	score.Instruments[3].Settings[0] = 0x40
	score.Patterns[0].Commands = append(score.Patterns[0].Commands, SourceCommand{Opcode: 0x88, Operand: []byte{1, 2, 3}})
	p, report, err := SourceProject(score, 0, 25)
	if err != nil || report.UnsupportedEvents != 12 || report.UntranslatedCommands[0x88] != 1 || p.Bank.Instruments[3].Name() != "Source 03 ?" {
		t.Fatalf("unsupported source details disappeared: %+v %v", report, err)
	}
	if p.Bank.Instruments[3][17] != 0 || p.Bank.Instruments[3][48] != 0 {
		t.Fatal("unsupported source synthesis became an invented playable sound")
	}
	if len(report.Warnings) < 3 {
		t.Fatal("partial conversion did not expose its limits")
	}
}

func TestClippedSourceExcerptRestoresItsStartingNoteWithoutDroppingAnInitialLegato(t *testing.T) {
	score, err := DecodeSource(sourceFixture(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	for _, start := range []int{4, 6} {
		p, report, err := SourceProject(score, start, 20)
		if err != nil {
			t.Fatal(err)
		}
		cell := p.Song.Patterns[p.Song.Orders[0][0]][0]
		wanted := byte(84)
		if start == 6 {
			wanted = 86
		}
		if cell.Note != wanted || cell.Instrument != 4 || len(report.Warnings) == 0 {
			t.Fatal("cropped source playback lost its first sounding instrument")
		}
	}
}

func TestSourceExcerptRejectsInvalidEventsAndNativeCapacityOverflow(t *testing.T) {
	score, err := DecodeSource(sourceFixture(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*SourceScore){
		func(s *SourceScore) { s.Events[0].Channel = 3 },
		func(s *SourceScore) { s.Events[0].Instrument = 32 },
		func(s *SourceScore) { s.Events[0].Note = 200 },
		func(s *SourceScore) { s.Events[0].Frame = -1 },
	} {
		candidate := score
		candidate.Events = append([]SourceEvent(nil), score.Events...)
		mutate(&candidate)
		if _, _, err := SourceProject(candidate, 0, 25); err == nil {
			t.Fatal("invalid source event reached native editing")
		}
	}
	score.Frames = 256 * model.Rows
	if _, _, err := SourceProject(score, 0, 0); err == nil {
		t.Fatal("native arrangement capacity overflow was accepted")
	}
}
