package ymimport

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func classicTransposeFixture() SourceScore {
	s := SourceScore{Player: madMaxClassic, Frames: 6, Rate: 50, Speed: 1}
	for id := 0; id < 32; id++ {
		s.Instruments = append(s.Instruments, SourceInstrument{ID: id, Settings: []byte{0, 0, 0, 0, 1, 1}, VolumeSequence: []byte{12}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}}})
	}
	s.Patterns = []SourcePattern{
		{ID: 0, Commands: []SourceCommand{{Opcode: 0xe0}, {Opcode: 0xc1}, {Opcode: 60}, {Opcode: 0x89, Offset: 100, Operand: []byte{0}}, {Opcode: 62}, {Opcode: 0x89, Offset: 104, Operand: []byte{0xf4}}, {Opcode: 0x90, Operand: []byte{0}}, {Opcode: 0x87}}},
		{ID: 1, Commands: []SourceCommand{{Opcode: 0xe3}, {Opcode: 0xc1}, {Opcode: 64}, {Opcode: 0x87}}},
		{ID: 2, Commands: []SourceCommand{{Opcode: 0xe3}, {Opcode: 0xc1}, {Opcode: 67}, {Opcode: 0x87}}},
	}
	for ch := 0; ch < 3; ch++ {
		s.Orders[ch] = []SourceOrder{{Pattern: ch}}
	}
	return s
}

func TestClassicGlobalTransposeRetunesHeldVoicesWithoutRetriggering(t *testing.T) {
	s := classicTransposeFixture()
	var err error
	s.Events, s.Controls, err = sourceTimelineControls(s, s.Frames)
	if err != nil {
		t.Fatal(err)
	}
	want := [3][3]int{{72, 76, 79}, {86, 88, 91}, {74, 76, 79}}
	seen := 0
	for _, e := range s.Events {
		if e.Frame > 2 {
			continue
		}
		seen++
		if e.Note != want[e.Frame][e.Channel] {
			t.Fatalf("incorrect global pitch at frame %d channel %d: %+v", e.Frame, e.Channel, e)
		}
		pitchOnly := e.Frame == 2 || e.Frame == 1 && e.Channel != 0
		if e.PitchOnly != pitchOnly || pitchOnly && e.Retrigger {
			t.Fatal("global transpose restarted a held source note:", e)
		}
	}
	if seen != 9 {
		t.Fatal("missing held-voice pitch changes")
	}
	p, report, err := SourceProject(s, 0, s.Frames)
	if err != nil || report.UntranslatedCommands[0x89] != 0 {
		t.Fatalf("global transpose was not retained in the editable score: %v", err)
	}
	for frame := 1; frame <= 2; frame++ {
		for ch := 1; ch < 3; ch++ {
			cell := p.Song.Patterns[p.Song.Orders[0][ch]][frame]
			if cell.Note != byte(want[frame][ch]) || cell.Instrument != 0 {
				t.Fatal("held pitch changed the instrument/envelope trigger:", cell)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "transpose.mys")
	if err := project.Save(p, path); err != nil {
		t.Fatal(err)
	}
	again, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	a, b := replay.New(p), replay.New(again)
	a.Play(false)
	b.Play(false)
	for frame := 0; frame < s.Frames; frame++ {
		a.Tick()
		b.Tick()
		if a.Registers != b.Registers || a.EnvelopeWrite != b.EnvelopeWrite {
			t.Fatal("native save/reload changed global-transpose playback")
		}
	}
}

func TestClassicGlobalTransposeUsesTheFinalControlOnTheSameCall(t *testing.T) {
	events := []SourceEvent{{Channel: 0, Frame: 0, Note: 72, Retrigger: true}, {Channel: 1, Frame: 0, Note: 76, Retrigger: true}}
	controls := []SourceControl{{Frame: 0, Opcode: 0x89, Operand: []byte{0}}, {Channel: 2, Frame: 0, Opcode: 0x89, Operand: []byte{0xff}}}
	got := classicGlobalTranspose(events, controls, 1)
	if len(got) != 2 || got[0].Note != 83 || got[1].Note != 87 || got[0].PitchOnly || got[1].PitchOnly {
		t.Fatal("earlier parsed voices missed the final signed global transpose")
	}
}

func TestClassicTransposeCommandIsDecodedFromBoundedPatternBytes(t *testing.T) {
	b, layout := classicTableFixture()
	copy(b[0x1000:], []byte{0xc1, 0xe1, 60, 0x89, 0, 62, 0x87})
	s, err := decodeSourceTables(b, layout, 12)
	if err != nil || len(s.Events) < 6 || s.Events[3].Note != 86 {
		t.Fatalf("classic 89 operand changed command timing: %v", err)
	}
}
