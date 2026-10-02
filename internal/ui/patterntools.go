package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) drawPatternTools(dst *ebiten.Image, e *replay.Engine) {
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, fmt.Sprintf("TRACK / BLOCK EDITING · pattern %02X · %s", a.pattern, map[bool]string{false: "YM", true: "PCM"}[a.channel == 3]), 42, 208, 16, fg)
	a.text(dst, "Selected pattern and cursor come from the Patterns workspace.", 42, 249, 12, dim)
	a.btn(dst, fmt.Sprintf("First %02X", a.blockFirst), 42, 280, 160, 36, "block-first", false)
	a.btn(dst, fmt.Sprintf("Last %02X", a.blockLast), 216, 280, 160, 36, "block-last", false)
	a.btn(dst, "Full track", 390, 280, 174, 36, "block-all", false)
	a.btn(dst, "Copy", 604, 280, 126, 36, "block-copy", false)
	a.btn(dst, "Cut", 744, 280, 126, 36, "block-cut", false)
	a.btn(dst, "Clear", 884, 280, 126, 36, "block-clear", false)
	a.btn(dst, "Paste", 1024, 280, 170, 36, "block-paste", false)
	for i, mode := range []edit.PasteMode{edit.Overwrite, edit.Overlay, edit.Underlay} {
		a.btn(dst, string(mode), 42+i*180, 342, 166, 34, "paste-mode:"+string(mode), a.pasteMode == mode)
	}
	a.text(dst, fmt.Sprintf("Paste at cursor %02X · clipboard %d rows", a.row, len(a.blockClipboard)), 636, 350, 13, accent)
	a.btn(dst, "All columns", 1044, 342, 168, 34, "column-mask-all", false)
	labels := []string{"Note", "Instrument", "Volume", "Effect 1", "Value 1", "Effect 2", "Value 2"}
	if a.channel == 3 {
		labels = []string{"Note A", "Sample A", "Volume A", "Note B", "Sample B", "Volume B", "Reserved"}
	}
	for i, label := range labels {
		a.btn(dst, label, 42+i*169, 398, 154, 34, fmt.Sprintf("column-mask:%d", i), a.columnMask[i])
	}
	a.btn(dst, "Insert row", 42, 458, 170, 36, "row-insert", false)
	a.btn(dst, "Delete row", 226, 458, 170, 36, "row-delete", false)
	a.btn(dst, "Expand ×2", 410, 458, 170, 36, "track-expand", false)
	a.btn(dst, "Shrink ÷2", 594, 458, 170, 36, "track-shrink", false)
	a.btn(dst, "Transpose", 42, 528, 190, 36, "block-transpose", false)
	a.btn(dst, "Attenuation", 248, 528, 190, 36, "block-volume", false)
	a.btn(dst, "Remap sound", 454, 528, 190, 36, "block-remap", false)
	a.btn(dst, "Remap song", 660, 528, 190, 36, "song-remap", false)
	a.btn(dst, "Pack project", 866, 528, 232, 36, "project-pack", false)
	a.btn(dst, "Clear song", 42, 578, 190, 30, "song-clear", false)
	a.btn(dst, "Clear bank", 248, 578, 190, 30, "bank-clear", false)
	a.text(dst, "Clear song keeps sounds; clear bank keeps notes. Ctrl+Z restores either operation.", 454, 587, 12, dim)
	a.text(dst, "Transpose/remap use the selected rows; PCM edits apply to both voices. Expand/shrink retain the clipboard.", 42, 636, 12, dim)
}

