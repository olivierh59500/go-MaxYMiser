package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestGeneratorAndMorphUpdateSelectedSequencesAndUndo(t *testing.T) {
	p := model.Demo()
	p.Bank.Sequences[7] = model.Sequence{Length: 6, Repeat: 5, Values: [63]uint16{3, 3, 3, 3, 3, 3}}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.sequence = 4
	app.generatorShape, app.generatorLow, app.generatorHigh = edit.Ramp, "000F", "0000"
	app.action("gen-apply")
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Sequences[4].Values[0] != 15 || e.Project.Bank.Sequences[4].Values[5] != 0 || e.Project.Bank.Sequences[4].Repeat != 5 {
		t.Fatalf("generator failed: %s", app.status)
	}
	app.morphDestination = "07"
	app.action("gen-morph")
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[5].Values[0] != 11 || e.Project.Bank.Sequences[6].Values[0] != 7 {
		t.Fatalf("morph did not populate intermediate definitions: %s", app.status)
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[5].Values[0] != 0x1000 {
		t.Fatal("morph could not be undone")
	}
}

func TestSampleToolsKeepUndoAndSaveEditedPCM(t *testing.T) {
	p := model.Demo()
	p.Bank.Samples[0].PCM = []byte{0, 64, 0, 192}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("sample-tune")
	app.entry = "-12"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if len(e.Project.Bank.Samples[0].PCM) != 8 || e.Project.Bank.Samples[0].PCM[1] != 32 {
		t.Fatal("tuning did not apply interpolation")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if len(e.Project.Bank.Samples[0].PCM) != 4 {
		t.Fatal("sample edit could not be undone")
	}
	app.action("sample-trim")
	app.entry = "1,2"
	app.applyModal()
	app.action("sample-save")
	path := filepath.Join(t.TempDir(), "sample.pcm")
	app.entry = path
	app.applyModal()
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 2 || data[0] != 64 || data[1] != 0 {
		t.Fatalf("edited PCM save failed: %v %v", data, err)
	}
	app.action("sample-preview")
	e, _ = app.synth.Snapshot()
	if e.Playing || e.DMA[0].Sample != 1 || e.DMA[0].Note != 60 {
		t.Fatal("sample preview did not reach the selected PCM voice")
	}
}

func TestMYIButtonsSaveAndImportAnIndependentSound(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SelectInstrument(1)
	app.action("instrument-save")
	path := filepath.Join(t.TempDir(), "chord.myi")
	app.entry = path
	app.applyModal()
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	app.SelectInstrument(7)
	app.action("instrument-load")
	app.entry = path
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[7].Name() != "Chord pulse" || e.Project.Bank.Instruments[1][49] != 3 {
		t.Fatalf("MYI did not import without altering the source: %s", app.status)
	}
	arp := e.Project.Bank.Instruments[7][49]
	if e.Project.Bank.Sequences[arp].Values[1] != 4 {
		t.Fatal("MYI import lost the arpeggio")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Instruments[7].Name() != "" {
		t.Fatal("MYI import could not be undone")
	}
}
