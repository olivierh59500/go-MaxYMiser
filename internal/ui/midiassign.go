package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) midiAssignmentAction(name string) bool {
	var track int
	if _, err := fmt.Sscanf(name, "midi-sound:%d", &track); err != nil {
		return false
	}
	if track >= 0 && track < 5 {
		e, _ := a.synth.Snapshot()
		a.modal = fmt.Sprintf("MIDI sound %d (00 off, DD drums)", track)
		a.entry = fmt.Sprintf("%02X", e.Project.Song.State[[]int{32, 33, 34, 35, 50}[track]])
	}
	return true
}

func (a *App) midiAssignmentModal(modal, entry string) bool {
	if !strings.HasPrefix(modal, "MIDI sound ") {
		return false
	}
	var track int
	if _, err := fmt.Sscanf(modal, "MIDI sound %d", &track); err != nil || track < 0 || track > 4 {
		return true
	}
	n, err := strconv.ParseUint(entry, 16, 8)
	maximum := uint64(32)
	if track >= 3 {
		maximum = 8
	}
	if err != nil || n > maximum && (track >= 3 || n != 0xdd) {
		a.status = "Use 00 to disable, 01–20 for YM, DD for percussion, or 01–08 for PCM"
		return true
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) { e.Project.Song.State[[]int{32, 33, 34, 35, 50}[track]] = byte(n) })
	a.dirty = true
	a.status = "Native MIDI sound assignment updated"
	return true
}
