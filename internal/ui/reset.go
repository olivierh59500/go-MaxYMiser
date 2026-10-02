package ui

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) resetContent(action string) {
	a.remember()
	a.synth.PauseReference()
	a.synth.Edit(func(e *replay.Engine) {
		e.Stop()
		if action == "song-clear" {
			edit.ClearSong(e.Project)
		} else {
			edit.ClearBank(e.Project)
		}
		e.Reset()
	})
	a.editing, a.dirty = false, true
	if action == "song-clear" {
		a.pattern, a.row, a.channel, a.scroll = 0, 0, 0, 0
		a.status = "Song cleared; sound bank and settings retained · Ctrl+Z restores it"
	} else {
		a.instrument, a.sequence, a.sample = 0, 0, 0
		a.status = "Sound bank cleared; notes retained · Ctrl+Z restores sounds and samples"
	}
}
