package ui

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/midi"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) receiveMIDI(message []byte) {
	if command, ok := midi.MachineControl(message); ok && command == 1 || len(message) == 1 && message[0] == 0xfc {
		a.editing = false
	}
	noteEvent := len(message) >= 3 && (message[0]&0xf0 == 0x90 || message[0]&0xf0 == 0x80)
	controlEvent := len(message) >= 3 && message[0]&0xf0 == 0xb0
	if noteEvent && a.editing || controlEvent {
		a.remember()
	}
	changed := false
	a.synth.Edit(func(e *replay.Engine) {
		beforeYM, beforeDMA := e.Voices, e.DMA
		beforeBank := e.Project.Bank.Instruments
		midi.ApplyMapped(e, message)
		changed = changed || e.Project.Bank.Instruments != beforeBank
		if !noteEvent || !a.editing {
			return
		}
		for track := 0; track < 5; track++ {
			note, instrument, volume := byte(0), byte(0), byte(0)
			if track < 3 {
				voice := e.Voices[track]
				if voice.PreviewTriggers == beforeYM[track].PreviewTriggers {
					continue
				}
				note, instrument = voice.Note, voice.Instrument
				volume = byte(max(0, min(15, voice.ColumnVolume)))
			} else {
				voice := e.DMA[track-3]
				if voice.PreviewTriggers == beforeDMA[track-3].PreviewTriggers {
					continue
				}
				note, instrument, volume = voice.Note, voice.Sample, voice.Volume
			}
			if note <= 1 {
				note = model.NoteOff
				instrument = 0
			}
			if volume == 0 {
				volume = 16
			}
			channel := min(track, 3)
			pattern, row := a.pattern, a.row
			if e.Playing {
				pattern, row = int(e.Patterns[channel]), e.Row
			}
			if pattern >= len(e.Project.Song.Patterns) {
				continue
			}
			cell := &e.Project.Song.Patterns[pattern][row]
			if track == 4 {
				cell.Effect1, cell.Parameter1, cell.Effect2 = note, instrument, volume
			} else {
				cell.Note, cell.Instrument, cell.Volume = note, instrument, volume
			}
			changed = true
		}
	})
	if changed {
		a.dirty = true
	}
}
