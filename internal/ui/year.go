package ui

import (
	"fmt"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func validCompositionYear(year string) bool {
	if len(year) != 4 {
		return false
	}
	for _, c := range year {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (a *App) yearAction(action string) bool {
	if action != "song-year" {
		return false
	}
	e, _ := a.synth.Snapshot()
	a.modal, a.entry = "Composition year (YYYY)", e.Project.Year
	return true
}

func (a *App) yearModal(modal, entry string) bool {
	if modal != "Composition year (YYYY)" {
		return false
	}
	year := strings.TrimSpace(entry)
	if !validCompositionYear(year) {
		a.status = "Enter a four-digit composition year"
		return true
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) { e.Project.Year = year })
	copy(a.nativeConfiguration[13:17], year)
	a.dirty = true
	a.status = fmt.Sprintf("Composition year %s will be included in SNDH exports", year)
	return true
}
