package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestEditingAHeldSharedSequenceUpdatesBothVoicesWithoutRetriggering(t *testing.T) {
	p := model.New()
	p.Bank.Sequences[1] = model.Sequence{Values: [63]uint16{15, 10, 5}, Length: 3, Repeat: 2}
	e := New(p)
	e.Trigger(0, 69, 1)
	e.Trigger(1, 72, 1)
	for range 5 {
		e.Tick()
	}
	if e.Registers[8] != 5 || e.Registers[9] != 5 {
		t.Fatal("fixture did not reach its held tail")
	}
	before := e.Voices[0].PreviewTriggers
	p.Bank.Sequences[1].Values[2] = 9
	e.RefreshSequence(1)
	e.Tick()
	if e.Registers[8] != 9 || e.Registers[9] != 9 || e.Voices[0].PreviewTriggers != before || e.Voices[0].Triggered {
		t.Fatal("sequence edit did not refresh the held values or retriggered a note")
	}
	if e.Voices[0].SeqIndex[0] != 2 {
		t.Fatal("shared sequence edit restarted its phase")
	}
}

func TestShorteningAPlayingSequenceClampsItsCurrentStep(t *testing.T) {
	p := model.New()
	p.Bank.Sequences[1] = model.Sequence{Values: [63]uint16{15, 10, 5}, Length: 3, Repeat: 0}
	e := New(p)
	e.Trigger(0, 69, 1)
	e.Tick()
	e.Tick()
	p.Bank.Sequences[1].Length = 1
	p.Bank.Sequences[1].Values[0] = 12
	e.RefreshSequence(1)
	e.Tick()
	if e.Registers[8] != 12 || e.Voices[0].SeqIndex[0] != 0 {
		t.Fatal("shortened sequence retained an invalid live step")
	}
}

func TestInstrumentBankEditUpdatesALiveSoundWithoutRestartingTheScore(t *testing.T) {
	p := model.New()
	e := New(p)
	e.Play(false)
	e.Trigger(0, 69, 1)
	e.Tick()
	row, ticks, triggers := e.Row, e.Ticks, e.Voices[0].PreviewTriggers
	p.Bank.Instruments[0][38] = 4
	e.RefreshInstrumentParameter(0, 38)
	if e.Row != row || e.Ticks != ticks || e.Voices[0].PreviewTriggers != triggers {
		t.Fatal("bank parameter edit reset the playing arrangement")
	}
	e.Tick()
	if e.Registers[8] != 11 {
		t.Fatal("edited attenuation remained cached in the live voice")
	}
	p.Bank.Sequences[63] = model.Sequence{Values: [63]uint16{9}, Length: 1}
	p.Bank.Instruments[0][48] = 63
	e.RefreshInstrumentParameter(0, 48)
	e.Tick()
	if e.Registers[8] != 5 || p.Bank.SequenceCount != 64 {
		t.Fatal("new sequence link was inaudible or omitted from native serialization")
	}
}
