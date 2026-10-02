package ui

import (
	"fmt"
	"strconv"

	"github.com/olivierh59500/go-MaxYMiser/internal/midi"
)

func (a *App) midiOutputAction(name string) bool {
	if name != "midi-output" {
		return false
	}
	if a.midiOutput != nil {
		a.synth.EnableMIDIOutput(false)
		a.flushMIDIOutput()
		a.midiOutput.Close()
		a.midiOutput = nil
		a.status = "MIDI output disconnected"
		return true
	}
	destinations, err := midi.Destinations()
	if err != nil {
		a.status = err.Error()
	} else if len(destinations) == 0 {
		a.status = "No MIDI output destinations available"
	} else {
		a.midiDestinations = destinations
		a.modal, a.entry = "MIDI output destination (number)", "1"
	}
	return true
}

func (a *App) midiOutputModal(modal, entry string) bool {
	if modal != "MIDI output destination (number)" {
		return false
	}
	index, err := strconv.Atoi(entry)
	if err != nil || index < 1 || index > len(a.midiDestinations) {
		a.status = "Select a listed MIDI destination number"
		return true
	}
	output, err := midi.OpenOutput(a.midiDestinations[index-1].ID)
	if err != nil {
		a.status = err.Error()
	} else {
		a.midiOutput = output
		a.synth.EnableMIDIOutput(true)
		a.status = "MIDI output connected: " + a.midiDestinations[index-1].Name
	}
	return true
}

func (a *App) flushMIDIOutput() {
	if a.midiOutput == nil {
		return
	}
	count, dropped := a.synth.DrainMIDI(a.midiMessages[:])
	for _, message := range a.midiMessages[:count] {
		if err := a.midiOutput.Send(message.Data[:message.Size]); err != nil {
			a.status = err.Error()
			return
		}
	}
	if dropped > 0 {
		a.status = fmt.Sprintf("MIDI output queue dropped %d events", dropped)
	}
}
