package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func sourceInspectionApp(t *testing.T) *App {
	t.Helper()
	app, err := New(model.Demo(), "current.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	score := ymimport.SourceScore{Player: "constructed-player", Rate: 50, Speed: 3, Frames: 128,
		Instruments: []ymimport.SourceInstrument{{ID: 0, Settings: []byte{0, 0, 1, 1, 0, 1}, VolumeSequence: []byte{15}, Arpeggio: ymimport.SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}}},
		Patterns:    []ymimport.SourcePattern{{ID: 7}},
		Events:      []ymimport.SourceEvent{{Channel: 0, Frame: 0, Note: 60, Instrument: 0, Retrigger: true}, {Channel: 0, Frame: 6, Note: 64, Instrument: 0}},
	}
	p, report, err := ymimport.SourceProject(score, 0, 128)
	if err != nil {
		t.Fatal(err)
	}
	app.sourceScore, app.sourcePreview, app.sourceReport = &score, p, &report
	app.sourcePath = filepath.Join(t.TempDir(), "original.sndh")
	if err := os.WriteFile(app.sourcePath, []byte("original source"), 0600); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestInspectingAndChangingASourceExcerptRetainsTheEditableComposition(t *testing.T) {
	app := sourceInspectionApp(t)
	app.synth.Edit(func(e *replay.Engine) { e.Play(false) })
	app.action("source:range")
	app.entry = "4:90"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Title != "First signal" || !e.Playing || app.projectPath != "current.mys" || app.dirty || app.sourceReport.StartFrame != 4 || app.sourceReport.EndFrame != 90 {
		t.Fatal("source inspection replaced the composition, audio or save destination")
	}
	previous := app.sourcePreview
	app.action("source:range")
	app.entry = "-1:90"
	app.applyModal()
	if app.sourcePreview != previous || app.sourceReport.StartFrame != 4 {
		t.Fatal("invalid source range changed the conversion")
	}
}

func TestSourceImportCreatesAnIndependentEditablePairAndKeepsOriginalLabels(t *testing.T) {
	app := sourceInspectionApp(t)
	source := app.sourcePath
	app.action("source:import")
	e, _ := app.synth.Snapshot()
	if app.tab != "Patterns" || !app.dirty || app.projectPath != "" || app.sourceScore == nil || app.sourceScore.Patterns[0].ID != 7 || len(e.Project.ReplaySource) != 0 {
		t.Fatal("source import lost its labels or retained the executable as a save target")
	}
	if e.Project.Bank.Instruments[0].Name() != "Source 00" || e.Project.Song.Patterns[e.Project.Song.Orders[0][0]][6].Note != 64 {
		t.Fatal("source import did not populate editable notes and instruments")
	}
	app.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Instruments[0].SetName("Edited sound") })
	if app.sourcePreview.Bank.Instruments[0].Name() != "Source 00" {
		t.Fatal("editing the imported score changed the retained conversion")
	}
	target := filepath.Join(t.TempDir(), "converted.mys")
	app.save(target)
	loaded, err := project.Load(target, "")
	if err != nil || loaded.Bank.Instruments[0].Name() != "Edited sound" {
		t.Fatalf("converted source cannot be saved as native data: %v", err)
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "original source" {
		t.Fatal("source import or save replaced the original executable")
	}
	app.action("new")
	if app.sourceScore != nil || app.sourcePreview != nil {
		t.Fatal("new project retained a stale source view")
	}
}
