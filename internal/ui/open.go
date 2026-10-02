package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

// OpenMusic validates the complete input before replacing the current score or
// stopping its audio. It selects the first editable native track for the view.
func (a *App) OpenMusic(path string) error {
	if strings.EqualFold(filepath.Ext(path), ".ym") {
		return a.LoadYM(path)
	}
	if strings.EqualFold(filepath.Ext(path), ".myv") {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		bank, err := native.DecodeVoiceBank(raw)
		if err != nil {
			return err
		}
		a.acceptVoiceBank(bank, path)
		return nil
	}
	p, err := project.Load(path, "")
	if err != nil {
		if strings.EqualFold(filepath.Ext(path), ".snd") || strings.EqualFold(filepath.Ext(path), ".sndh") {
			if raw, e := os.ReadFile(path); e == nil {
				if e := a.inspectSource(raw, path); e == nil {
					return nil
				}
			}
		}
		return err
	}
	a.acceptProject(p, path, path)
	return nil
}

func (a *App) acceptVoiceBank(bank model.VoiceBank, path string) {
	a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
	a.sourcePath = ""
	a.sourceConversionError = ""
	a.remember()
	a.synth.CloseYM()
	a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project.Bank = bank })
	a.instrument, a.sequence = 0, 0
	a.tab, a.directory, a.dirty = "Instruments", filepath.Dir(path), true
	a.status = "Loaded voice bank: " + filepath.Base(path)
}

func (a *App) acceptProject(p *model.Project, path, savePath string) {
	a.collectionSource = nil
	a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
	a.sourcePath = ""
	a.sourceConversionError = ""
	reloaded := a.reloadConfiguration(&p.Song)
	var subtunes []native.EmbeddedProject
	if strings.EqualFold(filepath.Ext(path), ".snd") || strings.EqualFold(filepath.Ext(path), ".sndh") {
		subtunes, _ = native.DecodeContainers(p.ReplaySource)
	}
	a.remember()
	a.synth.CloseYM()
	a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = p; e.Reset() })
	a.subtunes, a.subtuneIndex = subtunes, 0
	if len(subtunes) > 1 {
		a.collectionSource = append([]byte(nil), p.ReplaySource...)
	}
	a.projectPath, a.directory = savePath, filepath.Dir(path)
	a.instrument, a.sequence, a.sample = 0, 0, 0
	a.pattern, a.row, a.channel = 0, 0, 0
	for channel, id := range p.Song.Orders[0] {
		if int(id) < len(p.Song.Patterns) {
			a.pattern, a.channel = int(id), channel
			break
		}
	}
	a.tab, a.editing, a.dirty = "Patterns", false, reloaded
	a.initializeSubtuneWorkspaces()
	a.status = fmt.Sprintf("Loaded %s · %d patterns · %d native subtune(s)", filepath.Base(path), len(p.Song.Patterns), max(1, len(subtunes)))
	if declared := native.DeclaredSubtunes(p.ReplaySource); declared > len(subtunes) {
		a.status += fmt.Sprintf(" · header declares %d songs; some use another player or an unsupported layout", declared)
	}
	if reloaded {
		a.status += " · personal configuration reapplied"
	}
}

func (a *App) showOpenError(path string, err error) {
	a.ymAlternatives = nil
	a.errorDetails = []string{"File: " + filepath.Base(path)}
	if raw, e := os.ReadFile(path); e == nil && (strings.EqualFold(filepath.Ext(path), ".sndh") || strings.EqualFold(filepath.Ext(path), ".snd")) {
		if plain, e := native.UnpackICE(raw); e == nil && !bytes.Contains(plain, []byte("MYM0INST")) && !bytes.Contains(plain, []byte("MYM1INST")) {
			a.errorDetails = append(a.errorDetails, "This SNDH uses another replay format and contains no MaxYMiser score.", "Open a MaxYMiser MYS/MYV or SNDH to edit its instruments and patterns.")
		} else {
			a.errorDetails = append(a.errorDetails, "This native score could not be decoded: "+err.Error())
		}
	} else {
		a.errorDetails = append(a.errorDetails, err.Error())
	}
	a.errorDetails = append(a.errorDetails, "The current composition and playback were retained.")
	if raw, err := os.ReadFile(path); err == nil {
		a.suggestYM(path, raw)
	}
	a.modal, a.entry = "Unable to open this music", ""
	a.status = "Open failed: " + filepath.Base(path)
}
