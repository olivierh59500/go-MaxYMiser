package project

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestStreamedFileCommitFailurePreservesTheCompleteOriginal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "output.wav")
	old := []byte("previous complete audio")
	if err := os.WriteFile(path, old, 0640); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("simulated streaming commit failure")
	err := writeFileWithRename(path, func(w io.Writer) error {
		_, err := io.WriteString(w, "fully rendered replacement audio")
		return err
	}, func(from, to string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("stream commit failure was not returned: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, old) {
		t.Fatalf("stream commit failure damaged the original: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed stream commit left temporary files: %v", err)
	}
}

func TestSingleFileCommitFailurePreservesThePreviousFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "music.sndh")
	old := []byte("previous complete output")
	if err := os.WriteFile(path, old, 0640); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("simulated rename failure")
	err := atomicWriteWithRename(path, []byte("updated output"), func(from, to string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("commit failure was not returned: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, old) {
		t.Fatalf("failed save changed the previous output: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("failed save changed original permissions: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed commit left a staged file: %v", err)
	}
}

func TestSingleFileSaveRejectsDirectories(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sound.myi")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("sound")); err == nil {
		t.Fatal("a directory was accepted as an ordinary file")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("failed save changed the directory or left temporary files: %v", err)
	}
}

func TestSingleFileSavePreservesExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MYM.CNF")
	old := native.Configuration{0: 255, 17: 7}
	if err := os.WriteFile(path, old[:], 0640); err != nil {
		t.Fatal(err)
	}
	updated := old
	updated[10] = 255
	if err := SaveConfiguration(updated, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, updated[:]) {
		t.Fatalf("saved preferences differ from the edited data: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("saving preferences changed existing file permissions: %v", err)
	}
}

func TestSingleFileSaveRejectsSymlinksWithoutReplacingThem(t *testing.T) {
	root := t.TempDir()
	target, link := filepath.Join(root, "personal.cnf"), filepath.Join(root, "MYM.CNF")
	old := native.Configuration{0: 255, 17: 7}
	if err := os.WriteFile(target, old[:], 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfiguration(native.Configuration{}, link); err == nil {
		t.Fatal("a symlink was accepted as an ordinary save target")
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("failed save replaced the symlink: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(raw, old[:]) {
		t.Fatalf("failed save changed the linked file: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed save left temporary files: %v", err)
	}
}
