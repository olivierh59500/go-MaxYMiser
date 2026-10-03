package ui

import (
	"crypto/sha256"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func TestExecutableSongsRetainTheirEditsSavePathsAndUndo(t *testing.T) {
	a, err := New(model.Demo(), "previous.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.queueSNDH(foreignSNDHFixture(), "collection.sndh", 1); err != nil {
		t.Fatal(err)
	}
	awaitSNDHImport(t, a)
	a.pattern, a.row, a.channel = 0, 0, 0
	a.editCell(func(c *model.Cell) { c.Note = 72 })
	firstPath := filepath.Join(t.TempDir(), "first.mys")
	a.save(firstPath)
	a.row, a.field, a.nibble, a.scroll = 9, 4, 1, 11
	a.instrument, a.sequence, a.sample, a.editing = 3, 7, 2, true
	a.ymOptions = ymimport.ReconstructionOptions{StartFrame: 4, EndFrame: 120, FramesPerRow: 2}
	a.action("sndh:next")
	awaitSNDHImport(t, a)
	if a.sndhSubtune != 2 || a.projectPath != "" || len(a.undo) != 0 || len(a.redo) != 0 {
		t.Fatal("a new executable song inherited the preceding save destination or undo history")
	}
	a.pattern, a.row, a.channel = 0, 0, 0
	a.editCell(func(c *model.Cell) { c.Note = 79 })
	a.action("sndh:previous")
	if a.ImportPending() {
		t.Fatal("a cached editable song was discarded and analyzed again")
	}
	first, _ := a.synth.Snapshot()
	if a.sndhSubtune != 1 || a.projectPath != firstPath || a.dirty || first.Project.Song.Patterns[0][0].Note != 72 || a.row != 9 || a.field != 4 || a.nibble != 1 || a.scroll != 11 || a.instrument != 3 || a.sequence != 7 || a.sample != 2 || !a.editing || a.ymOptions.StartFrame != 4 || a.ymOptions.EndFrame != 120 || a.ymOptions.FramesPerRow != 2 {
		t.Fatal("returning to the first executable song lost its independent workspace")
	}
	ref, ok := a.synth.Reference()
	if !ok || !ref.Seeking && ref.Position != 80 {
		t.Fatal("the restored original did not seek to its editable excerpt's start")
	}
	a.restore(false)
	first, _ = a.synth.Snapshot()
	if first.Project.Song.Patterns[0][0].Note != 69 || !a.dirty {
		t.Fatal("the first executable song did not retain its own undo")
	}
	a.action("sndh:next")
	second, _ := a.synth.Snapshot()
	if a.sndhSubtune != 2 || a.projectPath != "" || !a.dirty || second.Project.Song.Patterns[0][0].Note != 79 {
		t.Fatal("the second executable song inherited the first song's undo or save state")
	}
	a.restore(false)
	second, _ = a.synth.Snapshot()
	if second.Project.Song.Patterns[0][0].Note != 69 {
		t.Fatal("undo in the second executable song restored another song's score")
	}
	a.action("sndh:previous")
	a.restore(true)
	first, _ = a.synth.Snapshot()
	if first.Project.Song.Patterns[0][0].Note != 72 || a.projectPath != firstPath {
		t.Fatal("the first executable song's redo or save destination was lost")
	}
}

func TestExecutableWorkspaceOwnsItsTraceReportAndScoreSnapshots(t *testing.T) {
	a, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.sndhData, a.sndhPath, a.sndhSubtune = foreignSNDHFixture(), "collection.sndh", 1
	a.sndhTrace = &ymimport.Trace{Rate: 50, Frames: [][14]byte{{28}}}
	a.ymReport = &ymimport.Report{
		Warnings:       []string{"original warning"},
		Evidence:       []ymimport.Evidence{{Examples: []string{"original example"}}},
		SourceLabels:   []ymimport.SourceEvidence{{TrainingGroups: []string{"original group"}}},
		SourcePatterns: []ymimport.PatternEvidence{{Patterns: []int{7}}},
	}
	a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[0].PCM = []byte{1, 2} })
	a.remember()
	a.stashSNDHWorkspace()
	a.sndhTrace.Frames[0][0] = 99
	a.ymReport.Warnings[0] = "changed warning"
	a.ymReport.Evidence[0].Examples[0] = "changed example"
	a.ymReport.SourceLabels[0].TrainingGroups[0] = "changed group"
	a.ymReport.SourcePatterns[0].Patterns[0] = 99
	a.undo[0].Song.Patterns[0][0].Note = 99
	a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[0].PCM[0] = 99 })
	if !a.restoreSNDHWorkspace(1) {
		t.Fatal("valid cached workspace could not be restored")
	}
	e, _ := a.synth.Snapshot()
	if a.sndhTrace.Frames[0][0] != 28 || a.ymReport.Warnings[0] != "original warning" || a.ymReport.Evidence[0].Examples[0] != "original example" || a.ymReport.SourceLabels[0].TrainingGroups[0] != "original group" || a.ymReport.SourcePatterns[0].Patterns[0] != 7 || a.undo[0].Song.Patterns[0][0].Note == 99 || e.Project.Bank.Samples[0].PCM[0] != 1 {
		t.Fatal("live mutation changed the cached source analysis or score")
	}
	a.sndhTrace.Frames[0][0] = 88
	a.ymReport.SourcePatterns[0].Patterns[0] = 88
	a.undo[0].Song.Patterns[0][0].Note = 88
	a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[0].PCM[0] = 88 })
	if !a.restoreSNDHWorkspace(1) {
		t.Fatal("cached workspace could not be restored a second time")
	}
	e, _ = a.synth.Snapshot()
	if a.sndhTrace.Frames[0][0] != 28 || a.ymReport.SourcePatterns[0].Patterns[0] != 7 || a.undo[0].Song.Patterns[0][0].Note == 88 || e.Project.Bank.Samples[0].PCM[0] != 1 {
		t.Fatal("restoration exposed cached data to later edits")
	}
}

