// Package project handles native tracker projects and atomic file writes.
package project

import (
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Load(songPath, bankPath string) (*model.Project, error) {
	return loadWithReader(songPath, bankPath, os.ReadFile)
}

// LoadFS uses the same native decoding and paired-bank lookup for dropped
// files or embedded filesystems. Both payloads are validated before returning.
func LoadFS(files fs.FS, songPath, bankPath string) (*model.Project, error) {
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
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	song, err := native.EncodeSong(p.Song)
	if err != nil {
		return err
	}
	bank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		return err
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
	if err = atomicWrite(stem+".mys", song); err != nil {
		return err
	}
	return atomicWrite(stem+".myv", bank)
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
