package ui

import (
	"os"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) LoadConfiguration(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	config, err := native.DecodeConfiguration(raw)
	if err != nil {
		return err
	}
	a.nativeConfiguration = config
	a.synth.Edit(func(e *replay.Engine) { e.Stop(); config.Apply(&e.Project.Song); e.Reset() })
	return nil
}

func (a *App) configurationAction(name string) bool {
	switch name {
	case "config-load":
		a.beginFileBrowser("Load native configuration (.cnf)", "", false)
	case "config-save":
		a.beginFileBrowser("Save native configuration (.cnf)", "MYM.CNF", true)
	default:
		return false
	}
	return true
}

func (a *App) configurationModal(modal, entry string) bool {
	switch modal {
	case "Load native configuration (.cnf)":
		a.remember()
		if err := a.LoadConfiguration(entry); err != nil {
			a.status = err.Error()
		} else {
			a.status = "Native configuration loaded; instrument and channel mappings retained"
		}
	case "Save native configuration (.cnf)":
		e, _ := a.synth.Snapshot()
		config := native.CaptureConfiguration(e.Project.Song, a.nativeConfiguration)
		file, err := os.OpenFile(entry, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			_, err = file.Write(config[:])
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			a.status = err.Error()
		} else {
			a.nativeConfiguration = config
			a.status = "Native MYM.CNF configuration saved"
		}
	default:
		return false
	}
	return true
}
