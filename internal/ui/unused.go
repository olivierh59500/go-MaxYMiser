package ui

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) unusedAction(action string) bool {
	if action != "pattern-unused" && action != "sequence-unused" {
		return false
	}
	e, _ := a.synth.Snapshot()
	if action == "pattern-unused" {
		live := append([]byte(nil), e.Patterns[:]...)
		for channel := 0; channel < 4; channel++ {
			if id, ok := e.QueuedPattern(channel); ok {
				live = append(live, id)
			}
		}
		id, err := edit.UnusedPattern(e.Project, a.pattern, live)
		if err != nil {
			a.status = err.Error()
			return true
		}
		if id >= len(e.Project.Song.Patterns) {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) {
				for len(e.Project.Song.Patterns) <= id {
					e.Project.Song.Patterns = append(e.Project.Song.Patterns, model.Pattern{})
				}
			})
			a.dirty = true
		}
		a.pattern, a.row = id, 0
		a.status = fmt.Sprintf("Unused pattern %02X selected; stored material and live playback retained", id)
		return true
	}
	var live []byte
	for _, voice := range e.Voices {
		for _, offset := range []int{32, 33, 34, 35, 36, 37, 38, 39} {
			live = append(live, voice.Parameters[offset])
		}
	}
	id, err := edit.UnusedSequence(e.Project, a.sequence, live)
	if err != nil {
		a.status = err.Error()
		return true
	}
	if id >= e.Project.Bank.SequenceCount || e.Project.Bank.Sequences[id].Length == 0 {
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, id+1)
			e.Project.Bank.Sequences[id] = model.Sequence{Length: 1}
		})
		a.dirty = true
	}
	a.sequence = id
	a.status = fmt.Sprintf("Unused sequence %02X selected; instrument and effect references retained", id)
	return true
}
