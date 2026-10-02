package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

// Defaults retains the chosen startup project and configuration independently.
// A bank or individual instrument is a starting point, not a song save target.
type Defaults struct {
	Project                 *model.Project
	Configuration           *native.Configuration
	SourcePath, ProjectPath string
	Messages                []string
}

// LoadDefaults reads only the selected directory. Candidates are validated
// before they replace the initial composition; failed candidates are reported
// and the next native fallback is attempted. Filenames are case insensitive.
func LoadDefaults(directory string) (Defaults, error) {
	result, err := LoadDefaultsFS(os.DirFS(directory))
	if err != nil {
		return result, err
	}
	if result.SourcePath != "" {
		result.SourcePath = filepath.Join(directory, result.SourcePath)
	}
	if result.ProjectPath != "" {
		result.ProjectPath = filepath.Join(directory, result.ProjectPath)
	}
	return result, nil
}

func LoadDefaultsFS(files fs.FS) (Defaults, error) {
	result := Defaults{Project: model.Demo()}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return result, err
	}
	names := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		key := strings.ToUpper(e.Name())
		if key != "MYM.CNF" && key != "DEFAULT.SND" && key != "DEFAULT.SNDH" && key != "DEFAULT.MYS" && key != "DEFAULT.MYV" && key != "DEFAULT.MYI" {
			continue
		}
		if _, found := names[key]; found {
			return result, fmt.Errorf("defaults: ambiguous native filename %s", key)
		}
		names[key] = e.Name()
	}
	if name := names["MYM.CNF"]; name != "" {
		raw, e := fs.ReadFile(files, name)
		if e == nil {
			var c native.Configuration
			c, e = native.DecodeConfiguration(raw)
			if e == nil {
				result.Configuration = &c
			}
		}
		if e != nil {
			result.Messages = append(result.Messages, name+": "+e.Error())
		}
	}
	accept := func(p *model.Project, source, target string, loadedSong bool) Defaults {
		if result.Configuration != nil && (!loadedSong || result.Configuration[10] != 0) {
			result.Configuration.Apply(&p.Song)
		}
		if p.Year == "" && result.Configuration != nil {
			year := string(result.Configuration[13:17])
			valid := true
			for _, digit := range year {
				valid = valid && digit >= '0' && digit <= '9'
			}
			if valid {
				p.Year = year
			}
		}
		result.Project, result.SourcePath, result.ProjectPath = p, source, target
		return result
	}
	for _, key := range []string{"DEFAULT.SND", "DEFAULT.SNDH"} {
		name := names[key]
		if name == "" {
			continue
		}
		p, e := LoadFS(files, name, "")
		if e == nil {
			return accept(p, name, name, true), nil
		}
		result.Messages = append(result.Messages, name+": "+e.Error())
	}
	if song := names["DEFAULT.MYS"]; song != "" {
		bank := names["DEFAULT.MYV"]
		var p *model.Project
		var e error
		if bank == "" {
			e = fmt.Errorf("matching DEFAULT.MYV is missing")
		} else {
			p, e = LoadFS(files, song, bank)
		}
		if e == nil {
			return accept(p, song, song, true), nil
		}
		result.Messages = append(result.Messages, song+": "+e.Error())
	}
	if name := names["DEFAULT.MYV"]; name != "" {
		p, e := LoadFS(files, "", name)
		if e == nil {
			return accept(p, name, "", false), nil
		}
		result.Messages = append(result.Messages, name+": "+e.Error())
	}
	if name := names["DEFAULT.MYI"]; name != "" {
		raw, e := fs.ReadFile(files, name)
		p := model.New()
		if e == nil {
			var instrument native.InstrumentFile
			instrument, e = native.DecodeInstrument(raw)
			if e == nil {
				e = ImportInstrument(p, 0, instrument, native.InstrumentReservations{})
			}
		}
		if e == nil {
			return accept(p, name, "", false), nil
		}
		result.Messages = append(result.Messages, name+": "+e.Error())
	}
	return accept(result.Project, "", "", false), nil
}
