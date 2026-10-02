package ui

import "github.com/olivierh59500/go-MaxYMiser/internal/replay"

func (a *App) recordPattern() {
	a.editing = true
	if r, ok := a.synth.Reference(); ok && r.Active {
		a.synth.PauseReference()
	}
	a.synth.Edit(func(e *replay.Engine) {
		if e.Playing {
			e.PatternMode = true
		} else {
			e.Patterns[a.channel] = byte(a.pattern)
			e.Play(true)
		}
	})
	a.status = "Recording notes into the current pattern combination"
}

func (a *App) togglePatternRecord() {
	e, _ := a.synth.Snapshot()
	if e.Playing {
		a.editing = !a.editing
		return
	}
	a.recordPattern()
}

func (a *App) patternViewStart(e *replay.Engine) int {
	row := a.row
	if e.Playing && !a.editing && e.Project.Song.State[11] != 0 {
		row = e.Row
	}
	return max(0, min(44, row-10))
}
