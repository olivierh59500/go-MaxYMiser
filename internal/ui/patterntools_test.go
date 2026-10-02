package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestBlockEditCopyPasteTransposeAndUndoKeepTheNativeColumns(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][4] = model.Cell{Note: 60, Instrument: 1, Effect1: 'S', Parameter1: 6}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.blockFirst, app.blockLast = 4, 7
	app.action("block-copy")
	app.row, app.pasteMode = 12, edit.Overlay
	app.action("block-paste")
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Patterns[0][12].Note != 60 || e.Project.Song.Patterns[0][12].Parameter1 != 6 {
		t.Fatal("block clipboard did not paste at the cursor")
	}
	app.blockFirst, app.blockLast = 12, 15
	app.action("block-transpose")
	app.entry = "12"
	app.applyModal()
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][12].Note != 72 || e.Project.Song.Patterns[0][12].Effect1 != 'S' {
		t.Fatal("YM transposition modified an effect instead of only notes")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][12].Note != 60 {
		t.Fatal("block transposition could not be undone")
	}
}

func TestSecondPCMLaneVolumeIsEnteredAsAttenuation(t *testing.T) {
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.channel, app.field, app.editing = 3, 5, true
	app.enterField('0')
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Effect2 != 16 || app.field != 5 {
		t.Fatal("PCM second-lane volume was treated as a YM effect code")
	}
}
