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
