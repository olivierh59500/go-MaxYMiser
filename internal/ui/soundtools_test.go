package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSoundExportsCanUpdateExistingFilesWithoutSavingTheProject(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.SelectInstrument(1)
	root := t.TempDir()
	instrumentPath := filepath.Join(root, "chord.myi")
	samplePath := filepath.Join(root, "drum.pcm")
	for iteration := 0; iteration < 2; iteration++ {
		name := []string{"First sound", "Updated sound"}[iteration]
		pcm := [][]byte{{0, 127, 128, 255}, {128, 127}}[iteration]
		app.synth.Edit(func(e *replay.Engine) {
			e.Project.Bank.Instruments[1].SetName(name)
			e.Project.Bank.Samples[0].PCM = append([]byte(nil), pcm...)
		})
		app.dirty, app.icePacking = true, iteration == 1
		app.modal, app.entry = "Save instrument (.myi)", instrumentPath
		app.applyModal()
		raw, err := os.ReadFile(instrumentPath)
		if err != nil {
			t.Fatal(err)
		}
		file, err := native.DecodeInstrument(raw)
		if err != nil || file.Instrument.Name() != name {
			t.Fatalf("instrument save did not update its existing file: %v (%s)", err, app.status)
		}
		app.modal, app.entry = "Save signed PCM sample", samplePath
		app.applyModal()
		raw, err = os.ReadFile(samplePath)
		if err != nil || !bytes.Equal(raw, pcm) {
			t.Fatalf("sample save did not update its existing file: %v (%s)", err, app.status)
		}
		if !app.dirty || app.projectPath != "" || app.instrument != 1 || app.sample != 0 {
			t.Fatal("sound export changed composition save state or editor selection")
		}
	}
}

func TestMYIImportRetainsScoreOnlySequenceAndLiveSampleReferences(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Effect1: 'L', Parameter1: 3, Effect2: 'A', Parameter2: 4}
	app, err := New(p, "current.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	file := native.InstrumentFile{Version: 3, Sample: []byte{0, 127, 128, 255}}
	file.Instrument.SetName("Imported")
	file.Instrument[36] = 1
	file.Sequences[0] = model.Sequence{Length: 2, Values: [63]uint16{15, 8}, Repeat: 1}
	raw, err := native.EncodeInstrument(file)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "import.myi")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	app.synth.Edit(func(e *replay.Engine) {
		e.Voices[0].Parameters[32] = 5
		e.DMA[0].Sample = 2
	})
	app.instrument = 7
	app.modal, app.entry = "Load instrument (.myi)", path
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Instruments[7].Name() != "Imported" || e.Project.Bank.Instruments[7][48] != 6 || e.Project.Bank.Instruments[7][36] != 3 {
		t.Fatalf("MYI allocation ignored score/live references: %s", app.status)
	}
	if e.Project.Bank.Sequences[3].Values[0] != 0 || e.Project.Bank.Sequences[4].Values[0] != 0 || e.Project.Bank.Sequences[5].Values[0] != 0 || len(e.Project.Bank.Samples[1].PCM) != 0 {
		t.Fatal("MYI import overwrote a referenced empty definition")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Instruments[7].Name() != "" || len(e.Project.Bank.Samples[2].PCM) != 0 || e.Project.Song.Patterns[0][0].Parameter1 != 3 {
		t.Fatal("undo did not restore the pre-import bank and score references")
	}
}

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

func TestSampleLengthEditCanBeUndoneAndInvalidInputKeepsTheBank(t *testing.T) {
	p := model.Demo()
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("sample-length")
	if app.entry != "4" {
		t.Fatal("sample length control did not show the selected sample")
	}
	app.entry = "7"
	app.applyModal()
	e, _ := app.synth.Snapshot()
	if !bytes.Equal(e.Project.Bank.Samples[0].PCM, []byte{0, 127, 128, 255, 0, 0, 0}) || !app.dirty {
		t.Fatal("sample length input did not enter an editable change")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if !bytes.Equal(e.Project.Bank.Samples[0].PCM, []byte{0, 127, 128, 255}) {
		t.Fatal("sample resizing could not restore the prior payload")
	}
	app.action("sample-length")
	app.entry = "32769"
	app.applyModal()
	e, _ = app.synth.Snapshot()
	if len(e.Project.Bank.Samples[0].PCM) != 4 {
		t.Fatal("oversized length input replaced the selected sample")
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

func TestYMiseSampleOperationCanBeUndone(t *testing.T) {
	p := model.New()
	p.Bank.Samples[0].PCM = []byte{0, 50, 83, 128, 190, 255}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("sample-ymise")
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Samples[0].PCM[0] != 7 || e.Project.Bank.Samples[0].PCM[3] != 128 {
		t.Fatal("YMise did not use native DAC quantization")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Samples[0].PCM[0] != 0 || e.Project.Bank.Samples[0].PCM[1] != 50 {
		t.Fatal("YMise could not be undone")
	}
}
