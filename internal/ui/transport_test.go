package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestLiveSongPatternAndRecordChangesPreserveThePlayingRowAndSound(t *testing.T) {
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
	for _, action := range []string{"pattern", "play", "record"} {
		app.action(action)
		after, _ := app.synth.Snapshot()
		if !after.Playing || after.Row != before.Row || after.TickInRow != before.TickInRow || after.Ticks != before.Ticks || after.Registers != before.Registers || after.Voices != before.Voices || after.Patterns != before.Patterns {
			t.Fatalf("%s restarted or interrupted live playback", action)
		}
		if after.PatternMode != (action == "pattern") {
			t.Fatalf("%s did not select the intended mode", action)
		}
	}
	if !app.editing {
		t.Fatal("live record did not enable note entry")
	}
	app.action("stop")
	e, _ := app.synth.Snapshot()
	if e.Playing || app.editing {
		t.Fatal("Stop did not exit live recording")
	}
}

func TestPatternRecordStartsTheSelectedCombinationAndTogglesWithoutRestarting(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.pattern = 3
	app.action("record-pattern")
	e, _ := app.synth.Snapshot()
	if !e.PatternMode || !e.Playing || !app.editing || e.Patterns[0] != 3 {
		t.Fatal("pattern record did not use the selected pattern combination")
	}
	app.synth.Edit(func(e *replay.Engine) {
		for range 17 {
			e.Tick()
		}
	})
	before, _ := app.synth.Snapshot()
	app.togglePatternRecord()
	e, _ = app.synth.Snapshot()
	if app.editing || !e.Playing || e.Row != before.Row || e.TickInRow != before.TickInRow || e.Voices != before.Voices {
		t.Fatal("record toggle interrupted the current pattern")
	}
}

func TestNativeScrollPreferenceControlsTheViewWithoutChangingPlayback(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.row = 0
	e, _ := app.synth.Snapshot()
	e.Playing, e.Row = true, 32
	if app.patternViewStart(&e) != 22 {
		t.Fatal("native scrolling preference was ignored")
	}
	e.Project.Song.State[11] = 0
	if app.patternViewStart(&e) != 0 || e.Row != 32 {
		t.Fatal("fixed view moved with the playback row")
	}
}
