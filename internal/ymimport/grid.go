package ymimport

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type GridCandidate struct {
	FramesPerRow int
	Alignment    float64
	Events       int
}

type ReconstructionOptions struct {
	StartFrame, EndFrame int
	FramesPerRow         int
}

// EstimateGrid ranks onset alignment. It is evidence for a proposed row grid,
// not recovery of the original song speed. Pitch modulation can create false
// onsets, so automatic selection requires adequate support and coverage.
func EstimateGrid(trace Trace) []GridCandidate {
	var starts []int
	for channel := 0; channel < 3; channel++ {
		for _, event := range ExtractEvents(trace, channel) {
			starts = append(starts, event.Start)
		}
	}
	var results []GridCandidate
	for step := 2; step <= 16; step++ {
		counts := make([]int, step)
		for _, start := range starts {
			counts[start%step]++
		}
		best := 0
		for _, count := range counts {
			best = max(best, count)
		}
		alignment := 0.0
		if len(starts) > 0 {
			alignment = float64(best) / float64(len(starts))
		}
		results = append(results, GridCandidate{step, alignment, len(starts)})
	}
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Alignment > results[i].Alignment || results[j].Alignment == results[i].Alignment && results[j].FramesPerRow > results[i].FramesPerRow {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results
}

// ReconstructSelection retains the reference timeline and permits explicit
// cropping for long recordings. Coarser grids are user-selected and sampled;
// their approximation and confidence are recorded in the reconstruction report.
func ReconstructSelection(trace Trace, options ReconstructionOptions) (*model.Project, Report, error) {
	start, end := options.StartFrame, options.EndFrame
	if end == 0 {
		end = len(trace.Frames)
	}
	if start < 0 || end <= start || end > len(trace.Frames) {
		return nil, Report{}, fmt.Errorf("ymimport: invalid reconstruction selection")
	}
	step := options.FramesPerRow
	if step == 0 {
		step = 1
		for _, candidate := range EstimateGrid(Trace{Name: trace.Name, Author: trace.Author, Clock: trace.Clock, Rate: trace.Rate, Frames: trace.Frames[start:end]}) {
			if candidate.Events >= 16 && candidate.Alignment >= 0.95 {
				step = candidate.FramesPerRow
				break
			}
		}
	}
	if step < 1 || step > 16 {
		return nil, Report{}, fmt.Errorf("ymimport: row spacing must be 1–16 frames")
	}
	selection := trace
	selection.Frames = nil
	for at := start; at < end; at += step {
		selection.Frames = append(selection.Frames, trace.Frames[at])
	}
	project, report, err := Reconstruct(selection)
	if err != nil {
		return nil, report, err
	}
	project.Song.SetSpeed(step)
	report.Frames = end - start
	report.StartFrame, report.EndFrame, report.FramesPerRow = start, end, step
	report.GridCandidates = EstimateGrid(trace)
	if step > 1 {
		report.Warnings = appendUnique(report.Warnings, fmt.Sprintf("Proposed %d-frame row grid samples register states; modulation within rows may differ from the original YM.", step))
	}
	return project, report, nil
}
