package ui

import (
	"os"
	"strconv"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func (a *App) SetYMLibrary(directory string) error {
	library, err := ymimport.IndexLibrary(directory)
	if err != nil {
		return err
	}
	a.ymLibrary = library
	a.ymLibraryDirectory = directory
	a.status = "YM library indexed; alternative titles can be selected when SNDH has no MaxYMiser score"
	return nil
}

func (a *App) suggestYM(path string, raw []byte) {
	a.ymAlternatives = a.ymLibrary.Alternatives(path, raw)
	if len(a.ymAlternatives) > 0 {
		a.errorDetails = append(a.errorDetails, "Possible YM recordings are listed below; choose a version for playback.")
	}
}

func (a *App) alternativeAction(name string) bool {
	if !strings.HasPrefix(name, "ym-alternative:") {
		return false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(name, "ym-alternative:"))
	if err != nil || id < 0 || id >= len(a.ymAlternatives) {
		return true
	}
	path := a.ymAlternatives[id].Path
	if err := a.LoadYM(path); err != nil {
		a.status = err.Error()
	} else {
		a.modal = ""
		a.status = "Loaded selected YM version; Reconstruct proposes an editable score"
	}
	return true
}

func (a *App) libraryModal(modal, entry string) bool {
	if modal != "YM library directory" {
		return false
	}
	if info, err := os.Stat(entry); err != nil || !info.IsDir() {
		a.status = "Select a directory containing YM recordings"
		return true
	}
	if err := a.SetYMLibrary(entry); err != nil {
		a.status = err.Error()
	}
	return true
}
