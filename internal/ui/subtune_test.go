package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSubtuneSelectionPreservesEditsToThePreviousSong(t *testing.T) {
	first, second := model.Demo(), model.New()
	app, err := New(first, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.subtunes = []native.EmbeddedProject{{Song: first.Song, Bank: first.Bank}, {Song: second.Song, Bank: second.Bank}}
	app.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[0][0].Note = 75 })
	app.action("subtune-next")
	if app.subtuneIndex != 1 {
		t.Fatal("second subtune was not selected")
	}
	app.action("subtune-next")
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Note != 75 {
		t.Fatal("switching subtunes discarded an edit")
	}
}
