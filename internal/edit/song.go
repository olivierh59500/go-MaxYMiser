package edit

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// RemapSong visits each arrangement pattern once, keeping YM and PCM roles
// separate. Unused patterns are preserved; ambiguous shared patterns are
// rejected before changing the composition.
func RemapSong(project *model.Project, from, to byte, pcm bool) error {
	p := project.Clone()
	var roles [240]byte
	for _, order := range p.Song.Orders[:p.Song.Length] {
		for channel, id := range order {
			if id >= 240 {
				continue
			}
			if int(id) >= len(p.Song.Patterns) {
				return fmt.Errorf("edit: arrangement refers to missing pattern %02X", id)
			}
			if channel == 3 {
				roles[id] |= 2
			} else {
				roles[id] |= 1
			}
		}
	}
	role := byte(1)
	if pcm {
		role = 2
	}
	for id, used := range roles {
		if used&role == 0 {
			continue
		}
		if used == 3 {
			return fmt.Errorf("edit: pattern %02X shares YM and PCM roles; separate it before song remapping", id)
		}
		if err := RemapInstrument(&p.Song.Patterns[id], 0, 63, from, to, pcm); err != nil {
			return err
		}
	}
	*project = *p
	return nil
}

// CopyInstrument preserves the original native behaviour: the definition is
// duplicated while its shared sequence and sample links remain shared.
func CopyInstrument(bank *model.VoiceBank, from, to int) error {
	if from < 0 || from >= 32 || to < 0 || to >= 32 {
		return fmt.Errorf("edit: instrument indices must be 00–1F")
	}
	bank.Instruments[to] = bank.Instruments[from]
	return nil
}
