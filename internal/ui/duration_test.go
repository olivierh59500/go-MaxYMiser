package ui

import (
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestSongDurationControlSetsExportWithoutChangingTheEditableScore(t *testing.T) {
	p := model.New()
	p.Song.SetSpeed(2)
	app, err := New(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("export-song-duration")
	if app.exportDuration != 2560*time.Millisecond || app.dirty {
		t.Fatal("measuring export duration edited music or ignored tracker timing")
	}
}