func (a *App) patternAction(name string) bool {
	if a.modal != "" {
		return false
	}
	if strings.HasPrefix(name, "paste-mode:") {
		a.pasteMode = edit.PasteMode(strings.TrimPrefix(name, "paste-mode:"))
		return true
	}
	if strings.HasPrefix(name, "column-mask:") {
		id, err := strconv.Atoi(strings.TrimPrefix(name, "column-mask:"))
		if err == nil && id >= 0 && id < 7 {
			a.columnMask[id] = !a.columnMask[id]
		}
		return true
	}
	switch name {
	case "column-mask-all":
		all := true
		for _, on := range a.columnMask {
			all = all && on
		}
		for i := range a.columnMask {
			a.columnMask[i] = !all
		}
	case "project-pack":
		e, _ := a.synth.Snapshot()
		selected, queued := e.Patterns, e.Patterns
		if a.channel >= 0 && a.channel < 4 && a.pattern >= 0 && a.pattern <= 255 {
			selected[a.channel] = byte(a.pattern)
		}
		for channel := 0; channel < 4; channel++ {
			if id, ok := e.QueuedPattern(channel); ok {
				queued[channel] = id
			}
		}
		result, err := edit.PackProjectWithSelections(e.Project, e.Patterns, selected, queued)
		if err != nil {
			a.status = err.Error()
			return true
		}
		a.remember()
		a.synth.Edit(func(engine *replay.Engine) { engine.Stop(); engine.Project = e.Project; engine.Reset() })
		a.pattern, a.row = 0, 0
		a.selectChannel(a.channel)
		a.dirty = true
		a.status = fmt.Sprintf("Packed %d→%d patterns and %d→%d sequences", result.PatternsBefore, result.PatternsAfter, result.SequencesBefore, result.SequencesAfter)
	case "song-clear", "bank-clear":
		a.resetContent(name)
	case "block-first", "block-last":
		a.modal, a.entry = "Block first row", fmt.Sprintf("%02X", a.blockFirst)
		if name == "block-last" {
			a.modal, a.entry = "Block last row", fmt.Sprintf("%02X", a.blockLast)
		}
	case "block-all":
		a.blockFirst, a.blockLast = 0, 63
	case "block-copy", "block-cut":
		e, _ := a.synth.Snapshot()
		if a.pattern >= len(e.Project.Song.Patterns) {
			a.status = "Select an ordinary pattern first"
			return true
		}
		block, err := edit.CopyBlock(e.Project.Song.Patterns[a.pattern], a.blockFirst, a.blockLast)
		if err != nil {
			a.status = err.Error()
			return true
		}
		a.blockClipboard = block
		a.status = fmt.Sprintf("Copied %d rows", len(block))
		if name == "block-cut" {
			a.applyPatternEdit(func(pattern *model.Pattern) error {
				return edit.ClearBlock(pattern, a.blockFirst, a.blockLast, a.columnMask)
			})
		}
	case "block-paste":
		if len(a.blockClipboard) > 0 {
			row := a.row
			if len(a.blockClipboard) == model.Rows {
				row = 0
			}
			a.applyPatternEdit(func(pattern *model.Pattern) error {
				return edit.PasteBlock(pattern, row, a.blockClipboard, a.columnMask, a.pasteMode)
			})
		}
	case "block-clear":
		a.applyPatternEdit(func(pattern *model.Pattern) error {
			return edit.ClearBlock(pattern, a.blockFirst, a.blockLast, a.columnMask)
		})
	case "row-insert":
		a.applyPatternEdit(func(pattern *model.Pattern) error { return edit.InsertRow(pattern, a.row) })
	case "row-delete":
		a.applyPatternEdit(func(pattern *model.Pattern) error { return edit.DeleteRow(pattern, a.row) })
	case "track-expand":
		a.applyPatternEdit(func(pattern *model.Pattern) error { a.blockClipboard = edit.ExpandPattern(pattern); return nil })
	case "track-shrink":
		a.applyPatternEdit(func(pattern *model.Pattern) error { a.blockClipboard = edit.ShrinkPattern(pattern); return nil })
	case "block-transpose":
		a.modal, a.entry = "Transpose block (semitones)", "12"
	case "block-volume":
		a.modal, a.entry = "Adjust block attenuation (steps)", "1"
	case "block-remap":
		a.modal, a.entry = "Remap block (source,destination hex IDs)", "01,02"
	case "song-remap":
		a.modal, a.entry = "Remap song (source,destination hex IDs)", "01,02"
	default:
		return false
	}
	return true
}

func (a *App) applyPatternEdit(operation func(*model.Pattern) error) {
	e, _ := a.synth.Snapshot()
	if a.pattern >= len(e.Project.Song.Patterns) {
		a.status = "Select an ordinary pattern first"
		return
	}
	pattern := e.Project.Song.Patterns[a.pattern]
	if err := operation(&pattern); err != nil {
		a.status = err.Error()
		return
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[a.pattern] = pattern })
	a.dirty, a.status = true, "Pattern edited"
}

func (a *App) patternModal(modal, entry string) bool {
	switch modal {
	case "Block first row", "Block last row":
		row, err := strconv.ParseUint(entry, 16, 6)
		if err != nil {
			a.status = "Enter a hexadecimal row from 00 to 3F"
		} else if modal == "Block first row" {
			a.blockFirst, a.blockLast = int(row), max(int(row), a.blockLast)
		} else {
			a.blockLast, a.blockFirst = int(row), min(int(row), a.blockFirst)
		}
	case "Transpose block (semitones)", "Adjust block attenuation (steps)":
		amount, err := strconv.Atoi(entry)
		if err != nil {
			a.status = "Enter a signed decimal amount"
		} else if strings.HasPrefix(modal, "Transpose") {
			a.applyPatternEdit(func(pattern *model.Pattern) error {
				return edit.TransposeNotes(pattern, a.blockFirst, a.blockLast, amount, a.channel == 3)
			})
		} else {
			a.applyPatternEdit(func(pattern *model.Pattern) error {
				return edit.AdjustVolume(pattern, a.blockFirst, a.blockLast, amount, a.channel == 3)
			})
		}
	case "Remap block (source,destination hex IDs)", "Remap song (source,destination hex IDs)":
		parts := strings.Split(entry, ",")
		if len(parts) != 2 {
			a.status = "Enter source,destination hexadecimal IDs"
			return true
		}
		from, err1 := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 8)
		to, err2 := strconv.ParseUint(strings.TrimSpace(parts[1]), 16, 8)
		if err1 != nil || err2 != nil {
			a.status = "Enter source,destination hexadecimal IDs"
		} else if strings.HasPrefix(modal, "Remap song") {
			e, _ := a.synth.Snapshot()
			if err := edit.RemapSong(e.Project, byte(from), byte(to), a.channel == 3); err != nil {
				a.status = err.Error()
			} else {
				a.remember()
				a.synth.Edit(func(engine *replay.Engine) { engine.Stop(); engine.Project = e.Project; engine.Reset() })
				a.dirty, a.status = true, "Song instrument references remapped"
			}
		} else {
			a.applyPatternEdit(func(pattern *model.Pattern) error {
				return edit.RemapInstrument(pattern, a.blockFirst, a.blockLast, byte(from), byte(to), a.channel == 3)
			})
		}
	default:
		return false
	}
	return true
}
