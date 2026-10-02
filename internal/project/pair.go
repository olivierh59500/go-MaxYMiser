package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type pairFile struct {
	path               string
	data               []byte
	stage, backup      string
	existed, committed bool
}

var pairSaveMutex sync.Mutex

func existingNativeCompanion(stem, extension string) string {
	entries, err := os.ReadDir(filepath.Dir(stem))
	if err != nil {
		return ""
	}
	name := filepath.Base(stem) + extension
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return filepath.Join(filepath.Dir(stem), entry.Name())
		}
	}
	return ""
}

func nativePairPaths(path string) (string, string) {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	song, bank := stem+".mys", stem+".myv"
	if strings.EqualFold(ext, ".mys") {
		song = path
	}
	if strings.EqualFold(ext, ".myv") {
		bank = path
	}
	if existing := existingNativeCompanion(stem, ".myv"); existing != "" {
		bank = existing
	}
	if !strings.EqualFold(ext, ".mys") {
		if existing := existingNativeCompanion(stem, ".mys"); existing != "" {
			song = existing
		}
	}
	return song, bank
}

// writeNativePair stages and syncs both files before touching either target.
// Existing files are retained as rollback copies until both renames succeed.
// This handles write/rename failures; it is not a crash-atomic filesystem group.
func writeNativePair(songPath string, song []byte, bankPath string, bank []byte) error {
	pairSaveMutex.Lock()
	defer pairSaveMutex.Unlock()
	return writeNativePairWithRename([]pairFile{{path: songPath, data: song}, {path: bankPath, data: bank}}, os.Rename)
}

func writeNativePairWithRename(files []pairFile, rename func(string, string) error) error {
	defer func() {
		for _, file := range files {
			if file.stage != "" {
				os.Remove(file.stage)
			}
			if file.backup != "" {
				os.Remove(file.backup)
			}
		}
	}()
	for i := range files {
		file := &files[i]
		info, err := os.Lstat(file.path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("project: save target %s is not a regular file", file.path)
			}
			file.existed = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		stage, err := os.CreateTemp(filepath.Dir(file.path), ".maxymiser-stage-")
		if err != nil {
			return err
		}
		file.stage = stage.Name()
		mode := os.FileMode(0644)
		if file.existed {
			mode = info.Mode().Perm()
		}
		if err := stage.Chmod(mode); err != nil {
			stage.Close()
			return err
		}
		if _, err = stage.Write(file.data); err == nil {
			err = stage.Sync()
		}
		closeErr := stage.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if file.existed {
			backup, err := os.CreateTemp(filepath.Dir(file.path), ".maxymiser-backup-")
			if err != nil {
				return err
			}
			file.backup = backup.Name()
			if err := backup.Close(); err != nil {
				return err
			}
			if err := os.Remove(file.backup); err != nil {
				return err
			}
			if err := os.Link(file.path, file.backup); err != nil {
				return err
			}
		}
	}
	for i := range files {
		file := &files[i]
		if err := rename(file.stage, file.path); err != nil {
			var rollback []error
			for j := i; j >= 0; j-- {
				previous := &files[j]
				if !previous.committed && j != i {
					continue
				}
				if previous.existed {
					if restoreErr := os.Rename(previous.backup, previous.path); restoreErr != nil {
						rollback = append(rollback, fmt.Errorf("restore %s from retained %s: %w", previous.path, previous.backup, restoreErr))
						previous.backup = ""
					}
				} else if removeErr := os.Remove(previous.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					rollback = append(rollback, removeErr)
				}
			}
			if len(rollback) > 0 {
				return errors.Join(append([]error{err, fmt.Errorf("project: rollback did not complete; retained backup files need recovery")}, rollback...)...)
			}
			return err
		}
		file.stage = ""
		file.committed = true
	}
	return nil
}
