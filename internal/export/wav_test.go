package export

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type fragmentedReader struct {
	data []byte
	size int
}

type audioReaderFunc func([]byte) (int, error)

func (fn audioReaderFunc) Read(p []byte) (int, error) { return fn(p) }

func TestExternalClockExportReportsMissingTimingWithoutCreatingSilentAudio(t *testing.T) {
	for _, mode := range []byte{1, 3} {
		p := model.Demo()
		p.Song.State[31] = mode
		path := filepath.Join(t.TempDir(), "external.wav")
		if err := WAV(p, path, time.Second); err == nil {
			t.Fatal("offline rendering accepted a song with no clock pulses")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("missing external timing left a misleading WAV output")
		}
	}
}

func (r *fragmentedReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), min(r.size, len(r.data)))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func TestWAVExportHandlesShortAudioReadsWithoutResidualSamples(t *testing.T) {
	data := bytes.Repeat([]byte{1, 2, 3, 4}, 480)
	path := filepath.Join(t.TempDir(), "fragmented.wav")
	if err := writeAudio(&fragmentedReader{data: append([]byte(nil), data...), size: 7}, path, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) != 44+len(data) || !bytes.Equal(raw[44:], data) || binary.LittleEndian.Uint32(raw[40:44]) != uint32(len(data)) {
		t.Fatalf("short reads changed exported PCM: %v", err)
	}
}

func TestIncompleteAudioExportRemovesOnlyItsOwnPartialFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "short.wav")
	if err := writeAudio(bytes.NewReader([]byte{1, 2, 3, 4}), path, 10*time.Millisecond); err == nil {
		t.Fatal("truncated audio was accepted as a full export")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed audio export retained a corrupt file")
	}
	os.WriteFile(path, []byte("existing"), 0600)
	if err := writeAudio(bytes.NewReader(nil), path, 10*time.Millisecond); err == nil {
		t.Fatal("existing output was overwritten")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "existing" {
		t.Fatal("failed export removed an unrelated existing output")
	}
}

func TestWAVCanReplaceAnExistingExportWithItsNewLengthAndPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mix.wav")
	first := bytes.Repeat([]byte{1, 2, 3, 4}, 960)
	if err := writeAudio(bytes.NewReader(first), path, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	updated := bytes.Repeat([]byte{4, 3, 2, 1}, 480)
	if err := writeAudio(bytes.NewReader(updated), path, 10*time.Millisecond); err != nil {
		t.Fatalf("an existing presentation could not be re-exported: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) != 44+len(updated) || !bytes.Equal(raw[44:], updated) || binary.LittleEndian.Uint32(raw[40:44]) != uint32(len(updated)) {
		t.Fatalf("re-export retained previous samples or stale duration data: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("re-export changed existing file permissions: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("re-export left temporary output files: %v", err)
	}
}

func TestFailedWAVReExportKeepsTheCompletePreviousAudio(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mix.wav")
	first := bytes.Repeat([]byte{1, 2, 3, 4}, 480)
	if err := writeAudio(bytes.NewReader(first), path, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Enough bytes for one rendered chunk, followed by an incomplete chunk.
	partial := bytes.Repeat([]byte{4, 3, 2, 1}, 4100)
	if err := writeAudio(bytes.NewReader(partial), path, time.Second); err == nil {
		t.Fatal("an incomplete replacement was accepted as a finished render")
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, old) {
		t.Fatalf("failed re-export damaged the last complete audio: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed re-export retained staged audio: %v", err)
	}
}

func TestWAVKeepsThePreviousFileReadableUntilRenderingFinishes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mix.wav")
	first := bytes.Repeat([]byte{1, 2, 3, 4}, 480)
	if err := writeAudio(bytes.NewReader(first), path, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := bytes.NewReader(bytes.Repeat([]byte{4, 3, 2, 1}, 960))
	reads := 0
	reader := audioReaderFunc(func(p []byte) (int, error) {
		reads++
		raw, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(raw, old) {
			t.Fatalf("rendering exposed incomplete replacement audio: %v", err)
		}
		return updated.Read(p)
	})
	if err := writeAudio(reader, path, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if reads == 0 {
		t.Fatal("audio was replaced without rendering its source")
	}
}

func TestTrackerWAVReExportRendersTheEditedComposition(t *testing.T) {
	p := model.Demo()
	path := filepath.Join(t.TempDir(), "song.wav")
	if err := WAV(p.Clone(), path, 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Song.Patterns[0][0].Note = 72
	if err := WAV(p.Clone(), path, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil || len(updated) != 44+960*4 {
		t.Fatalf("edited tracker re-export has incorrect duration: %v", err)
	}
	if bytes.Equal(updated[44:], first[44:len(updated)]) {
		t.Fatal("re-export retained audio from the earlier tracker notes")
	}
	if bytes.Equal(updated[44:], make([]byte, len(updated)-44)) {
		t.Fatal("real tracker re-export produced silent PCM")
	}
}

func TestWAVTargetPreflightDoesNotReadAudioForADirectoryOrSymlink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "original.wav")
	if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	link, directory := filepath.Join(root, "link.wav"), filepath.Join(root, "directory.wav")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, directory} {
		reader := audioReaderFunc(func(p []byte) (int, error) {
			t.Fatal("an invalid WAV destination started audio rendering")
			return 0, io.EOF
		})
		if err := writeAudio(reader, path, time.Second); err == nil {
			t.Fatal("a nonregular WAV target was accepted")
		}
	}
	raw, err := os.ReadFile(file)
	if err != nil || string(raw) != "original" {
		t.Fatalf("invalid WAV save changed its linked target: %v", err)
	}
}
