package ymimport

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestExplicitRowGridPreservesSimpleNoteTiming(t *testing.T) {
	trace := melodyTrace([]int{60, 64, 62, 67, 65, 69, 66, 71}, 12, 0, 0, false)
	p, report, err := ReconstructSelection(trace, ReconstructionOptions{StartFrame: 12, EndFrame: 72, FramesPerRow: 6})
	if err != nil {
		t.Fatal(err)
	}
	if p.Song.Speed() != 6 || report.StartFrame != 12 || report.EndFrame != 72 || report.FramesPerRow != 6 {
		t.Fatal("selected timeline or row spacing was lost")
	}
	e := replay.New(p)
	e.Play(false)
	for tick := 0; tick < 60; tick++ {
		e.Tick()
		wanted := []byte{64, 62, 67, 65, 69}[tick/12]
		if e.Voices[0].Note != wanted {
			t.Fatalf("tick %d note %d, want %d", tick, e.Voices[0].Note, wanted)
		}
	}
	if len(report.Warnings) == 0 {
		t.Fatal("coarse grid approximation was hidden")
	}
}

func TestSelectionRejectsInvalidBoundsAndDoesNotModifyReference(t *testing.T) {
	trace, _ := Decode(simpleYM3(128))
	before := trace.Frames[0]
	if _, _, err := ReconstructSelection(trace, ReconstructionOptions{StartFrame: 100, EndFrame: 99}); err == nil {
		t.Fatal("reversed range accepted")
	}
	if _, _, err := ReconstructSelection(trace, ReconstructionOptions{EndFrame: 129}); err == nil {
		t.Fatal("range beyond reference accepted")
	}
	if trace.Frames[0] != before {
		t.Fatal("selection modified original reference data")
	}
}
