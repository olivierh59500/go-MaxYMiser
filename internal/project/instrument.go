package project

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

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
