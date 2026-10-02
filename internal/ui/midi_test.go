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
	app.action("midi-channel:4")
	app.entry = "0A"
	app.applyModal()
	path := filepath.Join(t.TempDir(), "midi.mys")
	app.save(path)
	p, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Song.State[31]&1 == 0 || p.Song.State[51] != 10 {
		t.Fatal("external clock or PCM MIDI channel was not saved natively")
	}
}
