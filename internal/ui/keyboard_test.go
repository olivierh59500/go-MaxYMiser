package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestPercussionKeyboardAndRecordUseNativeInstruments(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.drumKeyboard, app.editing = true, true
	app.enterNote(60)
	e, _ := app.synth.Snapshot()
	if e.Voices[0].Note != 48 || e.Project.Song.Patterns[0][0].Note != 48 || e.Project.Song.Patterns[0][0].Instrument != 17 {
		t.Fatal("percussion keyboard did not map note to a middle-C sound")
	}
	app.action("record")
	e, _ = app.synth.Snapshot()
	if !app.editing || !e.Playing {
		t.Fatal("Record did not start the native recording transport")
	}
	app.action("stop")
	e, _ = app.synth.Snapshot()
	if app.editing || e.Playing {
		t.Fatal("Stop did not stop recording mode")
	}
}

func TestHelpTopicsAreSelectable(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("help:1")
	if app.helpTopic != 1 || len(helpText[1]) < 9 {
		t.Fatal("tracker effects help not available")
	}
}
