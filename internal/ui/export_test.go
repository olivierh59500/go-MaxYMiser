package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestWAVExportUsesChosenDurationAndLeavesEditorUsable(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.modal, app.entry = "Setting export-duration", "0.1"
	app.applyModal()
	if app.exportDuration != 100*time.Millisecond {
		t.Fatal("export duration was not applied")
	}
	path := filepath.Join(t.TempDir(), "short.wav")
	app.modal, app.entry = "Export WAV", path
	app.applyModal()
	if !app.exporting {
		t.Fatal("export did not start asynchronously")
	}
	// A UI operation can proceed while the cloned project renders separately.
	app.action("instrument:1")
	select {
	case err := <-app.exportResults:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bounded short export did not complete")
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) != 44+4800*4 || binary.LittleEndian.Uint32(raw[40:44]) != 4800*4 {
		t.Fatalf("WAV did not use selected duration: bytes=%d err=%v", len(raw), err)
	}
}
