package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestFileBrowserFiltersAndNavigatesNativeProjects(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "songs")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := project.Save(model.Demo(), filepath.Join(child, "example.mys")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(child, "unrelated.txt"), []byte("text"), 0600)
	app, err := New(model.New(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.directory = root
	app.action("open")
	if app.browser == nil || len(app.browser.entries) != 1 || !app.browser.entries[0].IsDir() {
		t.Fatal("browser did not show the child directory")
	}
	app.action("file:select:0")
	if app.browser.directory != child || len(app.browser.entries) != 2 {
		t.Fatal("browser did not navigate/filter native files")
	}
	for i, entry := range app.browser.entries {
		if entry.Name() == "example.mys" {
			app.action("file:select:" + strconv.Itoa(i))
		}
	}
	app.applyModal()
	if app.browser != nil || app.projectPath != filepath.Join(child, "example.mys") {
		t.Fatalf("browser did not open chosen project: %s", app.status)
	}
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[1].Name() != "Chord pulse" {
		t.Fatal("opening MYS from browser did not load its matching bank")
	}
}

func TestFileBrowserSaveResolvesRelativeNamesInChosenDirectory(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.directory = t.TempDir()
	app.action("save")
	app.entry = "saved.mys"
	app.applyModal()
	if _, err := os.Stat(filepath.Join(app.directory, "saved.mys")); err != nil {
		t.Fatalf("relative save did not use browser directory: %v", err)
	}
}

func TestSaveAsChoosesANewPairWithoutOverwritingTheCurrentDestination(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original.mys")
	if err := project.Save(model.Demo(), original); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.Demo(), original, true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("save-as")
	app.entry = "variation.mys"
	app.applyModal()
	if app.projectPath != filepath.Join(root, "variation.mys") {
		t.Fatal("Save as did not set the new native pair destination")
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatal("Save as removed the original song")
	}
}

func TestFailedSaveKeepsTheEditorDirtyAndItsCurrentDestination(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original.mys")
	app, err := New(model.Demo(), original, true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.dirty = true
	target := filepath.Join(root, "blocked.mys")
	if err := os.Mkdir(filepath.Join(root, "blocked.myv"), 0700); err != nil {
		t.Fatal(err)
	}
	app.save(target)
	if !app.dirty || app.projectPath != original {
		t.Fatal("failed native save cleared unsaved edits or changed the current destination")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("failed save left a partial song file")
	}
}
