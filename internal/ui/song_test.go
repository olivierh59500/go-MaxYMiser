package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestArrangementCreatesMissingPatternsAndPreservesValidLoopOnSave(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("song-length")
	app.entry = "04"
	app.applyModal()
	app.action("song-repeat")
	app.entry = "03"
	app.applyModal()
	app.action("order:3:0")
	app.entry = "0A"
	app.applyModal()
	app.action("position:3")
	if app.pattern != 10 {
		t.Fatal("selected order did not select its new pattern")
	}
	path := filepath.Join(t.TempDir(), "arranged.mys")
	app.save(path)
	p, err := project.Load(path, "")
	if err != nil || p.Song.Length != 4 || p.Song.Repeat != 3 || len(p.Song.Patterns) != 11 || p.Song.Orders[3][0] != 10 {
		t.Fatalf("arrangement did not round-trip: %v", err)
	}
	app.action("order:0:0")
	app.entry = "F0"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Orders[0][0] != 0 {
		t.Fatal("unsupported special pattern was accepted")
	}
	app.action("order:remove")
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Length != 3 || e.Project.Song.Repeat != 2 || e.Position != 2 {
		t.Fatal("shrinking song left an invalid repeat or current position")
	}
}
