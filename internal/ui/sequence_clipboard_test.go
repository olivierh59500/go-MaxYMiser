package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSequenceClipboardPreservesLoopMetadataAndCanUndoCut(t *testing.T) {
	p := model.New()
	p.Bank.Sequences[7] = model.Sequence{Values: [63]uint16{0xffff, 7, 12}, Length: 3, Repeat: 1}
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.sequence = 7
	app.action("sequence-copy")
	app.sequence = 63
	app.action("sequence-paste")
	e, _ := app.synth.Snapshot()
	if e.Project.Bank.Sequences[63] != p.Bank.Sequences[7] || e.Project.Bank.SequenceCount != 64 || !app.dirty {
		t.Fatal("sequence paste lost signed words, timing, looping or serialized count")
	}
	app.action("sequence-cut")
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[63].Length != 1 || e.Project.Bank.Sequences[63].Values[0] != 0 {
		t.Fatal("sequence cut did not clear the selected definition")
	}
	app.restore(false)
	e, _ = app.synth.Snapshot()
	if e.Project.Bank.Sequences[63].Repeat != 1 || e.Project.Bank.Sequences[63].Values[0] != 0xffff {
		t.Fatal("sequence cut could not be undone")
	}
}

func TestEditingAHeldSequenceWordRefreshesTheLiveSharedSound(t *testing.T) {
	p := model.New()
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.AuditionInstrument(69)
	app.Synth().Edit(func(e *replay.Engine) {
		for range 3 {
			e.Tick()
		}
	})
	app.sequence = 1
	app.modal, app.entry = "Sequence word 0", "0009"
	app.applyModal()
	app.Synth().Edit(func(e *replay.Engine) { e.Tick() })
	e, _ := app.synth.Snapshot()
	if e.Registers[8] != 9 {
		t.Fatal("sequence word edit did not refresh a held live volume")
	}
}
