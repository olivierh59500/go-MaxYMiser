package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestMIDIClockAndNativeChannelMappingSurviveSave(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("midi-clock")
	app.action("midi-controllers")
	app.action("midi-sound:0")
	app.entry = "DD"
	app.applyModal()
	app.action("midi-channel:4")
	app.entry = "0A"
	app.applyModal()
	path := filepath.Join(t.TempDir(), "midi.mys")
	app.save(path)
	p, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Song.State[31]&5 != 5 || p.Song.State[51] != 10 || p.Song.State[32] != 0xdd {
		t.Fatal("external clock or PCM MIDI channel was not saved natively")
	}
}

func TestPCMMappingRejectsDDAndSupportsAnExplicitDisabledTrack(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("midi-sound:4")
	app.entry = "DD"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Song.State[50] != 1 {
		t.Fatal("PCM mapping accepted a YM percussion mode")
	}
	app.action("midi-sound:4")
	app.entry = "00"
	app.applyModal()
	e, _ = app.synth.Snapshot()
	if e.Project.Song.State[50] != 0 {
		t.Fatal("PCM mapping could not disable a track")
	}
}
