package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestWAVReExportKeepsLivePlaybackAndUsesTheCurrentScore(t *testing.T) {
	app, err := New(model.Demo(), "current.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.synth.Edit(func(e *replay.Engine) { e.Play(false) })
	path := filepath.Join(t.TempDir(), "mix.wav")
	for iteration := 0; iteration < 2; iteration++ {
		app.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[0][0].Note = byte(60 + iteration*12) })
		app.dirty = true
		app.exportDuration = time.Duration(30-iteration*10) * time.Millisecond
		app.modal, app.entry = "Export WAV", path
		app.applyModal()
		deadline := time.Now().Add(5 * time.Second)
		for !app.pollExportResult() {
			if time.Now().After(deadline) {
				t.Fatal("bounded re-export did not finish")
			}
			time.Sleep(time.Millisecond)
		}
		if app.exporting || app.status != "WAV export complete" {
			t.Fatalf("re-export failed or did not release the renderer: %s", app.status)
		}
		raw, err := os.ReadFile(path)
		frames := int64(app.exportDuration) * 48000 / int64(time.Second)
		if err != nil || int64(len(raw)) != 44+frames*4 {
			t.Fatalf("re-export did not replace the chosen duration: %v", err)
		}
		e, _ := app.synth.Snapshot()
		if !e.Playing || !app.dirty || app.projectPath != "current.mys" || e.Project.Song.Patterns[0][0].Note != byte(60+iteration*12) {
			t.Fatal("background export changed live playback, score edits or save state")
		}
	}
}

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

func TestSNDHExportRequiresAValidatedLocallySuppliedReplay(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	path := filepath.Join(t.TempDir(), "song.snd")
	app.modal, app.entry = "Export native SNDH", path
	app.applyModal()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("export created a non-playable SNDH without a replay prefix")
	}
	bad := filepath.Join(t.TempDir(), "bad.snd")
	os.WriteFile(bad, []byte("SNDH"), 0600)
	app.modal, app.entry = "Load SNDH replay template", bad
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if len(e.Project.ReplaySource) != 0 || e.Project.Title != "First signal" {
		t.Fatal("invalid replay template changed the current composition")
	}
	if _, err := native.ParseSNDHTemplate([]byte("not a template")); err == nil {
		t.Fatal("invalid replay template accepted")
	}
}

func TestICEOptionSavesACompressedEditablePair(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("ice-packing")
	path := filepath.Join(t.TempDir(), "packed.mys")
	app.save(path)
	raw, err := os.ReadFile(path)
	if err != nil || string(raw[:4]) != "ICE!" {
		t.Fatalf("UI native save ignored ICE: %v", err)
	}
	if _, err = native.DecodeSong(raw); err != nil {
		t.Fatal(err)
	}
}
