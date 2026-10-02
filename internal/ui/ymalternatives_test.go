package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestChoosingYMAlternativeRetainsTheNativeProjectAndOpensReference(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Warhawk.ym")
	data := make([]byte, 4+14*64)
	copy(data, "YM3!")
	for i := 0; i < 64; i++ {
		data[4+i] = 28
		data[4+64+i] = 1
		data[4+7*64+i] = 62
		data[4+8*64+i] = 15
		data[4+13*64+i] = 255
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err = app.SetYMLibrary(root); err != nil {
		t.Fatal(err)
	}
	app.suggestYM("Warhawk.sndh", []byte("SNDH"))
	app.modal = "Unable to open this music"
	app.action("ym-alternative:0")
	e, _ := app.synth.Snapshot()
	ref, ok := app.synth.Reference()
	if !ok || !ref.Active || app.tab != "YM" || app.modal != "" || e.Project.Title != "First signal" {
		t.Fatal("explicit YM alternative did not open independent reference playback")
	}
}
