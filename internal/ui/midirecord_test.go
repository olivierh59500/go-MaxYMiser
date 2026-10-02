package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMIDIRecordWritesSelectedNativeVoicesAndUndo(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.editing = true
	app.receiveMIDI([]byte{0x90, 69, 127})
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Note != 69 || e.Project.Song.Patterns[0][0].Instrument != 1 {
		t.Fatal("MIDI recording played a note without storing it")
	}
	app.receiveMIDI([]byte{0x94, 60, 127})
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Effect1 != 60 || e.Project.Song.Patterns[0][0].Parameter1 != 1 {
		t.Fatal("second PCM voice was not recorded in its native columns")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Effect1 != 0 || e.Project.Song.Patterns[0][0].Note != 69 {
		t.Fatal("MIDI edit could not be undone independently")
	}
	app.row = 1
	app.receiveMIDI([]byte{0x90, 69, 127})
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][1].Note != 69 {
		t.Fatal("repeated same-pitch MIDI note was not recorded")
	}
}

func TestNativeInstrumentControllerEditsCanBeUndoneAndRemoteStopLeavesRecordMode(t *testing.T) {
	p := model.New()
	p.Song.State[31] |= 4
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.receiveMIDI([]byte{0xb0, 110, 127})
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[0][32] != 32 || !app.dirty {
		t.Fatal("controller edit was not reflected in the editable instrument")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Instruments[0][32] != 1 {
		t.Fatal("instrument controller edit could not be undone")
	}
	app.action("record")
	app.receiveMIDI([]byte{0xf0, 0x7f, 0x7f, 6, 1, 0xf7})
	e, _ = app.synth.Snapshot()
	if app.editing || e.Playing {
		t.Fatal("MMC stop left the editor recording")
	}
}
