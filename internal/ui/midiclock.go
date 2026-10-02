package ui

import (
	"fmt"
	"strconv"

	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) midiClockAction(action string) bool {
	if action != "midi-latency" {
		return false
	}
	e, _ := a.synth.Snapshot()
	a.modal, a.entry = "MIDI latency (0–255 clock pulses)", fmt.Sprint(e.Project.Song.State[57])
	return true
}

func (a *App) midiClockModal(modal, entry string) bool {
	if modal != "MIDI latency (0–255 clock pulses)" {
		return false
	}
	value, err := strconv.Atoi(entry)
	if err != nil || value < 0 || value > 255 {
		a.status = "Enter a MIDI compensation from 0 to 255 clock pulses"
		return true
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) { e.Project.Song.State[57] = byte(value) })
	a.dirty = true
	a.status = "MIDI latency applies on Start and Continue in units of external clock pulses"
	return true
}
