package ui

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) songDurationAction(action string) bool {
	if action != "export-song-duration" {
		return false
	}
	e, _ := a.synth.Snapshot()
	duration, err := replay.MeasureSongDuration(e.Project)
	if err != nil {
		a.status = err.Error()
		return true
	}
	a.exportDuration = duration.Duration
	a.status = fmt.Sprintf("Export duration %.3f seconds; one arrangement traversal to repeat %02X:%02X", duration.Duration.Seconds(), duration.RepeatPosition, duration.RepeatRow)
	return true
}
