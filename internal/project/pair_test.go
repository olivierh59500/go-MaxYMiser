package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestPairSaveRollsBackTheSongIfTheBankCommitFails(t *testing.T) {
	root := t.TempDir()
	song, bank := filepath.Join(root, "piece.mys"), filepath.Join(root, "piece.myv")
	os.WriteFile(song, []byte("old song"), 0600)
	os.WriteFile(bank, []byte("old bank"), 0600)
	failed := errors.New("simulated second rename failure")
	err := writeNativePairWithRename([]pairFile{{path: song, data: []byte("new song")}, {path: bank, data: []byte("new bank")}}, func(from, to string) error {
		if to == bank {
			return failed
		}
		return os.Rename(from, to)
	})
	if !errors.Is(err, failed) {
		t.Fatalf("missing commit error: %v", err)
	}
	for path, want := range map[string]string{song: "old song", bank: "old bank"} {
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != want {
			t.Fatalf("partial save changed %s: %q %v", path, raw, err)
		}
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatal("successful rollback left staged or backup files")
	}
}

func TestPairRollbackRestoresACompanionChangedByAFailedCommit(t *testing.T) {
	root := t.TempDir()
	song, bank := filepath.Join(root, "piece.mys"), filepath.Join(root, "piece.myv")
	os.WriteFile(song, []byte("old song"), 0600)
	os.WriteFile(bank, []byte("old bank"), 0600)
	err := writeNativePairWithRename([]pairFile{{path: song, data: []byte("new song")}, {path: bank, data: []byte("new bank")}}, func(from, to string) error {
		if to == bank {
			if err := os.Rename(from, to); err != nil {
				return err
			}
			return errors.New("failure reported after replacement")
		}
		return os.Rename(from, to)
	})
	if err == nil {
		t.Fatal("simulated commit failure was ignored")
	}
	a, _ := os.ReadFile(song)
	b, _ := os.ReadFile(bank)
	if string(a) != "old song" || string(b) != "old bank" {
		t.Fatal("rollback restored only the first half of the pair")
	}
}

func TestFailedFirstPairSaveDoesNotLeaveHalfOfANewProject(t *testing.T) {
	root := t.TempDir()
	song, bank := filepath.Join(root, "new.mys"), filepath.Join(root, "new.myv")
	err := writeNativePairWithRename([]pairFile{{path: song, data: []byte("song")}, {path: bank, data: []byte("bank")}}, func(from, to string) error {
		if to == bank {
			return errors.New("bank failed")
		}
		return os.Rename(from, to)
	})
	if err == nil {
		t.Fatal("simulated write failure was ignored")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("failed pair creation left a partial project")
	}
}

func TestPairSavePreflightsAnInvalidCompanionBeforeReplacingTheSong(t *testing.T) {
	root := t.TempDir()
	song := filepath.Join(root, "piece.mys")
	old := []byte("existing song")
	os.WriteFile(song, old, 0600)
	os.Mkdir(filepath.Join(root, "piece.myv"), 0700)
	if err := Save(model.New(), song); err == nil {
		t.Fatal("directory was accepted as a voice-bank target")
	}
	got, _ := os.ReadFile(song)
	if !bytes.Equal(got, old) {
		t.Fatal("bank failure left a newly written song")
	}
}

func TestNativePairSavePreservesOriginalFilenameCase(t *testing.T) {
	root := t.TempDir()
	song, bank := filepath.Join(root, "MUSIC.MYS"), filepath.Join(root, "MUSIC.MYV")
	if err := Save(model.New(), song); err != nil {
		t.Fatal(err)
	}
	// Rename the initially new lower-case bank to the native upper-case name.
	if err := os.Rename(filepath.Join(root, "MUSIC.myv"), bank); err != nil {
		t.Fatal(err)
	}
	p, err := Load(song, "")
	if err != nil {
		t.Fatal(err)
	}
	p.Song.Patterns[0][0].Note = 72
	p.Bank.Instruments[0].SetName("Saved sound")
	if err := Save(p, song); err != nil {
		t.Fatal(err)
	}
	got, err := Load(song, "")
	if err != nil || got.Song.Patterns[0][0].Note != 72 || got.Bank.Instruments[0].Name() != "Saved sound" {
		t.Fatalf("case-preserved pair did not reopen with its edited bank: %v", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatal("save created duplicate names with different extensions")
	}
}

func TestPairSaveKeepsActualCompanionCaseAndPermissions(t *testing.T) {
	root := t.TempDir()
	song, bank := filepath.Join(root, "piece.mys"), filepath.Join(root, "piece.myv")
	if err := Save(model.New(), song); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(song, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bank, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(model.New(), song); err != nil {
		t.Fatal(err)
	}
	for path, wanted := range map[string]os.FileMode{song: 0640, bank: 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != wanted {
			t.Fatalf("save changed permissions of %s: %v", path, err)
		}
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if entry.Name() != "piece.mys" && entry.Name() != "piece.myv" {
			t.Fatalf("save changed actual native filename case: %s", entry.Name())
		}
	}
}

func TestInvalidNativeReferencesNeverReplaceAnExistingPair(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "piece.mys")
	if err := Save(model.New(), path); err != nil {
		t.Fatal(err)
	}
	oldSong, _ := os.ReadFile(path)
	oldBank, _ := os.ReadFile(filepath.Join(root, "piece.myv"))
	p := model.New()
	p.Song.Orders[0][0] = 17
	if err := Save(p, path); err == nil {
		t.Fatal("unreloadable music was saved")
	}
	song, _ := os.ReadFile(path)
	bank, _ := os.ReadFile(filepath.Join(root, "piece.myv"))
	if !bytes.Equal(song, oldSong) || !bytes.Equal(bank, oldBank) {
		t.Fatal("invalid native references replaced a valid pair")
	}
}

func TestNativePairReopensAMixedCaseCompanion(t *testing.T) {
	root := t.TempDir()
	song := filepath.Join(root, "Piece.Mys")
	p := model.New()
	p.Bank.Instruments[0].SetName("Companion voice")
	if err := Save(p, song); err != nil {
		t.Fatal(err)
	}
	bank := filepath.Join(root, "Piece.MyV")
	if err := os.Rename(filepath.Join(root, "Piece.myv"), bank); err != nil {
		t.Fatal(err)
	}
	got, err := Load(song, "")
	if err != nil || got.Bank.Instruments[0].Name() != "Companion voice" {
		t.Fatalf("mixed-case companion was not loaded: %v", err)
	}
}

func TestVirtualNativePairUsesTheSameMixedCaseLookup(t *testing.T) {
	p := model.New()
	p.Bank.Instruments[0].SetName("Virtual voice")
	song, _ := native.EncodeSong(p.Song)
	bank, _ := native.EncodeVoiceBank(p.Bank)
	files := fstest.MapFS{"songs/Piece.Mys": {Data: song}, "songs/Piece.MyV": {Data: bank}}
	got, err := LoadFS(files, "songs/Piece.Mys", "")
	if err != nil || got.Bank.Instruments[0].Name() != "Virtual voice" {
		t.Fatalf("virtual paired-bank lookup lost filename case: %v", err)
	}
}
