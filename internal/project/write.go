package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func atomicWrite(path string, data []byte) error {
	return atomicWriteWithRename(path, data, os.Rename)
}

// Stage and sync the complete output before replacing a regular target. Existing
// file permissions are retained, consistently with native song/bank pair saves.
func atomicWriteWithRename(path string, data []byte, rename func(string, string) error) error {
	mode := os.FileMode(0644)
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("project: save target %s is not a regular file", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".maxymiser-save-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return rename(name, path)
}
