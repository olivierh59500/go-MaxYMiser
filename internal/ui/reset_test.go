package ui

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestIndependentContentClearingRetainsTheOtherHalfAndSupportsUndoRedo(t *testing.T) {
	for _, action := range []string{"song-clear", "bank-clear"} {
		t.Run(action, func(t *testing.T) {
			p := model.Demo()
			p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
			p.Bank.Samples[0].Trailer = []byte{0}
			before := p.Clone()
			app, err := New(p, "composition.mys", true)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			app.pattern, app.row, app.instrument, app.editing = 7, 40, 2, true
			app.synth.Edit(func(e *replay.Engine) { e.Play(false) })
			app.action(action)
			e, _ := app.synth.Snapshot()
			if e.Playing || app.editing || !app.dirty || app.projectPath != "composition.mys" {
				t.Fatal("clearing content did not stop recording or keep the editable save destination")
			}
			if action == "song-clear" {
				if !reflect.DeepEqual(e.Project.Bank, before.Bank) || e.Project.Song.Patterns[0] != (model.Pattern{}) || app.pattern != 0 || app.row != 0 {
					t.Fatal("song clearing altered reusable sounds or left an invalid cursor")
				}
			} else if !reflect.DeepEqual(e.Project.Song, before.Song) || len(e.Project.Bank.Samples[0].PCM) != 0 {
				t.Fatal("bank clearing altered the partition or retained samples")
			}
			cleared := e.Project.Clone()
			app.restore(false)
			e, _ = app.synth.Snapshot()
			if !reflect.DeepEqual(e.Project.Song, before.Song) || !reflect.DeepEqual(e.Project.Bank, before.Bank) {
				t.Fatal("undo failed to restore the original notes, sequences or samples")
			}
			app.restore(true)
			e, _ = app.synth.Snapshot()
			if !reflect.DeepEqual(e.Project.Song, cleared.Song) || !reflect.DeepEqual(e.Project.Bank, cleared.Bank) {
				t.Fatal("redo did not restore the independent clearing operation")
			}
		})
	}
}

func TestClearingContentKeepsSiblingSubtunesAndTheirUndoIndependent(t *testing.T) {
	for _, action := range []string{"song-clear", "bank-clear"} {
		t.Run(action, func(t *testing.T) {
			first, second := model.Demo(), model.Demo()
			second.Song.Patterns[0][0].Note = 79
			second.Bank.Instruments[0].SetName("Sibling sound")
			app, err := New(first, "collection.sndh", true)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			app.subtunes = []native.EmbeddedProject{{Title: "First", Song: first.Song, Bank: first.Bank}, {Title: "Second", Song: second.Song, Bank: second.Bank}}
			app.initializeSubtuneWorkspaces()
			app.action(action)
			if err := app.SelectSubtune(1); err != nil {
				t.Fatal(err)
			}
			e, _ := app.synth.Snapshot()
			if e.Project.Song.Patterns[0][0].Note != 79 || e.Project.Bank.Instruments[0].Name() != "Sibling sound" || len(app.undo) != 0 {
				t.Fatal("clearing one song changed a sibling or its undo history")
			}
			if err := app.SelectSubtune(0); err != nil {
				t.Fatal(err)
			}
			if !app.dirty || len(app.undo) != 1 {
				t.Fatal("returning to the cleared song lost its change or original snapshot")
			}
			app.restore(false)
			e, _ = app.synth.Snapshot()
			if e.Project.Song.Patterns[0][0].Note != 60 || e.Project.Bank.Instruments[0].Name() != "Square" {
				t.Fatal("undo after switching subtunes did not restore the selected song")
			}
		})
	}
}
