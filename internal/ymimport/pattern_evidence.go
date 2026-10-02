package ymimport

import (
	"fmt"
	"sort"
)

// PatternPrototype describes a complete source pattern occurrence by its
// normalized register output. Several source IDs can legitimately sound alike.
type PatternPrototype struct {
	Pattern     int     `json:"source_pattern"`
	SourceFrame int     `json:"training_source_frame"`
	Frames      int     `json:"duration_frames"`
	Features    []int16 `json:"features"`
}

type PatternEvidence struct {
	Channel  int     `json:"channel"`
	Start    int     `json:"ym_start_frame"`
	End      int     `json:"ym_end_frame"`
	Patterns []int   `json:"possible_source_patterns"`
	Distance float64 `json:"feature_distance"`
	Known    bool    `json:"known_source_pair,omitempty"`
}

type PatternValidation struct {
	SourcePassages int               `json:"held_out_source_passages"`
	Known          int               `json:"passages_with_known_source_pattern"`
	Recognized     int               `json:"recognized_source_passages"`
	Hits           int               `json:"held_out_candidate_passages"`
	Correct        int               `json:"candidates_including_correct_source_pattern"`
	Unmatched      []PatternEvidence `json:"unmatched_examples,omitempty"`
}

type sourcePassage struct {
	channel, pattern, start, end, sourceFrame int
}

func sourcePassages(score SourceScore, trace Trace, alignment PairAlignment) []sourcePassage {
	var passages []sourcePassage
	for ch := 0; ch < 3; ch++ {
		var current *sourcePassage
		order := -1
		for _, e := range score.Events {
			if e.Channel != ch {
				continue
			}
			if e.Order == order {
				continue
			}
			start := alignedSourceOnset(score, trace, e, alignment)
			if current != nil {
				current.end = start
				if current.start >= 0 && current.end <= len(trace.Frames) && current.end-current.start >= 12 {
					passages = append(passages, *current)
				}
			}
			current = &sourcePassage{channel: ch, pattern: e.Pattern, start: start, sourceFrame: e.Frame}
			order = e.Order
		}
	}
	return passages
}

func patternFeatures(trace Trace, ch, start, end int) []int16 {
	// More samples than an instrument fingerprint retain phrase timing and
	// rests. Silence has zero mixer bits so stale periods cannot invent notes.
	const samples = 64
	features := make([]int16, samples*5)
	base := 0
	peak := 1
	for at := start; at < end; at++ {
		r := trace.Frames[at]
		peak = max(peak, int(r[8+ch]&15))
		if n, ok := tracePitch(trace, ch, at); ok && base == 0 {
			base = n
		}
	}
	for i := 0; i < samples; i++ {
		at := start + i*(end-start)/samples
		r := trace.Frames[at]
		if r[8+ch]&31 == 0 {
			continue
		}
		mix := 0
		if note, ok := tracePitch(trace, ch, at); ok {
			mix |= 1
			features[i*5] = int16(max(-127, min(127, 2*(note-base))))
		}
		if r[7]&(1<<(ch+3)) == 0 {
			mix |= 2
			features[i*5+3] = int16(r[6] & 31)
		}
		if r[8+ch]&16 != 0 {
			mix |= 4
			features[i*5+4] = int16(r[13] & 15)
		}
		features[i*5+1] = int16(int(r[8+ch]&15) * 15 / peak)
		features[i*5+2] = int16(mix)
	}
	return features
}

func informativePattern(features []int16) bool {
	pitches := map[int16]bool{}
	active := 0
	for at := 0; at < len(features); at += 5 {
		if features[at+2]&1 != 0 {
			pitches[features[at]] = true
			active++
		}
	}
	return active >= 16 && len(pitches) >= 3
}

func learnPatternPrototypes(score SourceScore, trace Trace, alignment PairAlignment, trainingEnd int) []PatternPrototype {
	var out []PatternPrototype
	seen := map[string]bool{}
	for _, p := range sourcePassages(score, trace, alignment) {
		if p.end > trainingEnd+alignment.Offset || p.sourceFrame >= trainingEnd || p.end-p.start > 2048 {
			continue
		}
		f := patternFeatures(trace, p.channel, p.start, p.end)
		if !informativePattern(f) {
			continue
		}
		key := fmt.Sprint(p.pattern, ":", p.end-p.start, ":", signature("pattern", f))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, PatternPrototype{p.pattern, p.sourceFrame, p.end - p.start, f})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pattern != out[j].Pattern {
			return out[i].Pattern < out[j].Pattern
		}
		return out[i].SourceFrame < out[j].SourceFrame
	})
	return out
}

