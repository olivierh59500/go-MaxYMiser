package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestNativeReloadKeepsPersonalMappingsAndLoadedMusic(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			root := t.TempDir()
			p := model.New()
			p.Song.State[40] = 2
			p.Song.State[32] = 3
			p.Song.SetSpeed(8)
			p.Song.Patterns[0][0] = model.Cell{Note: 72, Instrument: 3}
			path := filepath.Join(root, "piece.mys")
			if err := project.Save(p, path); err != nil {
				t.Fatal(err)
			}
			c := native.CaptureConfiguration(model.New().Song, native.Configuration{})
			c[17], c[22], c[28] = 7, 4, 5
			if enabled {
				c[10] = 255
			}
			configPath := filepath.Join(root, "MYM.CNF")
			if err := os.WriteFile(configPath, c[:], 0600); err != nil {
				t.Fatal(err)
			}
			app, err := New(model.Demo(), "", true)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			if err := app.LoadConfiguration(configPath); err != nil {
				t.Fatal(err)
			}
			if err := app.OpenMusic(path); err != nil {
				t.Fatal(err)
			}
			e, _ := app.synth.Snapshot()
			wantedChannel, wantedInstrument := byte(2), byte(3)
			if enabled {
				wantedChannel, wantedInstrument = 7, 4
			}
			if e.Project.Song.State[40] != wantedChannel || e.Project.Song.State[32] != wantedInstrument || e.Project.Song.Speed() != 8 || e.Project.Song.Patterns[0][0].Note != 72 {
				t.Fatal("reload replaced musical settings or ignored the native preference")
			}
			if app.dirty != enabled {
				t.Fatal("reapplied settings were not marked as an editable change")
			}
		})
	}
}

func TestReloadCanCaptureCurrentSettingsAndInvalidOpenKeepsThem(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("config-reload")
	if !app.configurationLoaded || app.nativeConfiguration[10] == 0 {
		t.Fatal("reload did not capture the current settings")
	}
	before := app.nativeConfiguration
	path := filepath.Join(t.TempDir(), "bad.mys")
	os.WriteFile(path, []byte("broken"), 0600)
	if err := app.OpenMusic(path); err == nil {
		t.Fatal("invalid music opened")
	}
	if app.nativeConfiguration != before {
		t.Fatal("failed music opening changed the personal configuration")
	}
}

func TestSaveConfigurationUpdatesTheExistingNativeFile(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	path := filepath.Join(t.TempDir(), "MYM.CNF")
	app.modal, app.entry = "Save native configuration (.cnf)", path
	app.applyModal()
	app.action("config-reload")
	app.modal, app.entry = "Save native configuration (.cnf)", path
	app.applyModal()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := native.DecodeConfiguration(raw)
	if err != nil || config[10] == 0 {
		t.Fatalf("saved reload preference did not replace the existing CNF: %v", err)
	}
}