func TestFailedCachedExecutableSongKeepsTheSelectedScoreAndReference(t *testing.T) {
	a, err := New(model.Demo(), "first.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	raw := foreignSNDHFixture()
	// Initializer returns for song one but loops for song two.
	init := len(raw)
	binary.BigEndian.PutUint16(raw[2:4], uint16(init-2))
	raw = append(raw, 0x0c, 0x40, 0, 2, 0x67, 2, 0x4e, 0x75, 0x60, 0xfe)
	if err := a.synth.LoadSNDH(raw, 1); err != nil {
		t.Fatal(err)
	}
	a.sndhData, a.sndhPath, a.sndhSubtune = raw, "two-players.sndh", 1
	a.stashSNDHWorkspace()
	second := a.sndhWorkspaces[1]
	second.project = model.New()
	second.project.Title = "Second draft"
	a.sndhWorkspaces[2] = second
	before, _ := a.synth.Snapshot()
	refBefore, _ := a.synth.Reference()
	if a.restoreSNDHWorkspace(2) {
		t.Fatal("an endless cached initializer was accepted")
	}
	after, _ := a.synth.Snapshot()
	refAfter, _ := a.synth.Reference()
	if a.sndhSubtune != 1 || a.projectPath != "first.mys" || a.dirty || after.Project.Title != before.Project.Title || after.Project.Bank.Instruments != before.Project.Bank.Instruments || after.Row != before.Row || refAfter.Subtune != refBefore.Subtune || refAfter.Position != refBefore.Position || !refAfter.Active || !refAfter.Playing {
		t.Fatal("failed cached song selection replaced score, save state or original playback")
	}
}

func TestExecutableWorkspaceCannotRestoreAChangedSource(t *testing.T) {
	a, err := New(model.Demo(), "first.mys", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.sndhData, a.sndhSubtune = foreignSNDHFixture(), 1
	a.stashSNDHWorkspace()
	oldHash := sha256.Sum256(a.sndhData)
	a.sndhData = append([]byte(nil), a.sndhData...)
	a.sndhData[20] ^= 1
	if oldHash == sha256.Sum256(a.sndhData) || a.restoreSNDHWorkspace(1) {
		t.Fatal("a new executable reused the preceding source's cached song")
	}
	e, _ := a.synth.Snapshot()
	if e.Project.Title != "First signal" || a.projectPath != "first.mys" || a.dirty {
		t.Fatal("rejecting a cache from another source changed the current composition")
	}
	a.clearSNDH()
	if len(a.sndhWorkspaces) != 0 {
		t.Fatal("clearing the executable retained another source's cached songs")
	}
}
