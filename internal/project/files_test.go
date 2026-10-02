package project

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"os"
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

func TestICESavePairPreservesAllEditableNativeData(t *testing.T) {
	p := model.Demo()
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
	path := filepath.Join(t.TempDir(), "packed.mys")
	if err := SavePacked(p, path, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw[:4]) != "ICE!" {
		t.Fatalf("song was not packed: %v", err)
	}
	got, err := Load(path, "")
	if err != nil || got.Song.Patterns[0] != p.Song.Patterns[0] || got.Bank.Instruments != p.Bank.Instruments || got.Bank.Samples[0].PCM[2] != 128 {
		t.Fatalf("packed pair did not reload: %v", err)
	}
}
