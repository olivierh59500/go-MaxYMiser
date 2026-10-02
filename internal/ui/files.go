package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type fileBrowser struct {
	directory string
	entries   []os.DirEntry
	scroll    int
	selected  int
	save      bool
}

func (b *fileBrowser) read(directory, modal string) error {
	path, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	all, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var directories, files []os.DirEntry
	for _, entry := range all {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			directories = append(directories, entry)
		} else if allowedFile(modal, entry.Name()) {
			files = append(files, entry)
		}
	}
	b.directory, b.entries = path, append(directories, files...)
	b.scroll, b.selected = 0, -1
	return nil
}

func allowedFile(modal, name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case strings.HasPrefix(modal, "Open music"):
		return ext == ".mys" || ext == ".myv" || ext == ".snd" || ext == ".sndh" || ext == ".ym"
	case modal == "Load SNDH replay template":
		return ext == ".snd" || ext == ".sndh"
	case strings.Contains(modal, "instrument"):
		return ext == ".myi"
	case strings.Contains(modal, "profile"):
		return ext == ".json"
	case strings.Contains(modal, "configuration"):
		return ext == ".cnf"
	case strings.HasPrefix(modal, "Import raw"):
		return ext == ".pcm" || ext == ".raw" || ext == ".wav" || ext == ".spl" || ext == ".snd"
	default:
		return true
	}
}

func (b *fileBrowser) move(delta int) {
	if len(b.entries) == 0 {
		return
	}
	b.selected = max(0, min(len(b.entries)-1, b.selected+delta))
	if b.selected < b.scroll {
		b.scroll = b.selected
	}
	if b.selected >= b.scroll+10 {
		b.scroll = b.selected - 9
	}
}

func (a *App) beginFileBrowser(modal, filename string, save bool) {
	b := &fileBrowser{save: save, selected: -1}
	if err := b.read(a.directory, modal); err != nil {
		a.status = err.Error()
		return
	}
	a.modal, a.entry, a.browser = modal, filename, b
}

func (a *App) drawFileBrowser(dst *ebiten.Image) {
	b := a.browser
	rect(dst, 0, 0, width, height, color.RGBA{0, 0, 0, 195})
	rect(dst, 170, 140, 940, 536, panel)
	a.text(dst, a.modal, 198, 162, 18, fg)
	path := b.directory
	if len(path) > 88 {
		path = "…" + path[len(path)-85:]
	}
	a.text(dst, path, 198, 202, 13, accent)
	a.btn(dst, "Parent", 960, 190, 122, 32, "file:parent", false)
	for visible := 0; visible < 10; visible++ {
		index := b.scroll + visible
		if index >= len(b.entries) {
			break
		}
		entry := b.entries[index]
		label := entry.Name()
		if entry.IsDir() {
			label = "[dir] " + label
		}
		a.btn(dst, label, 198, 238+visible*28, 884, 26, fmt.Sprintf("file:select:%d", index), index == b.selected)
	}
	rect(dst, 198, 538, 884, 48, bg)
	a.text(dst, a.entry, 210, 552, 13, accent)
	a.text(dst, "Choose a directory or file. Enter confirms; paths can also be typed. Escape cancels.", 198, 605, 11, dim)
	a.btn(dst, "Cancel", 814, 630, 124, 32, "modal:cancel", false)
	a.btn(dst, map[bool]string{true: "Save", false: "Open"}[b.save], 950, 630, 132, 32, "modal:apply", true)
}

func (a *App) fileAction(name string) bool {
	if a.browser == nil || !strings.HasPrefix(name, "file:") {
		return false
	}
	b := a.browser
	if name == "file:parent" {
		if err := b.read(filepath.Dir(b.directory), a.modal); err != nil {
			a.status = err.Error()
		}
		return true
	}
	index, err := strconv.Atoi(strings.TrimPrefix(name, "file:select:"))
	if err != nil || index < 0 || index >= len(b.entries) {
		return true
	}
	entry := b.entries[index]
	if entry.IsDir() {
		if err := b.read(filepath.Join(b.directory, entry.Name()), a.modal); err != nil {
			a.status = err.Error()
		}
	} else {
		b.selected, a.entry = index, entry.Name()
	}
	return true
}

func (a *App) browserConfirm() bool {
	b := a.browser
	entry := strings.TrimSpace(a.entry)
	if entry == "" && b.selected >= 0 {
		entry = b.entries[b.selected].Name()
	}
	if entry == "" {
		a.status = "Choose a file or enter its path"
		return false
	}
	path := entry
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.directory, path)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if err = b.read(path, a.modal); err != nil {
			a.status = err.Error()
		}
		a.entry = ""
		return false
	}
	a.entry, a.directory, a.browser = path, b.directory, nil
	return true
}
