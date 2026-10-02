package project

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"path/filepath"
	"testing"
)

func TestSaveNativePairAndReloadEditedProject(t *testing.T) {
	p := model.Demo()
	p.Song.Patterns[0][10] = model.Cell{Note: 67, Instrument: 2, Effect1: 'T', Parameter1: 12}
	path := filepath.Join(t.TempDir(), "edited.mys")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Song.Patterns[0][10] != p.Song.Patterns[0][10] || got.Bank.Sequences[3] != p.Bank.Sequences[3] {
		t.Fatal("native save/load lost edits")
	}
}
