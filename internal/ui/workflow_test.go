package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestCreateEditSaveReopenAndExportRetainsAnOriginalComposition(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("new")
	app.editing, app.channel, app.pattern, app.row = true, 0, 0, 0
	app.enterNote(60)
	app.row = 4
	app.enterNote(64)
	app.row = 8
	app.enterNote(67)
	app.action("parameter:38")
	app.entry = "03"
	app.applyModal()
	if !app.dirty {
		t.Fatal("editing did not mark the new composition as modified")
	}
	before, _ := app.synth.Snapshot()
	wantSong, err := native.EncodeSong(before.Project.Song)
	if err != nil {
		t.Fatal(err)
	}
	wantBank, err := native.EncodeVoiceBank(before.Project.Bank)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	app.directory = root
	app.action("save-as")
	app.entry = "first-composition.mys"
	app.applyModal()
	path := filepath.Join(root, "first-composition.mys")
	if app.dirty || app.projectPath != path {
		t.Fatalf("new composition save failed: %s", app.status)
	}
	app.action("new")
	if err := app.OpenMusic(path); err != nil {
		t.Fatal(err)
	}
	after, _ := app.synth.Snapshot()
	gotSong, err := native.EncodeSong(after.Project.Song)
	if err != nil {
		t.Fatal(err)
	}
	gotBank, err := native.EncodeVoiceBank(after.Project.Bank)
	if err != nil || string(gotSong) != string(wantSong) || string(gotBank) != string(wantBank) || after.Project.Bank.Instruments[0][38] != 3 {
		t.Fatal("reopening changed entered notes or instrument parameters")
	}
	app.exportDuration = 200 * time.Millisecond
	app.modal, app.entry = "Export WAV", filepath.Join(root, "first-composition.wav")
	app.applyModal()
	deadline := time.Now().Add(5 * time.Second)
	for !app.pollExportResult() {
		if time.Now().After(deadline) {
			t.Fatal("composition export did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	raw, err := os.ReadFile(filepath.Join(root, "first-composition.wav"))
	if err != nil || len(raw) != 44+9600*4 || string(raw[:4]) != "RIFF" {
		t.Fatalf("invalid exported stereo WAV: %v", err)
	}
	nonzero := false
	for at := 44; at+2 <= len(raw); at += 2 {
		if binary.LittleEndian.Uint16(raw[at:]) != 0 {
			nonzero = true
			break
		}
	}
	if !nonzero || app.dirty || app.projectPath != path || app.status != "WAV export complete" {
		t.Fatal("export was silent or changed the saved composition state")
	}
}
