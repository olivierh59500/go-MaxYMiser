package project

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

// ImportInstrument protects stored sequence commands and PCM sample references,
// including reserved arrangement rows and the editor's saved pattern overrides.
// Native decoding and allocation finish before the bank is changed.
func ImportInstrument(p *model.Project, index int, file native.InstrumentFile, reserved native.InstrumentReservations) error {
	if p == nil {
		return fmt.Errorf("project: no composition for instrument import")
	}
	sequenceCommand := func(code byte) bool {
		switch code {
		case 'L', 'A', 'V', 'M', 'N', 'F', 'I', '8':
			return true
		}
		return false
	}
	for _, pattern := range p.Song.Patterns {
		for _, cell := range pattern {
			if sequenceCommand(cell.Effect1) {
				reserved.Sequences[cell.Parameter1] = true
			}
			if sequenceCommand(cell.Effect2) {
				reserved.Sequences[cell.Parameter2] = true
			}
		}
	}
	var pcmPatterns [model.MaxPatterns]bool
	markPattern := func(id byte) {
		if int(id) < len(p.Song.Patterns) && int(id) < len(pcmPatterns) {
			pcmPatterns[id] = true
		}
	}
	for _, order := range p.Song.Orders {
		markPattern(order[3])
	}
	markPattern(p.Song.State[7])
	markPattern(p.Song.State[55])
	markSample := func(id byte) {
		if id > 0 && int(id) <= len(reserved.Samples) {
			reserved.Samples[id-1] = true
		}
	}
	for id, used := range pcmPatterns {
		if !used {
			continue
		}
		for _, cell := range p.Song.Patterns[id] {
			markSample(cell.Instrument)
			markSample(cell.Parameter1)
		}
	}
	markSample(p.Song.State[35])
	markSample(p.Song.State[50])
	return native.ImportInstrumentReserved(&p.Bank, index, file, reserved)
}

// SaveInstrument exports an independent MYI3 sound with its linked sequences
// and sample. Encoding and optional packing finish before the target changes.
func SaveInstrument(bank *model.VoiceBank, index int, path string, packed bool) error {
	if bank == nil {
		return fmt.Errorf("project: no instrument bank to save")
	}
	file, err := native.ExportInstrument(bank, index)
	if err != nil {
		return err
	}
	raw, err := native.EncodeInstrument(file)
	if err != nil {
		return err
	}
	if packed {
		raw, err = native.PackICE(raw)
		if err != nil {
			return err
		}
	}
	return atomicWrite(path, raw)
}
