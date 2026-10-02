package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestInstrumentPreviewDoesNotWritePatternOrResetArrangement(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	before, _ := app.synth.Snapshot()
	app.tab, app.instrument, app.editing = "Instruments", 1, true
	app.action("instrument-preview")
	e, _ := app.synth.Snapshot()
	if e.Voices[0].Note != 48 || e.Voices[0].Instrument != 2 || e.Project.Song.Patterns[0] != before.Project.Song.Patterns[0] || app.dirty {
		t.Fatal("instrument audition changed score data or previewed another sound")
	}
}

func TestNativeConfigurationLoadSaveRetainsAllTwentyNineBytes(t *testing.T) {
	raw := []byte{255, 0, 7, 6, 0, 0, 7, 0, 0, 0, 255, 255, 0, 50, 48, 50, 53, 0, 1, 2, 3, 4, 1, 1, 1, 1, 1, 0, 0}
	root := t.TempDir()
	source := filepath.Join(root, "original.cnf")
	output := filepath.Join(root, "saved.cnf")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err = app.LoadConfiguration(source); err != nil {
		t.Fatal(err)
	}
	app.modal, app.entry = "Save native configuration (.cnf)", output
	app.applyModal()
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("native configuration did not preserve hardware settings: %v", err)
	}
	if _, err = native.DecodeConfiguration(got); err != nil {
		t.Fatal(err)
	}
}
