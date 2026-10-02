package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestArrangementClipboardAndCloneChangeOnlyTheSelectedOccurrence(t *testing.T) {
	p := model.New()
	p.Song.Length = 2
	p.Song.Orders[1] = p.Song.Orders[0]
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("position:1")
	app.action("order:clone-pattern")
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Orders[0][0] != 0 || e.Project.Song.Orders[1][0] != 3 || app.pattern != 3 || !app.dirty {
		t.Fatal("cloning an arrangement track did not create an independent occurrence")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Orders[1][0] != 0 || len(e.Project.Song.Patterns) != 3 {
		t.Fatal("arrangement clone could not be undone")
	}
	app.action("order:copy")
	app.action("order:paste")
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Length != 3 {
		t.Fatal("copied order was not inserted")
	}
	app.action("order:delete")
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Length != 2 {
		t.Fatal("selected position could not be deleted")
	}
}

func TestPatternAllocationIsMarkedDirtyAndUndoableAndAllMasksCanToggle(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.pattern = 2
	app.action("pat:+")
	e, _ := app.synth.Snapshot()
	if len(e.Project.Song.Patterns) != 4 || !app.dirty {
		t.Fatal("new editable pattern allocation was not recorded as an edit")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if len(e.Project.Song.Patterns) != 3 {
		t.Fatal("pattern allocation could not be undone")
	}
	app.action("column-mask-all")
	if app.columnMask != (edit.ColumnMask{}) {
		t.Fatal("all column masks were not disabled")
	}
	app.action("column-mask-all")
	if app.columnMask != edit.AllColumns {
		t.Fatal("all column masks were not enabled")
	}
}
