package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func TestPatternEvidencePagingDoesNotRewriteTheComposition(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.ymReport = &ymimport.Report{SourcePatterns: make([]ymimport.PatternEvidence, 13)}
	app.action("ym:source-patterns")
	if !app.ymPatternView {
		t.Fatal("source pattern workspace did not open")
	}
	for range 4 {
		app.action("ym:patterns-next")
	}
	if app.ymPatternPage != 2 {
		t.Fatal("evidence page exceeded its available passages")
	}
	app.action("ym:patterns-prev")
	if app.ymPatternPage != 1 {
		t.Fatal("evidence page did not go backwards")
	}
	app.action("ym:pattern-hit:99")
	e, _ := app.synth.Snapshot()
	if e.Project.Title != "First signal" || app.dirty {
		t.Fatal("browsing source candidates edited the current composition")
	}
}
