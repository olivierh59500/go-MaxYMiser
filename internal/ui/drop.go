package ui

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

// OpenDroppedMusic validates the selected dropped music and its companion bank
// before replacing the editor. Song files take priority over their MYV bank.
func (a *App) OpenDroppedMusic(files fs.FS) error {
	directory := a.directory
	defer func() { a.directory = directory }()
	var paths []string
	if err := fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".mys", ".myv", ".ym", ".snd", ".sndh":
				paths = append(paths, path)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no supported music files were dropped")
	}
	sort.SliceStable(paths, func(i, j int) bool {
		iBank := strings.EqualFold(filepath.Ext(paths[i]), ".myv")
		jBank := strings.EqualFold(filepath.Ext(paths[j]), ".myv")
		if iBank != jBank {
			return !iBank
		}
		return paths[i] < paths[j]
	})
	path := paths[0]
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ym":
		raw, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		if err := a.loadYMBytes(raw); err != nil {
			return err
		}
		a.ymPath = path
	case ".myv":
		raw, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		bank, err := native.DecodeVoiceBank(raw)
		if err != nil {
			return err
		}
		a.acceptVoiceBank(bank, path)
	default:
		p, err := project.LoadFS(files, path, "")
		if err != nil {
			if strings.EqualFold(filepath.Ext(path), ".snd") || strings.EqualFold(filepath.Ext(path), ".sndh") {
				if raw, e := fs.ReadFile(files, path); e == nil {
					if e := a.inspectSource(raw, path); e == nil {
						return nil
					}
				}
			}
			return err
		}
		a.acceptProject(p, path, "")
	}
	return nil
}
