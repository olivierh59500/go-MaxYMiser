package ui

// pollExportResult updates the editor after its independent renderer finishes.
// It never blocks the UI frame or changes composition/playback state.
func (a *App) pollExportResult() bool {
	select {
	case result := <-a.exportResults:
		a.exporting = false
		if result != nil {
			a.status = result.Error()
		} else {
			a.status = "WAV export complete"
		}
		return true
	default:
		return false
	}
}
