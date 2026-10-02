package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestMIDILatencyEditingRetainsNativeRangeAndSupportsUndo(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	for _, invalid := range []string{"-1", "256", "1.5", "FF"} {
		app.action("midi-latency")
		app.entry = invalid
		app.applyModal()
		e, _ := app.synth.Snapshot()
		if e.Project.Song.State[57] != 0 || len(app.undo) != 0 || app.dirty {
			t.Fatal("invalid MIDI latency changed the native value or edit history")
		}
	}
	app.action("midi-latency")
	app.entry = "255"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Song.State[57] != 255 || !app.dirty {
		t.Fatal("native MIDI latency could not use its complete byte range")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Song.State[57] != 0 {
		t.Fatal("MIDI compensation could not be undone")
	}
	app.restore(true)
	e, _ = app.synth.Snapshot()
	config := native.CaptureConfiguration(e.Project.Song, native.Configuration{})
	if config[28] != 255 {
		t.Fatal("configuration export lost MIDI compensation")
	}
	path := filepath.Join(t.TempDir(), "clock.mys")
	app.save(path)
	loaded, err := project.Load(path, "")
	if err != nil || loaded.Song.State[57] != 255 {
		t.Fatalf("native song round trip lost MIDI compensation: %v", err)
	}
}

func TestImportedSync24SelectionCanBeChangedToMIDIWithoutLosingControllers(t *testing.T) {
	p := model.New()
	p.Song.State[31] = 7
	p.Song.SetSpeed(17)
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("midi-clock")
	e, _ := app.synth.Snapshot()
	if !e.ExternalClock || e.Project.Song.State[31] != 5 || e.Speed != 6 || e.Project.Song.Speed() != 6 {
		t.Fatal("MIDI selection retained the Sync24 bit or disabled controllers")
	}
	app.action("midi-clock")
	e, _ = app.synth.Snapshot()
	if e.ExternalClock || e.Project.Song.State[31] != 4 {
		t.Fatal("internal clock selection lost controller enablement")
	}
}

func TestUnselectedMIDITransportDoesNotInterruptRecording(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("record")
	app.receiveMIDI([]byte{0xfc})
	e, _ := app.synth.Snapshot()
	if !app.editing || !e.Playing {
		t.Fatal("unselected MIDI Stop interrupted internal-clock recording")
	}
}
