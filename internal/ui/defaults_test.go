package ui

import (
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
)

func TestStartupPreferencesRemainIndependentOfSavedSongSettings(t *testing.T) {
	for _, reload := range []bool{false, true} {
		c := native.Configuration{}
		c[17] = 5
		if reload {
			c[10] = 255
		}
		p := model.New()
		p.Song.State[40] = 2
		if reload {
			c.Apply(&p.Song)
		}
		app, err := New(p, "", true)
		if err != nil {
			t.Fatal(err)
		}
		app.SetDefaultsConfiguration(project.Defaults{Project: p, Configuration: &c, SourcePath: "sounds/DEFAULT.MYS", ProjectPath: "sounds/DEFAULT.MYS"})
		before, _ := app.synth.Snapshot()
		want := byte(2)
		if reload {
			want = 5
		}
		if !app.configurationLoaded || app.nativeConfiguration != c || before.Project.Song.State[40] != want || app.dirty {
			t.Fatal("remembering startup preferences replaced saved song settings")
		}
		other := model.New()
		other.Song.State[40] = 4
		path := filepath.Join(t.TempDir(), "another.mys")
		if err := project.Save(other, path); err != nil {
			t.Fatal(err)
		}
		if err := app.OpenMusic(path); err != nil {
			t.Fatal(err)
		}
		after, _ := app.synth.Snapshot()
		want = 4
		if reload {
			want = 5
		}
		if after.Project.Song.State[40] != want {
			t.Fatal("startup reload preference was lost after opening another song")
		}
		app.Close()
	}
}
