package ui

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

type sndhImportResult struct {
	raw       []byte
	path      string
	subtune   int
	trace     ymimport.Trace
	candidate *model.Project
	report    ymimport.Report
	err       error
	options   ymimport.ReconstructionOptions
}

// queueSNDH executes and analyzes outside the graphical and audio threads. A
// cancelled generation cannot replace the newer song or reference selected by
// the editor. The raw player remains independent of native export templates.
func (a *App) queueSNDH(raw []byte, path string, subtune int) error {
	return a.queueSNDHSelection(raw, path, subtune, ymimport.ReconstructionOptions{EndFrame: 400, FramesPerRow: 1})
}

func (a *App) queueSNDHSelection(raw []byte, path string, subtune int, options ymimport.ReconstructionOptions) error {
	a.cancelSNDHImport()
	if options.EndFrame == 0 {
		options.EndFrame = max(400, options.StartFrame+400)
	}
	if options.StartFrame < 0 || options.EndFrame <= options.StartFrame || options.EndFrame > 16320 || options.FramesPerRow < 0 || options.FramesPerRow > 16 {
		return fmt.Errorf("sndh: choose 0–16320 frames and a row grid of 0–16")
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		return err
	}
	if subtune == 0 {
		subtune = file.Metadata.DefaultSubtune
	}
	if subtune < 1 || subtune > file.Metadata.Subtunes {
		return fmt.Errorf("sndh: invalid subtune")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.sndhCancel = cancel
	results := make(chan sndhImportResult, 1)
	a.sndhImportResults = results
	owned := append([]byte(nil), raw...)
	a.status = fmt.Sprintf("Analyzing SNDH song %d/%d; current composition retained", subtune, file.Metadata.Subtunes)
	go func() {
		result := sndhImportResult{raw: owned, path: path, subtune: subtune, options: options}
		result.trace, result.candidate, result.report, result.err = ymimport.ImportSNDHSelection(ctx, owned, subtune, options)

		if ctx.Err() == nil {
			results <- result
		}
	}()
	return nil
}

func (a *App) cancelSNDHImport() {
	if a.sndhCancel != nil {
		a.sndhCancel()
		a.sndhCancel = nil
	}
	a.sndhImportResults = nil
}

func (a *App) pollSNDHImport() bool {
	if a.sndhImportResults == nil {
		return false
	}
	select {
	case result := <-a.sndhImportResults:
		a.sndhImportResults = nil
		a.sndhCancel = nil
		if result.err != nil {
			a.status = "SNDH import retained the current composition: " + result.err.Error()
			return true
		}
		if err := a.synth.LoadSNDH(result.raw, result.subtune); err != nil {
			a.status = "SNDH import retained the current composition: " + err.Error()
			return true
		}
		sameSource := a.sndhPath == result.path && bytes.Equal(a.sndhData, result.raw)
		if !sameSource {
			a.sndhWorkspaces = nil
		} else if a.sndhSubtune != result.subtune {
			a.stashSNDHWorkspace()
		}
		a.remember()
		if sameSource && a.sndhSubtune != result.subtune {
			a.undo, a.redo = nil, nil
		}
		a.subtunes, a.subtuneWorkspaces, a.collectionSource = nil, nil, nil
		a.subtuneIndex = 0
		a.sndhData, a.sndhPath, a.sndhSubtune = result.raw, result.path, result.subtune
		a.sndhTrace = &result.trace
		a.ymData = nil
		a.ymPath = result.path
		a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
		a.sourcePath = ""
		a.sourceData = nil
		a.sourceConversionError = ""
		a.tab = "YM"
		a.ymReport = &result.report
		a.ymOptions = result.options
		if result.report.EndFrame > result.report.StartFrame {
			a.ymOptions.StartFrame, a.ymOptions.EndFrame = result.report.StartFrame, result.report.EndFrame
		}
		if result.report.FramesPerRow > 0 {
			a.ymOptions.FramesPerRow = result.report.FramesPerRow
		}
		if result.options.StartFrame > 0 {
			a.synth.SeekYM(uint32(result.options.StartFrame * 1000 / result.trace.Rate))
		}
		if result.candidate != nil {
			a.synth.Edit(func(e *replay.Engine) { e.Project = result.candidate; e.Reset() })
			a.projectPath = ""
			a.dirty = true
			a.pattern, a.row, a.channel = 0, 0, 0
			for ch, id := range result.candidate.Song.Orders[0] {
				if int(id) < len(result.candidate.Song.Patterns) {
					a.pattern, a.channel = int(id), ch
					break
				}
			}
			if result.report.SourcePlayer == "sndh-sampled-excerpt" {
				a.channel = 3
				a.pattern = int(result.candidate.Song.Orders[0][3])
			}
			a.status = fmt.Sprintf("%s: %d patterns; original SNDH available for comparison", sndhImportLabel(result.report.SourcePlayer), result.report.Patterns)
		} else {
			a.status = "SNDH reference loaded; editable reconstruction unavailable"
		}
		return true
	default:
		return false
	}
}

func (a *App) sndhAction(name string) bool {
	if name == "sndh:select-source" {
		raw, path := a.sourceData, a.sourcePath
		if len(raw) == 0 {
			e, _ := a.synth.Snapshot()
			raw, path = e.Project.ReplaySource, "Native SNDH source"
		}
		if len(raw) == 0 {
			raw, path = a.sndhData, a.sndhPath
		}
		file, err := sndh.Parse(raw)
		if err != nil {
			a.status = err.Error()
			return true
		}
		a.sndhSelectionData, a.sndhSelectionPath = append([]byte(nil), raw...), path
		a.modal, a.entry = fmt.Sprintf("SNDH executable song (1–%d)", file.Metadata.Subtunes), strconv.Itoa(file.Metadata.DefaultSubtune)
		return true
	}
	if name != "sndh:previous" && name != "sndh:next" {
		return false
	}
	if len(a.sndhData) == 0 {
		return true
	}
	ref, ok := a.synth.Reference()
	if !ok {
		return true
	}
	song := a.sndhSubtune
	if name == "sndh:previous" {
		song--
	} else {
		song++
	}
	if song < 1 || song > ref.Subtunes {
		return true
	}
	a.stashSNDHWorkspace()
	if a.restoreSNDHWorkspace(song) {
		return true
	}
	if err := a.queueSNDH(a.sndhData, a.sndhPath, song); err != nil {
		a.status = err.Error()
	}
	return true
}

func (a *App) clearSNDH() {
	a.cancelSNDHImport()
	a.sndhWorkspaces = nil
	a.sndhData = nil
	a.sndhTrace = nil
	a.sndhPath = ""
	a.sndhSubtune = 0
}

// ImportPending reports whether the editor is analyzing an executable source.
func (a *App) ImportPending() bool { return a.sndhImportResults != nil }

func (a *App) sndhModal(modal, entry string) bool {
	if !strings.HasPrefix(modal, "SNDH executable song (") {
		return false
	}
	song, err := strconv.Atoi(strings.TrimSpace(entry))
	if err == nil {
		var file *sndh.File
		file, err = sndh.Parse(a.sndhSelectionData)
		if err == nil && (song < 1 || song > file.Metadata.Subtunes) {
			err = fmt.Errorf("subtune outside executable range")
		}
	}
	if err == nil {
		if a.sndhSelectionPath == a.sndhPath && bytes.Equal(a.sndhSelectionData, a.sndhData) {
			a.stashSNDHWorkspace()
			if a.restoreSNDHWorkspace(song) {
				a.sndhSelectionData, a.sndhSelectionPath = nil, ""
				return true
			}
		}
		err = a.queueSNDH(a.sndhSelectionData, a.sndhSelectionPath, song)
	}
	if err != nil {
		a.status = "Source song retained: " + err.Error()
	}
	a.sndhSelectionData, a.sndhSelectionPath = nil, ""
	return true
}

func sndhImportLabel(player string) string {
	if player == "sndh-sampled-excerpt" {
		return "Sampled audio excerpt"
	}
	return "Inferred register score"
}
