package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"os"
)

func TestCompositionYearCanBeEditedUndoneAndStoredInNativeConfiguration(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("song-year")
	app.entry = "2026"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Year != "2026" || !app.dirty {
		t.Fatal("composition year was not applied")
	}
	path := filepath.Join(t.TempDir(), "MYM.CNF")
	app.modal, app.entry = "Save native configuration (.cnf)", path
	app.applyModal()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := native.DecodeConfiguration(raw)
	if err != nil || string(config[13:17]) != "2026" {
		t.Fatal("composition year was not written to native CNF")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Year != "" {
		t.Fatal("composition year could not be undone")
	}
}
