package ui

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) sequenceClipboardAction(action string) bool {
	if action != "sequence-cut" && action != "sequence-copy" && action != "sequence-paste" {
		return false
	}
	if action == "sequence-copy" || action == "sequence-cut" {
		e, _ := a.synth.Snapshot()
		a.sequenceClipboard = e.Project.Bank.Sequences[a.sequence]
		a.hasSequenceClipboard = true
		if action == "sequence-copy" {
			a.status = "Sequence copied with its length and repeat point"
			return true
		}
	}
	if action == "sequence-paste" && !a.hasSequenceClipboard {
		a.status = "Copy a sequence first"
		return true
	}
	sequence := model.Sequence{Length: 1}
	if action == "sequence-paste" {
		sequence = a.sequenceClipboard
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) {
		e.Project.Bank.Sequences[a.sequence] = sequence
		e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, a.sequence+1)
		e.RefreshSequence(a.sequence)
	})
	a.dirty = true
	a.status = "Sequence edited; shared sounding voices retain their current phase"
	return true
}
