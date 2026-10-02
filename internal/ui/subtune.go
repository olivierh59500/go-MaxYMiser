package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

type subtuneWorkspace struct {
	path                  string
	dirty                 bool
	undo, redo            []*model.Project
	pattern, row, channel int
	initialized           bool
	replaySource          []byte
}

func (a *App) initializeSubtuneWorkspaces() {
	a.subtuneWorkspaces = nil
	var source []byte
	a.synth.Edit(func(e *replay.Engine) { source = e.Project.ReplaySource })
	if len(a.subtunes) > 1 && len(a.collectionSource) == 0 {
		a.collectionSource = append([]byte(nil), source...)
	}
	if len(a.subtunes) < 2 {
		if native.DeclaredSubtunes(source) > 1 {
			a.projectPath = ""
		}
		return
	}
	a.subtuneWorkspaces = make([]subtuneWorkspace, len(a.subtunes))
	for i := range a.subtuneWorkspaces {
		a.subtuneWorkspaces[i].replaySource = source
	}
	// The multi-song executable wrapper is a read-only source for editing.
	// Each selected song gets an independent MYS/MYV save destination.
	a.projectPath = ""
	a.undo, a.redo = nil, nil
}

// SelectSubtune retains edits, cursor, save destination and undo history for
// each song. Native song/bank storage is cloned before it enters the editor.
func (a *App) SelectSubtune(index int) error {
	if index < 0 || index >= len(a.subtunes) {
		return fmt.Errorf("subtune index is outside the loaded container")
	}
	if index == a.subtuneIndex {
		return nil
	}
	if len(a.subtuneWorkspaces) != len(a.subtunes) {
		a.initializeSubtuneWorkspaces()
	}
	current := a.subtuneIndex
	var source []byte
	a.synth.Edit(func(e *replay.Engine) {
		p := e.Project.Clone()
		source = p.ReplaySource
		a.subtunes[current].Song, a.subtunes[current].Bank = p.Song, p.Bank
		a.subtunes[current].Title, a.subtunes[current].Author = p.Title, p.Author
		a.subtunes[current].Year = p.Year
	})
	a.subtuneWorkspaces[current] = subtuneWorkspace{path: a.projectPath, dirty: a.dirty, undo: a.undo, redo: a.redo, pattern: a.pattern, row: a.row, channel: a.channel, initialized: true, replaySource: source}
	value := a.subtunes[index]
	workspace := a.subtuneWorkspaces[index]
	source = workspace.replaySource
	p := (&model.Project{Title: value.Title, Author: value.Author, Year: value.Year, Song: value.Song, Bank: value.Bank, ReplaySource: source}).Clone()
	reloaded := a.reloadConfiguration(&p.Song)
	a.synth.CloseYM()
	a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = p; e.Reset() })
	a.subtuneIndex = index
	a.projectPath, a.dirty, a.undo, a.redo = workspace.path, workspace.dirty || reloaded, workspace.undo, workspace.redo
	a.pattern, a.row, a.channel = workspace.pattern, workspace.row, workspace.channel
	if !workspace.initialized || a.pattern >= len(p.Song.Patterns) || a.channel < 0 || a.channel >= 4 {
		a.pattern, a.row, a.channel = 0, 0, 0
		for ch, id := range p.Song.Orders[0] {
			if int(id) < len(p.Song.Patterns) {
				a.pattern, a.channel = int(id), ch
				break
			}
		}
	}
	a.instrument, a.sequence, a.sample, a.editing = 0, 0, 0, false
	a.status = fmt.Sprintf("Selected native subtune %d of %d; edits and save paths are independent", index+1, len(a.subtunes))
	return nil
}

func (a *App) subtuneAction(action string) bool {
	if action == "subtune-export-all" {
		if len(a.subtunes) < 2 || len(a.collectionSource) == 0 {
			a.status = "Load a complete native multi-song collection first"
			return true
		}
		if _, err := native.ParseMultiSNDHTemplate(a.collectionSource); err != nil {
			a.status = err.Error()
			return true
		}
		a.beginFileBrowser("Export complete SNDH collection", "collection.snd", true)
		return true
	}
	if action != "subtune-select" {
		return false
	}
	a.modal, a.entry = fmt.Sprintf("Select subtune (1–%d)", len(a.subtunes)), strconv.Itoa(a.subtuneIndex+1)
	return true
}

func (a *App) subtuneModal(modal, entry string) bool {
	if modal == "Export complete SNDH collection" {
		var current *model.Project
		a.synth.Edit(func(e *replay.Engine) { current = e.Project.Clone() })
		var projects []*model.Project
		var durations []time.Duration
		for index, value := range a.subtunes {
			p := (&model.Project{Title: value.Title, Author: value.Author, Year: value.Year, Song: value.Song, Bank: value.Bank}).Clone()
			if index == a.subtuneIndex {
				p = current
			}
			projects = append(projects, p)
			durations = append(durations, a.exportDuration)
		}
		if err := project.SaveCollectionSNDH(a.collectionSource, projects, entry, durations, a.icePacking); err != nil {
			a.status = err.Error()
		} else {
			a.status = "Complete SNDH collection exported; native selector and independent edits retained"
		}
		return true
	}
	if !strings.HasPrefix(modal, "Select subtune (") {
		return false
	}
	n, err := strconv.Atoi(entry)
	if err != nil {
		a.status = "Enter the decimal subtune number"
		return true
	}
	if err := a.SelectSubtune(n - 1); err != nil {
		a.status = err.Error()
	}
	return true
}
