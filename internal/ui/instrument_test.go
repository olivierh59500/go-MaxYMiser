package ui

import (
	"strings"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestExampleInstrumentViewsShowTheirLinkedSoundDefinitions(t *testing.T) {
	p := model.Demo()
	square := instrumentSequences(&p.Bank, 0)
	chord := instrumentSequences(&p.Bank, 1)
	drum := instrumentSequences(&p.Bank, 2)
	if square[1].values == chord[1].values || chord[1].values != "0000 0004 0007" {
		t.Fatalf("chord arpeggio is not visible: square=%+v chord=%+v", square[1], chord[1])
	}
	if drum[0].id != 4 || !strings.HasPrefix(drum[0].values, "000F 000C 0009 0006") || drum[3].values != "1000" || drum[4].values == square[4].values {
		t.Fatalf("drum envelope/mixer/noise were not selected: %+v", drum)
	}
	p.Bank.Sequences[3].Values[1] = 5
	if got := instrumentSequences(&p.Bank, 1)[1].values; got != "0000 0005 0007" {
		t.Fatalf("display used stale sequence values: %q", got)
	}
}

func TestInstrumentSelectionOpensAndEditsTheCorrectSequence(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("instrument:1")
	app.action("parameter:49")
	if app.instrument != 1 || app.entry != "03" {
		t.Fatalf("selection did not update the editable definition: index=%d entry=%q", app.instrument, app.entry)
	}
	app.modal = ""
	app.action("instrument-sequence:3")
	if app.tab != "Sequences" || app.sequence != 3 {
		t.Fatalf("sequence button opened the wrong editor: tab=%s sequence=%d", app.tab, app.sequence)
	}
	app.action("seq-value:1")
	app.entry = "0005"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Sequences[3].Values[1] != 5 || e.Project.Bank.Sequences[0].Values[1] != 0 {
		t.Fatal("editing a selected instrument changed another sequence")
	}
	app.action("instrument:2")
	app.action("parameter:48")
	if app.entry != "04" {
		t.Fatalf("drum still shows the previous instrument's volume link: %q", app.entry)
	}
}
