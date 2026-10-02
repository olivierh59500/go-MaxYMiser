package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestUnusedPatternSelectionPreservesPlaybackAndSupportsUndoOfNewStorage(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("play")
	app.synth.Edit(func(e *replay.Engine) {
		for range 17 {
			e.Tick()
		}
	})
	before, _ := app.synth.Snapshot()
	app.action("pattern-unused")
	after, _ := app.synth.Snapshot()
	if app.pattern != 8 || len(after.Project.Song.Patterns) != 9 || !app.dirty || after.Row != before.Row || after.TickInRow != before.TickInRow || after.Voices != before.Voices || after.Patterns != before.Patterns {
		t.Fatal("unused selection overwrote material or interrupted the transport")
	}
	app.restore(false)
	after, _ = app.synth.Snapshot()
	if len(after.Project.Song.Patterns) != 8 {
		t.Fatal("allocated unused pattern could not be undone")
	}
}

func TestUnusedSequenceSelectionProtectsLiveOverrides(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.sequence = 2
	app.synth.Edit(func(e *replay.Engine) { e.Voices[0].Parameters[34] = 3 })
	app.action("sequence-unused")
	e, _ := app.synth.Snapshot()
	if app.sequence != 4 || e.Voices[0].Parameters[34] != 3 || e.Project.Bank.Sequences[3].Values[0] != 0 {
		t.Fatal("free sequence search replaced a live effect override")
	}
}
