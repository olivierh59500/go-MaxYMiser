package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

// Constructed executable emits a square wave; no original player or tune is
// embedded. Its header exposes two executable song selections.
func foreignSNDHFixture() []byte {
	b := make([]byte, 96)
	for i, target := range []int{96, 98, 100} {
		binary.BigEndian.PutUint16(b[i*4:], 0x6000)
		binary.BigEndian.PutUint16(b[i*4+2:], uint16(target-i*4-2))
	}
	copy(b[12:], "SNDHTITLConstructed foreign music\x00COMMTest\x00##02\x00TC50\x00FRMS")
	copy(b[80:], "HDNS")
	b = append(b, 0x4e, 0x75, 0x4e, 0x75)
	for _, v := range []uint32{0x00001c00, 0x01000100, 0x07003e00, 0x08000f00} {
		b = append(b, 0x23, 0xfc, byte(v>>24), byte(v>>16), byte(v>>8), byte(v), 0, 0xff, 0x88, 0)
	}
	return append(b, 0x4e, 0x75)
}
func awaitSNDHImport(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !a.pollSNDHImport() {
		if time.Now().After(deadline) {
			t.Fatal("bounded SNDH import did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestForeignSNDHProducesReferenceAndEditableNativeCandidate(t *testing.T) {
	a, err := New(model.Demo(), "previous.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	file := filepath.Join(t.TempDir(), "constructed.sndh")
	if err := os.WriteFile(file, foreignSNDHFixture(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenMusic(file); err != nil {
		t.Fatal(err)
	}
	awaitSNDHImport(t, a)
	r, ok := a.synth.Reference()
	e, _ := a.synth.Snapshot()
	if !ok || r.Format != "SNDH" || r.Subtunes != 2 || !a.dirty || a.projectPath != "" || e.Project.Song.Patterns[0][0].Note != 69 || a.ymReport == nil {
		t.Fatalf("foreign import did not yield an editable audible candidate: %s, %+v", a.status, r)
	}
	pcm := make([]byte, 4800*4)
	if _, err := a.synth.Read(pcm); err != nil {
		t.Fatal(err)
	}
	on := false
	for _, v := range pcm {
		on = on || v != 0
	}
	if !on {
		t.Fatal("original executable reference was silent")
	}
	a.synth.SelectReference(false)
	a.pattern, a.row = 0, 0
	a.editCell(func(c *model.Cell) { c.Note = 72 })
	p, err := project.Load(file, "")
	if err == nil || p != nil {
		t.Fatal("constructed source was misidentified as a MaxYMiser bank")
	}
	path := filepath.Join(t.TempDir(), "edited.mys")
	a.save(path)
	again, err := project.Load(path, "")
	if err != nil || again.Song.Patterns[0][0].Note != 72 {
		t.Fatal("edited inferred partition did not save/reload", err)
	}
}

func TestFailedAndCancelledSNDHImportsRetainThePreviousTransport(t *testing.T) {
	a, err := New(model.Demo(), "previous.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.synth.Edit(func(e *replay.Engine) { e.Play(false); e.Row = 17 })
	bad := foreignSNDHFixture()
	bad[96], bad[97] = 0x60, 0xfe
	if err := a.queueSNDH(bad, "endless.sndh", 1); err != nil {
		t.Fatal(err)
	}
	a.cancelSNDHImport()
	before, _ := a.synth.Snapshot()
	if a.pollSNDHImport() || a.sndhImportResults != nil {
		t.Fatal("cancelled import committed a late result")
	}
	after, _ := a.synth.Snapshot()
	if !after.Playing || after.Row != before.Row || after.Project.Title != "First signal" || a.projectPath != "previous.mys" || a.dirty {
		t.Fatal("cancel changed the preceding project or transport")
	}
}

func TestDroppedSNDHSubtunesRetainOwnedExecutableBytes(t *testing.T) {
	a, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	files := fstest.MapFS{"virtual.sndh": {Data: foreignSNDHFixture()}}
	if err := a.OpenDroppedMusic(files); err != nil {
		t.Fatal(err)
	}
	awaitSNDHImport(t, a)
	a.action("sndh:next")
	awaitSNDHImport(t, a)
	r, ok := a.synth.Reference()
	if !ok || r.Subtune != 2 {
		t.Fatalf("dropped song selection tried reopening a virtual path: %+v %s", r, a.status)
	}
}

func TestSourceInspectionCancelsAnOlderExecutableImport(t *testing.T) {
	a := sourceInspectionApp(t)
	score := *a.sourceScore
	if err := a.queueSNDH(foreignSNDHFixture(), "old.sndh", 1); err != nil {
		t.Fatal(err)
	}
	a.inspectDecodedSource(score, "new-inspection.sndh")
	if a.ImportPending() || a.pollSNDHImport() || a.sourcePath != "new-inspection.sndh" {
		t.Fatal("older analysis can replace the newer source inspector")
	}
	e, _ := a.synth.Snapshot()
	if e.Project.Title != "First signal" || a.dirty {
		t.Fatal("inspection replaced the existing composition")
	}
}

func TestANewerFailedOpenCancelsTheOlderPendingImport(t *testing.T) {
	a, err := New(model.Demo(), "current.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.queueSNDH(foreignSNDHFixture(), "older.sndh", 1); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenMusic(filepath.Join(t.TempDir(), "missing.sndh")); err == nil {
		t.Fatal("missing newer file was accepted")
	}
	if a.ImportPending() || a.pollSNDHImport() {
		t.Fatal("older analysis survived a newer failed opening")
	}
	e, _ := a.synth.Snapshot()
	if e.Project.Title != "First signal" || a.projectPath != "current.mys" || a.dirty {
		t.Fatal("failed newer opening replaced existing composition")
	}
}

func TestExecutableSongSelectorRestoresAnEditedCachedSong(t *testing.T) {
	a, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.queueSNDH(foreignSNDHFixture(), "owned.sndh", 1); err != nil {
		t.Fatal(err)
	}
	awaitSNDHImport(t, a)
	a.synth.Edit(func(e *replay.Engine) { e.Project.Title = "Edited first song" })
	a.dirty = true
	a.action("sndh:next")
	awaitSNDHImport(t, a)
	a.action("sndh:select-source")
	a.entry = "1"
	a.applyModal()
	e, _ := a.synth.Snapshot()
	if a.ImportPending() || a.sndhSubtune != 1 || e.Project.Title != "Edited first song" || !a.dirty {
		t.Fatal("explicit song selector rebuilt an already edited song")
	}
}