// PatternEvidence searches complete phrase windows in the target recording.
// Source order positions are never used as target locations. Ambiguous source
// IDs are reported together; evidence does not rewrite the tracker arrangement.
func (profile PairedProfile) PatternEvidence(trace Trace) []PatternEvidence {
	byLength := map[int][]PatternPrototype{}
	for _, p := range profile.Patterns {
		if p.Frames >= 12 && p.Frames <= 2048 && len(p.Features) == 320 {
			byLength[p.Frames] = append(byLength[p.Frames], p)
		}
	}
	var lengths []int
	for length := range byLength {
		lengths = append(lengths, length)
	}
	sort.Ints(lengths)
	var out []PatternEvidence
	for ch := 0; ch < 3; ch++ {
		starts := map[int]bool{}
		for _, event := range ExtractEvents(trace, ch) {
			for shift := -2; shift <= 2; shift++ {
				if event.Start+shift >= 0 {
					starts[event.Start+shift] = true
				}
			}
		}
		var sortedStarts []int
		for start := range starts {
			sortedStarts = append(sortedStarts, start)
		}
		sort.Ints(sortedStarts)
		for _, start := range sortedStarts {
			for _, length := range lengths {
				end := start + length
				if end > len(trace.Frames) {
					continue
				}
				features := patternFeatures(trace, ch, start, end)
				if !informativePattern(features) {
					continue
				}
				best := 0.75
				distances := map[int]float64{}
				for _, prototype := range byLength[length] {
					d := featureDistance(features, prototype.Features)
					if old, ok := distances[prototype.Pattern]; !ok || d < old {
						distances[prototype.Pattern] = d
					}
					best = min(best, d)
				}
				var ids []int
				for id, d := range distances {
					if d <= best+0.05 && d <= 0.75 {
						ids = append(ids, id)
					}
				}
				if len(ids) == 0 {
					continue
				}
				sort.Ints(ids)
				unique := ids[:0]
				for _, id := range ids {
					if len(unique) == 0 || unique[len(unique)-1] != id {
						unique = append(unique, id)
					}
				}
				out = append(out, PatternEvidence{Channel: ch, Start: start, End: end, Patterns: append([]int(nil), unique...), Distance: best})
			}
		}
	}
	return compactPatternEvidence(out)
}

func compactPatternEvidence(hits []PatternEvidence) []PatternEvidence {
	// Adjacent sampling offsets describe one candidate phrase. Prefer the
	// smaller distance and retain all equally plausible source identities.
	var out []PatternEvidence
	for _, hit := range hits {
		if len(out) == 0 {
			out = append(out, hit)
			continue
		}
		last := &out[len(out)-1]
		if last.Channel == hit.Channel && abs(last.Start-hit.Start) <= 2 && abs(last.End-hit.End) <= 3 {
			if hit.Distance < last.Distance-0.05 {
				*last = hit
				continue
			}
			if abs(int((hit.Distance-last.Distance)*1000)) <= 50 {
				ids := append(append([]int(nil), last.Patterns...), hit.Patterns...)
				sort.Ints(ids)
				last.Patterns = ids[:0]
				for _, id := range ids {
					if len(last.Patterns) == 0 || last.Patterns[len(last.Patterns)-1] != id {
						last.Patterns = append(last.Patterns, id)
					}
				}
			}
			continue
		}
		out = append(out, hit)
	}
	return out
}

func validatePatternEvidence(score SourceScore, trace Trace, profile PairedProfile) PatternValidation {
	var v PatternValidation
	known := map[int]bool{}
	for _, p := range profile.Patterns {
		known[p.Pattern] = true
	}
	selection := trace
	if score.Frames > 0 {
		selection.Frames = trace.Frames[:max(0, min(len(trace.Frames), score.Frames+profile.Alignment.Offset))]
	}
	hits := profile.PatternEvidence(selection)
	passages := sourcePassages(score, trace, profile.Alignment)
	matches := func(hit PatternEvidence, passage sourcePassage) bool {
		if hit.Channel != passage.channel || abs(hit.Start-passage.start) > 2 || abs(hit.End-passage.end) > 3 {
			return false
		}
		for _, id := range hit.Patterns {
			if id == passage.pattern {
				return true
			}
		}
		return false
	}
	for _, p := range passages {
		if p.sourceFrame < profile.TrainingEnd {
			continue
		}
		v.SourcePassages++
		if !known[p.pattern] {
			continue
		}
		v.Known++
		for _, hit := range hits {
			if matches(hit, p) {
				v.Recognized++
				break
			}
		}
	}
	for _, hit := range hits {
		if hit.Start < profile.TrainingEnd+profile.Alignment.Offset || score.Frames > 0 && hit.End > score.Frames+profile.Alignment.Offset {
			continue
		}
		v.Hits++
		found := false
		for _, p := range passages {
			if matches(hit, p) {
				v.Correct++
				found = true
				break
			}
		}
		if !found && len(v.Unmatched) < 8 {
			v.Unmatched = append(v.Unmatched, hit)
		}
	}
	return v
}
