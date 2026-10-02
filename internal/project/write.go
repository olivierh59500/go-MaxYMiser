package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func atomicWrite(path string, data []byte) error {
	return atomicWriteWithRename(path, data, os.Rename)
}

// Stage and sync the complete output before replacing a regular target. Existing
// file permissions are retained, consistently with native song/bank pair saves.
func atomicWriteWithRename(path string, data []byte, rename func(string, string) error) error {
	return writeFileWithRename(path, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}, rename)
}

// WriteFileAtomically streams an output to a staged file and commits it only
// after the complete writer succeeds. It preserves regular target permissions.
func WriteFileAtomically(path string, write func(io.Writer) error) error {
	return writeFileWithRename(path, write, os.Rename)
}

func writeFileWithRename(path string, write func(io.Writer) error, rename func(string, string) error) error {
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
	defer f.Close()
	if err = f.Chmod(mode); err == nil {
		err = write(f)
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
