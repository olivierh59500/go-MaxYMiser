package ui

import "github.com/olivierh59500/go-MaxYMiser/internal/replay"

func (a *App) transportStartAction(action string) bool {
	if action != "play-start" && action != "pattern-cursor" {
		return false
	}
	if r, ok := a.synth.Reference(); ok && r.Active {
		a.synth.PauseReference()
	}
	a.synth.Edit(func(e *replay.Engine) {
		e.Stop()
		if action == "play-start" {
			e.SelectPosition(0)
			e.PlayFrom(false, 0)
		} else {
			e.Patterns[a.channel] = byte(a.pattern)
			e.PlayFrom(true, a.row)
		}
	})
	if action == "play-start" {
		a.status = "Song playback restarted at position 00, row 00"
	} else {
		a.status = "Pattern playback started at the selected cursor row"
	}
	return true
}

func (a *App) rightClickAction(action string) {
	switch action {
	case "play":
		a.action("play-start")
	case "pattern":
		a.action("pattern-cursor")
	}
}
