package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) arrangementAction(action string) bool {
	switch action {
	case "order:copy":
		e, _ := a.synth.Snapshot()
		a.orderClipboard = [][4]byte{e.Project.Song.Orders[e.Position]}
		a.status = "Song position copied; pattern definitions remain shared"
		return true
	case "order:copy-range":
		e, _ := a.synth.Snapshot()
		a.modal, a.entry = "Copy song positions (first,last hex)", fmt.Sprintf("%02X,%02X", e.Position, e.Position)
		return true
	}
	if action != "order:insert" && action != "order:delete" && action != "order:paste" && action != "order:clone-pattern" && action != "order:add" && action != "order:remove" {
		return false
	}
	e, _ := a.synth.Snapshot()
	position := e.Position
	var err error
	switch action {
	case "order:add":
		length := int(e.Project.Song.Length)
		err = edit.InsertPositions(&e.Project.Song, length, [][4]byte{e.Project.Song.Orders[length-1]})
	case "order:remove":
		last := int(e.Project.Song.Length) - 1
		err = edit.DeletePositions(&e.Project.Song, last, last)
		position = min(position, int(e.Project.Song.Length)-1)
	case "order:insert":
		err = edit.InsertPositions(&e.Project.Song, position, [][4]byte{e.Project.Song.Orders[position]})
	case "order:delete":
		err = edit.DeletePositions(&e.Project.Song, position, position)
		position = min(position, int(e.Project.Song.Length)-1)
	case "order:paste":
		err = edit.InsertPositions(&e.Project.Song, position, a.orderClipboard)
	case "order:clone-pattern":
		_, err = edit.ClonePositionPattern(e.Project, position, a.channel)
	}
	if err != nil {
		a.status = err.Error()
		return true
	}
	a.remember()
	a.synth.Edit(func(engine *replay.Engine) {
		engine.Stop()
		engine.Project = e.Project
		engine.Reset()
		engine.SelectPosition(position)
	})
	a.row = 0
	a.selectChannel(a.channel)
	a.dirty = true
	a.status = "Song arrangement edited; Ctrl+Z restores the previous state"
	return true
}

func (a *App) arrangementModal(modal, entry string) bool {
	if modal != "Copy song positions (first,last hex)" {
		return false
	}
	parts := strings.Split(entry, ",")
	if len(parts) != 2 {
		a.status = "Enter first,last hexadecimal song positions"
		return true
	}
	first, e1 := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 8)
	last, e2 := strconv.ParseUint(strings.TrimSpace(parts[1]), 16, 8)
	if e1 != nil || e2 != nil {
		a.status = "Enter first,last hexadecimal song positions"
		return true
	}
	e, _ := a.synth.Snapshot()
	block, err := edit.CopyPositions(e.Project.Song, int(first), int(last))
	if err != nil {
		a.status = err.Error()
	} else {
		a.orderClipboard = block
		a.status = fmt.Sprintf("Copied %d song positions", len(block))
	}
	return true
}

func (a *App) nextEditablePattern(delta int) {
	e, _ := a.synth.Snapshot()
	selected := max(0, min(model.MaxPatterns-1, a.pattern+delta))
	if selected >= len(e.Project.Song.Patterns) {
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			for len(e.Project.Song.Patterns) <= selected {
				e.Project.Song.Patterns = append(e.Project.Song.Patterns, model.Pattern{})
			}
		})
		a.dirty = true
	}
	a.pattern = selected
}
