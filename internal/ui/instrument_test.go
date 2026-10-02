package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
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

func TestInstrumentComponentMasksKeepOtherEffectsAndSupportUndo(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("instrument:1")
	app.action("mask:17:1")
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[1][17] != 5 || e.Project.Bank.Instruments[0][17] != 7 {
		t.Fatal("mask toggle affected wrong components or instrument")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Instruments[1][17] != 7 {
		t.Fatal("component mask could not be undone")
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
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[3].Values[1] != 4 {
		t.Fatal("instrument sequence edit could not be undone")
	}
	app.restore(true)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[3].Values[1] != 5 {
		t.Fatal("instrument sequence edit could not be redone")
	}
	app.action("instrument:2")
	app.action("parameter:48")
	if app.entry != "04" {
		t.Fatalf("drum still shows the previous instrument's volume link: %q", app.entry)
	}
}

func TestEditedInstrumentSavesAsAnEditableNativePair(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("instrument:1")
	app.action("parameter:33")
	app.entry = "40"
	app.applyModal()
	path := filepath.Join(t.TempDir(), "edited.mys")
	app.save(path)
	if app.dirty || app.projectPath != path {
		t.Fatalf("save did not succeed: %s", app.status)
	}
	loaded, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Bank.Instruments[1][33] != 0x40 || loaded.Bank.Instruments[0][33] != 0 || loaded.Bank.Instruments[1][49] != 3 {
		t.Fatal("selected instrument definition was not preserved in the native bank")
	}
}

func TestYMReconstructionKeepsTheOriginalAvailableDuringPatternPlayback(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	const frames = 64
	data := make([]byte, 4+frames*14)
	copy(data, "YM3!")
	for i := 0; i < frames; i++ {
		data[4+i] = 28
		data[4+frames+i] = 1
		data[4+7*frames+i] = 62
		data[4+8*frames+i] = 15
		data[4+13*frames+i] = 255
	}
	ympath := filepath.Join(t.TempDir(), "reference.ym")
	if err = os.WriteFile(ympath, data, 0600); err != nil {
		t.Fatal(err)
	}
	app.projectPath = ympath
	if err = app.LoadYM(ympath); err != nil {
		t.Fatal(err)
	}
	if app.projectPath != "" {
		t.Fatal("native save inherited the YM reference path")
	}
	app.action("ym:infer")
	if app.ymReport == nil || len(app.ymData) != len(data) {
		t.Fatalf("reconstruction lost its reference: %s", app.status)
	}
	app.action("pattern")
	ref, ok := app.synth.Reference()
	if !ok || ref.Active {
		t.Fatal("pattern playback unloaded the original YM")
	}
	app.action("ym:reference")
	ref, ok = app.synth.Reference()
	if !ok || !ref.Active || !ref.Playing {
		t.Fatal("original reference could not be selected again")
	}
}
