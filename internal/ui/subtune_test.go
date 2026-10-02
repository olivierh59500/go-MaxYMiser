package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
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

func TestSubtunesKeepSeparateUndoAndSaveDestinations(t *testing.T) {
	first, second := model.Demo(), model.New()
	originalNote := first.Song.Patterns[0][0].Note
	second.Song.Orders[0] = [4]byte{255, 1, 2, 255}
	app, err := New(first, "collection.sndh", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.subtunes = []native.EmbeddedProject{{Title: "First", Song: first.Song, Bank: first.Bank}, {Title: "Second", Song: second.Song, Bank: second.Bank}}
	app.initializeSubtuneWorkspaces()
	if app.projectPath != "" {
		t.Fatal("multi-song source remained the current one-song save target")
	}
	app.editCell(func(c *model.Cell) { c.Note = 75 })
	firstPath := filepath.Join(t.TempDir(), "first.mys")
	app.save(firstPath)
	if err := app.SelectSubtune(1); err != nil {
		t.Fatal(err)
	}
	if app.pattern != 1 || app.channel != 1 || app.projectPath != "" || len(app.undo) != 0 {
		t.Fatal("second subtune inherited another song's cursor, save target or history")
	}
	app.editCell(func(c *model.Cell) { c.Note = 79 })
	app.restore(false)
	e, _ := app.synth.Snapshot()
	if e.Project.Song.Patterns[1][0].Note != 0 || e.Project.Title != "Second" {
		t.Fatal("undo in the second subtune restored data from the first")
	}
	if err := app.SelectSubtune(0); err != nil {
		t.Fatal(err)
	}
	if app.projectPath != firstPath {
		t.Fatal("returning to a subtune lost its own save destination")
	}
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Note != 75 {
		t.Fatal("saved first subtune was replaced by another song")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Song.Patterns[0][0].Note == 75 {
		t.Fatal("first subtune's own undo was lost")
	}
	app.save(firstPath)
	p, err := project.Load(firstPath, "")
	if err != nil || p.Song.Patterns[0][0].Note != originalNote {
		t.Fatalf("independent subtune save failed: %v", err)
	}
}

func TestChangingOneSubtuneReplayTemplateDoesNotReplaceItsSiblingTemplate(t *testing.T) {
	first, second := model.New(), model.New()
	first.ReplaySource = []byte("original collection")
	app, err := New(first, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.subtunes = []native.EmbeddedProject{{Song: first.Song, Bank: first.Bank}, {Song: second.Song, Bank: second.Bank}}
	app.initializeSubtuneWorkspaces()
	if err := app.SelectSubtune(1); err != nil {
		t.Fatal(err)
	}
	app.synth.Edit(func(e *replay.Engine) { e.Project.ReplaySource = []byte("selected single-song template") })
	if err := app.SelectSubtune(0); err != nil {
		t.Fatal(err)
	}
	e, _ := app.synth.Snapshot()
	if string(e.Project.ReplaySource) != "original collection" {
		t.Fatal("sibling subtune inherited another song's export template")
	}
	if err := app.SelectSubtune(1); err != nil {
		t.Fatal(err)
	}
	e, _ = app.synth.Snapshot()
	if string(e.Project.ReplaySource) != "selected single-song template" {
		t.Fatal("selected subtune lost its own export template")
	}
	if string(app.collectionSource) != "original collection" {
		t.Fatal("collection export inherited a selected song's replacement template")
	}
}

func TestCollectionExportRejectsAnUnverifiedWrapperWithoutChangingEdits(t *testing.T) {
	first, second := model.New(), model.New()
	first.ReplaySource = []byte("unknown selector")
	app, err := New(first, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.subtunes = []native.EmbeddedProject{{Song: first.Song, Bank: first.Bank}, {Song: second.Song, Bank: second.Bank}}
	app.initializeSubtuneWorkspaces()
	app.editCell(func(c *model.Cell) { c.Note = 75 })
	app.action("subtune-export-all")
	e, _ := app.synth.Snapshot()
	if app.browser != nil || app.modal != "" || !app.dirty || e.Project.Song.Patterns[0][0].Note != 75 {
		t.Fatal("unknown collection wrapper opened an unsafe export or changed edits")
	}
}

func TestSubtuneSelectorUsesOneBasedNumbersAndRejectsMissingSongs(t *testing.T) {
	first, second := model.New(), model.New()
	app, err := New(first, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.subtunes = []native.EmbeddedProject{{Song: first.Song, Bank: first.Bank}, {Song: second.Song, Bank: second.Bank}}
	app.action("subtune-select")
	app.entry = "2"
	app.applyModal()
	if app.subtuneIndex != 1 {
		t.Fatal("subtune selector did not use a one-based index")
	}
	app.action("subtune-select")
	app.entry = "3"
	app.applyModal()
	if app.subtuneIndex != 1 {
		t.Fatal("missing subtune selection changed the current song")
	}
}
