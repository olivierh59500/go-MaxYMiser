package ui

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"os"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
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
	a.configurationLoaded = true
	a.synth.Edit(func(e *replay.Engine) { e.Stop(); config.Apply(&e.Project.Song); e.Reset() })
	return nil
}

func (a *App) configurationAction(name string) bool {
	switch name {
	case "config-load":
		a.beginFileBrowser("Load native configuration (.cnf)", "", false)
	case "config-save":
		a.beginFileBrowser("Save native configuration (.cnf)", "MYM.CNF", true)
	case "config-reload":
		if !a.configurationLoaded {
			e, _ := a.synth.Snapshot()
			a.nativeConfiguration = native.CaptureConfiguration(e.Project.Song, a.nativeConfiguration)
			a.configurationLoaded = true
		}
		if a.nativeConfiguration[10] == 0 {
			a.nativeConfiguration[10] = 255
		} else {
			a.nativeConfiguration[10] = 0
		}
		a.status = "Configuration reload preference changed; Save CNF keeps it for the next launch"
	default:
		return false
	}
	return true
}

func (a *App) reloadConfiguration(song *model.Song) bool {
	if !a.configurationLoaded || a.nativeConfiguration[10] == 0 {
		return false
	}
	a.nativeConfiguration.Apply(song)
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
		err := project.SaveConfiguration(config, entry)
		if err != nil {
			a.status = err.Error()
		} else {
			a.nativeConfiguration = config
			a.configurationLoaded = true
			a.status = "Native MYM.CNF configuration saved"
		}
	default:
		return false
	}
	return true
}
