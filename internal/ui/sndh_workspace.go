package ui

import (
	"crypto/sha256"
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

// sndhWorkspace retains an editable song independently of the executable's
// other songs. Its source digest prevents a slot from being reused for another
// file even when both containers expose the same song number.
type sndhWorkspace struct {
	source                               [32]byte
	project                              *model.Project
	path, directory                      string
	dirty                                bool
	undo, redo                           []*model.Project
	pattern, row, channel, field, nibble int
	instrument, sequence, sample, scroll int
	editing                              bool
	options                              ymimport.ReconstructionOptions
	trace                                *ymimport.Trace
	report                               *ymimport.Report
}

func (a *App) stashSNDHWorkspace() {
	if len(a.sndhData) == 0 || a.sndhSubtune < 1 {
		return
	}
	var p *model.Project
	a.synth.Edit(func(e *replay.Engine) { p = e.Project.Clone() })
	if a.sndhWorkspaces == nil {
		a.sndhWorkspaces = make(map[int]sndhWorkspace)
	}
	a.sndhWorkspaces[a.sndhSubtune] = sndhWorkspace{
		source: sha256.Sum256(a.sndhData), project: p,
		path: a.projectPath, directory: a.directory, dirty: a.dirty,
		undo: cloneSNDHHistory(a.undo), redo: cloneSNDHHistory(a.redo),
		pattern: a.pattern, row: a.row, channel: a.channel, field: a.field, nibble: a.nibble,
		instrument: a.instrument, sequence: a.sequence, sample: a.sample, scroll: a.scroll,
		editing: a.editing, options: a.ymOptions,
		trace: cloneSNDHTrace(a.sndhTrace), report: cloneSNDHReport(a.ymReport),
	}
}

// restoreSNDHWorkspace validates the original player before changing the
// editor. Each restoration clones cached data so later edits and undo actions
// cannot change the retained workspace or a sibling song.
func (a *App) restoreSNDHWorkspace(song int) bool {
	workspace, ok := a.sndhWorkspaces[song]
	if !ok || workspace.project == nil || len(a.sndhData) == 0 || workspace.source != sha256.Sum256(a.sndhData) {
		return false
	}
	if err := a.synth.LoadSNDH(a.sndhData, song); err != nil {
		a.status = "SNDH song retained the current composition: " + err.Error()
		return false
	}
	a.cancelSNDHImport()
	p := workspace.project.Clone()
	a.synth.Edit(func(e *replay.Engine) { e.Project = p; e.Reset() })
	a.sndhSubtune = song
	a.projectPath, a.directory, a.dirty = workspace.path, workspace.directory, workspace.dirty
	a.undo, a.redo = cloneSNDHHistory(workspace.undo), cloneSNDHHistory(workspace.redo)
	a.pattern, a.row, a.channel, a.field, a.nibble = workspace.pattern, workspace.row, workspace.channel, workspace.field, workspace.nibble
	a.instrument, a.sequence, a.sample, a.scroll = workspace.instrument, workspace.sequence, workspace.sample, workspace.scroll
	a.editing, a.ymOptions = workspace.editing, workspace.options
	a.sndhTrace, a.ymReport = cloneSNDHTrace(workspace.trace), cloneSNDHReport(workspace.report)
	if a.ymOptions.StartFrame > 0 && a.sndhTrace != nil && a.sndhTrace.Rate > 0 {
		a.synth.SeekYM(uint32(a.ymOptions.StartFrame * 1000 / a.sndhTrace.Rate))
	}
	a.ymData, a.ymPath = nil, a.sndhPath
	a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
	a.sourcePath, a.sourceConversionError, a.sourceData = "", "", nil
	a.tab = "YM"
	a.status = fmt.Sprintf("Selected SNDH song %d; edits, save path and undo history retained", song)
	return true
}

func cloneSNDHHistory(history []*model.Project) []*model.Project {
	if history == nil {
		return nil
	}
	copy := make([]*model.Project, len(history))
	for i, p := range history {
		if p != nil {
			copy[i] = p.Clone()
		}
	}
	return copy
}

func cloneSNDHTrace(trace *ymimport.Trace) *ymimport.Trace {
	if trace == nil {
		return nil
	}
	copy := *trace
	copy.Frames = append([][14]byte(nil), trace.Frames...)
	return &copy
}

func cloneSNDHReport(report *ymimport.Report) *ymimport.Report {
	if report == nil {
		return nil
	}
	copy := *report
	copy.Evidence = append([]ymimport.Evidence(nil), report.Evidence...)
	for i := range copy.Evidence {
		copy.Evidence[i].Examples = append([]string(nil), report.Evidence[i].Examples...)
	}
	copy.Warnings = append([]string(nil), report.Warnings...)
	copy.GridCandidates = append([]ymimport.GridCandidate(nil), report.GridCandidates...)
	copy.SourceLabels = append([]ymimport.SourceEvidence(nil), report.SourceLabels...)
	for i := range copy.SourceLabels {
		copy.SourceLabels[i].TrainingGroups = append([]string(nil), report.SourceLabels[i].TrainingGroups...)
	}
	copy.SourceCorpusGroups = append([]string(nil), report.SourceCorpusGroups...)
	copy.RecipeApplications = append([]ymimport.RecipeApplication(nil), report.RecipeApplications...)
	copy.SourcePatterns = append([]ymimport.PatternEvidence(nil), report.SourcePatterns...)
	for i := range copy.SourcePatterns {
		copy.SourcePatterns[i].Patterns = append([]int(nil), report.SourcePatterns[i].Patterns...)
	}
	return &copy
}
