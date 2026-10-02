package ui

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) moveSongPosition(delta int) {
	queued := false
	position := 0
	a.synth.Edit(func(e *replay.Engine) {
		position = e.Position
		if e.PositionQueued {
			position = e.NextPosition
		}
		position = max(0, min(int(e.Project.Song.Length)-1, position+delta))
		e.SelectPosition(position)
		queued = e.PositionQueued
	})
	if queued {
		a.status = fmt.Sprintf("Jam: position %02X queued for the next pattern boundary", position)
	} else {
		a.selectChannel(a.channel)
	}
}

func (a *App) moveLivePattern(delta int) {
	queued, blocked := false, false
	selected := a.pattern
	a.synth.Edit(func(e *replay.Engine) {
		if e.Playing && e.Jam && !e.PatternMode {
			blocked = true
			return
		}
		if e.Playing && e.PatternMode {
			selected = int(e.Patterns[a.channel])
			if p, ok := e.QueuedPattern(a.channel); ok {
				selected = int(p)
			}
		}
		selected = max(0, min(len(e.Project.Song.Patterns)-1, selected+delta))
		if selected >= model.MaxPatterns {
			return
		}
		if e.Playing && e.Jam && e.PatternMode {
			e.QueuePattern(a.channel, byte(selected))
			queued = true
		} else {
			e.Patterns[a.channel] = byte(selected)
		}
	})
	if blocked {
		a.status = "Jam song playback keeps the current pattern selection; select a song position instead"
		return
	}
	a.pattern = selected
	if queued {
		a.status = fmt.Sprintf("Jam: pattern %02X queued for the next boundary", selected)
	}
}
