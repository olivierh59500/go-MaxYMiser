package ui

import (
	"testing"
	"testing/fstest"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestDroppedSongAndVoiceBankOpenAsOneValidatedProject(t *testing.T) {
	p := model.New()
	p.Song.Orders[0] = [4]byte{255, 1, 2, 255}
	p.Song.Patterns[1][0] = model.Cell{Note: 75, Instrument: 2}
	p.Bank.Instruments[1].SetName("Dropped voice")
	song, _ := native.EncodeSong(p.Song)
	bank, _ := native.EncodeVoiceBank(p.Bank)
	files := fstest.MapFS{"piece.mys": {Data: song}, "piece.myv": {Data: bank}}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.directory = t.TempDir()
	directory := app.directory
	if err := app.OpenDroppedMusic(files); err != nil {
		t.Fatal(err)
	}
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[1].Name() != "Dropped voice" || e.Project.Song.Patterns[1][0].Note != 75 || app.pattern != 1 || app.channel != 1 || app.tab != "Patterns" {
		t.Fatal("drop did not use the paired bank and native opening workflow")
	}
	if app.projectPath != "" || app.directory != directory {
		t.Fatal("virtual dropped paths became an unsafe save target or browser directory")
	}
}

func TestBrokenDroppedCompanionDoesNotPartiallyReplaceThePlayingSong(t *testing.T) {
	song, _ := native.EncodeSong(model.New().Song)
	files := fstest.MapFS{"new.mys": {Data: song}, "new.myv": {Data: []byte("broken")}}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.synth.Edit(func(e *replay.Engine) {
		e.Play(false)
		for range 17 {
			e.Tick()
		}
	})
	before, _ := app.synth.Snapshot()
	if err := app.OpenDroppedMusic(files); err == nil {
		t.Fatal("broken companion bank was accepted")
	}
	after, _ := app.synth.Snapshot()
	if !after.Playing || after.Row != before.Row || after.Registers != before.Registers || after.Project.Title != before.Project.Title || after.Project.Bank.Instruments != before.Project.Bank.Instruments {
		t.Fatal("failed drop changed the song or interrupted its audio")
	}
}
