package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestOpeningNativeScoreUpdatesTheViewAndInitialTrack(t *testing.T) {
	p := model.New()
	p.Title = "Loaded piece"
	p.Bank.Instruments[0].SetName("Different sound")
	p.Song.Orders[0] = [4]byte{255, 1, 2, 255}
	p.Song.Patterns[1][0] = model.Cell{Note: 75, Instrument: 1}
	path := filepath.Join(t.TempDir(), "loaded.mys")
	if err := project.Save(p, path); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.tab, app.instrument, app.sequence = "Settings", 2, 4
	if err = app.OpenMusic(path); err != nil {
		t.Fatal(err)
	}
	e, _ := app.synth.Snapshot()
	if app.tab != "Patterns" || app.channel != 1 || app.pattern != 1 || app.instrument != 0 || e.Project.Bank.Instruments[0].Name() != "Different sound" || e.Project.Song.Patterns[1][0].Note != 75 {
		t.Fatal("loaded score kept the old workspace, instrument or partition")
	}
}

func TestUnsupportedSNDHShowsVisibleFailureAndRetainsTheComposition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other-player.sndh")
	raw := make([]byte, 32)
	copy(raw[12:], "SNDHTITLAnother player")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.modal, app.entry = "Open music", path
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if app.modal != "Unable to open this music" || len(app.errorDetails) < 3 || e.Project.Title != "First signal" || e.Project.Bank.Instruments[1].Name() != "Chord pulse" {
		t.Fatal("unsupported SNDH silently retained an old score without explaining why")
	}
}
