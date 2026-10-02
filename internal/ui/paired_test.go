package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func TestPairedProfileLabelsYMWithoutReplacingItByTheSourceScore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "music.ym")
	data := make([]byte, 4+14*64)
	copy(data, "YM3!")
	for i := 0; i < 64; i++ {
		data[4+i], data[4+64+i], data[4+7*64+i] = 28, 1, 62
		data[4+8*64+i], data[4+13*64+i] = 15, 255
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	trace, err := ymimport.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	events := ymimport.ExtractEvents(trace, 0)
	profile := ymimport.PairedProfile{Version: 1, Source: ymimport.SourceScore{Player: "verified-test-bank", Rate: 50, Instruments: []ymimport.SourceInstrument{{ID: 0}}}, Prototypes: []ymimport.PairedPrototype{{Instrument: 0, Features: events[0].Features, Examples: 3}}}
	profilePath := filepath.Join(root, "profile.json")
	if err := ymimport.SavePairedProfile(profile, profilePath); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.modal, app.entry = "Load paired source profile (.json)", profilePath
	app.applyModal()
	if app.pairedProfile == nil {
		t.Fatal("paired source profile was not loaded")
	}
	if err := app.LoadYM(path); err != nil {
		t.Fatal(err)
	}
	app.action("ym:infer")
	if app.ymReport == nil || app.ymReport.SourcePlayer != "verified-test-bank" || len(app.ymReport.SourceLabels) == 0 {
		t.Fatal("YM reconstruction omitted source-labelled evidence")
	}
	if _, ok := app.synth.Reference(); !ok {
		t.Fatal("source-labelled reconstruction discarded the independent YM reference")
	}
	before := app.pairedProfile
	app.modal, app.entry = "Load paired source profile (.json)", path
	app.applyModal()
	if app.pairedProfile != before {
		t.Fatal("invalid profile replaced the validated profile")
	}
}
