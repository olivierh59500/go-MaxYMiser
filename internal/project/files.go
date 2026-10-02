// Package project handles native tracker projects and atomic file writes.
package project

import (
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func Load(songPath, bankPath string) (*model.Project, error) {
	if bankPath == "" && strings.EqualFold(filepath.Ext(songPath), ".mys") {
		bankPath = existingNativeCompanion(strings.TrimSuffix(songPath, filepath.Ext(songPath)), ".myv")
	}
	return loadWithReader(songPath, bankPath, os.ReadFile)
}

// LoadSubtune selects a one-based native SNDH song without assuming physical
// payload order is the only editable song in the executable container.
func LoadSubtune(path string, index int) (*model.Project, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	projects, err := native.DecodeContainers(raw)
	if err != nil {
		return nil, err
	}
	if index < 1 || index > len(projects) {
		return nil, fmt.Errorf("project: subtune %d is outside 1–%d", index, len(projects))
	}
	value := projects[index-1]
	return (&model.Project{Title: value.Title, Author: value.Author, Year: value.Year, Song: value.Song, Bank: value.Bank, ReplaySource: append([]byte(nil), raw...)}).Clone(), nil
}

// LoadFS uses the same native decoding and paired-bank lookup for dropped
// files or embedded filesystems. Both payloads are validated before returning.
func LoadFS(files fs.FS, songPath, bankPath string) (*model.Project, error) {
	if bankPath == "" && strings.EqualFold(path.Ext(songPath), ".mys") {
		directory := path.Dir(songPath)
		name := strings.TrimSuffix(path.Base(songPath), path.Ext(songPath)) + ".myv"
		if entries, err := fs.ReadDir(files, directory); err == nil {
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), name) {
					bankPath = path.Join(directory, entry.Name())
					break
				}
			}
		}
	}
	return loadWithReader(songPath, bankPath, func(path string) ([]byte, error) { return fs.ReadFile(files, path) })
}

func loadWithReader(songPath, bankPath string, read func(string) ([]byte, error)) (*model.Project, error) {
	p := model.New()
	var bankData []byte
	if songPath != "" {
		b, err := read(songPath)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(filepath.Ext(songPath), ".snd") || strings.EqualFold(filepath.Ext(songPath), ".sndh") {
			embedded, err := native.DecodeContainer(b)
			if err != nil {
				return nil, err
			}
			p.Song, p.Bank = embedded.Song, embedded.Bank
			p.Title, p.Author = embedded.Title, embedded.Author
			p.Year = embedded.Year
			p.ReplaySource = append([]byte(nil), b...)
			return p, nil
		}
		song, err := native.DecodeSong(b)
		if err != nil {
			return nil, err
		}
		p.Song = song
		p.Title = strings.TrimSuffix(filepath.Base(songPath), filepath.Ext(songPath))
		if bankPath == "" {
			for _, ext := range []string{".MYV", ".myv"} {
				candidate := strings.TrimSuffix(songPath, filepath.Ext(songPath)) + ext
				if data, err := read(candidate); err == nil {
					bankPath = candidate
					bankData = data
					break
				}
			}
		}
	}
	if bankPath != "" {
		if bankData == nil {
			var err error
			bankData, err = read(bankPath)
			if err != nil {
				return nil, err
			}
		}
		bank, err := native.DecodeVoiceBank(bankData)
		if err != nil {
			return nil, err
		}
		p.Bank = bank
	}
	return p, nil
}
func Save(p *model.Project, path string) error {
	return SavePacked(p, path, false)
}

func SavePacked(p *model.Project, path string, packed bool) error {
	if strings.EqualFold(filepath.Ext(path), ".snd") || strings.EqualFold(filepath.Ext(path), ".sndh") {
		return SaveSNDHPacked(p, path, 0, packed)
	}
	song, err := native.EncodeSong(p.Song)
	if err != nil {
		return err
	}
	bank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		return err
	}
	if _, err := native.DecodeSong(song); err != nil {
		return fmt.Errorf("project: encoded song is not reloadable: %w", err)
	}
	if _, err := native.DecodeVoiceBank(bank); err != nil {
		return fmt.Errorf("project: encoded voice bank is not reloadable: %w", err)
	}
	if packed {
		song, err = native.PackICE(song)
		if err != nil {
			return err
		}
		bank, err = native.PackICE(bank)
		if err != nil {
			return err
		}
	}
	songPath, bankPath := nativePairPaths(path)
	return writeNativePair(songPath, song, bankPath, bank)
}

func SaveSNDH(p *model.Project, path string, duration time.Duration) error {
	return SaveSNDHPacked(p, path, duration, false)
}

func SaveSNDHPacked(p *model.Project, path string, duration time.Duration, packed bool) error {
	if len(p.ReplaySource) == 0 {
		return fmt.Errorf("project: load a native MaxYMiser SNDH replay template before exporting")
	}
	template, err := native.ParseSNDHTemplate(p.ReplaySource)
	if err != nil {
		return err
	}
	data, err := native.EncodeSNDH(template, p, duration)
	if err != nil {
		return err
	}
	if packed {
		data, err = native.PackICE(data)
		if err != nil {
			return err
		}
	}
	return atomicWrite(path, data)
}

// SaveCollectionSNDH exports all slots using a locally supplied complete native
// selector template. It does not substitute a single-song executable prefix.
func SaveCollectionSNDH(source []byte, projects []*model.Project, path string, durations []time.Duration, packed bool) error {
	template, err := native.ParseMultiSNDHTemplate(source)
	if err != nil {
		return err
	}
	data, err := native.EncodeMultiSNDH(template, projects, durations)
	if err != nil {
		return err
	}
	if packed {
		data, err = native.PackICE(data)
		if err != nil {
			return err
		}
	}
	return atomicWrite(path, data)
}
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".maxymiser-save-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func ImportSample(bank *model.VoiceBank, index int, path string) error {
	if index < 0 || index >= 8 {
		return fmt.Errorf("project: invalid sample bank")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b, err = DecodeSample(b)
	if err != nil {
		return err
	}
	if len(b) > 32768 {
		return fmt.Errorf("project: raw sample exceeds 32 KiB")
	}
	bank.Samples[index].PCM = append([]byte(nil), b...)
	bank.Samples[index].Trailer = []byte{0}
	return nil
}
