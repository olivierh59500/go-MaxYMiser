package ui

import (
	"path/filepath"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

// SetDefaultsConfiguration remembers startup preferences without reapplying
// them over a song whose saved configuration was intentionally retained.
func (a *App) SetDefaultsConfiguration(defaults project.Defaults) {
	if defaults.Configuration != nil {
		a.nativeConfiguration = *defaults.Configuration
		a.configurationLoaded = true
	}
	if defaults.SourcePath != "" {
		a.directory = filepath.Dir(defaults.SourcePath)
		a.status = "Loaded startup " + filepath.Base(defaults.SourcePath)
	}
	if len(defaults.Messages) > 0 {
		a.status += " · " + strings.Join(defaults.Messages, " · ")
	}
}
