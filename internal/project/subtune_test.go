package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestLoadSubtuneSelectsTheRequestedEditableSong(t *testing.T) {
	first, second := model.New(), model.New()
	first.Song.Patterns[0][0].Note = 60
	second.Song.Patterns[0][0].Note = 72
	header := make([]byte, 32)
	copy(header[12:], "SNDH")
	container := append([]byte(nil), header...)
	for _, p := range []*model.Project{first, second} {
		bank, err := native.EncodeVoiceBank(p.Bank)
		if err != nil {
			t.Fatal(err)
		}
		song, err := native.EncodeSong(p.Song)
		if err != nil {
			t.Fatal(err)
		}
		container = append(container, bank...)
		container = append(container, song...)
	}
	path := filepath.Join(t.TempDir(), "collection.sndh")
	if err := os.WriteFile(path, container, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadSubtune(path, 2)
	if err != nil || p.Song.Patterns[0][0].Note != 72 {
		t.Fatalf("wrong native subtune selected: %v", err)
	}
	if _, err := LoadSubtune(path, 3); err == nil {
		t.Fatal("out-of-range subtune accepted")
	}
	if _, err := LoadSubtune(path, 0); err == nil {
		t.Fatal("zero-based command selection accepted")
	}
}
