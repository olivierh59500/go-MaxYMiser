package ymimport

import "sort"

// RepeatedPhrases reuses the cross-recording melody matcher within one YM.
// IDs belong to this analysis only; they are not recovered tracker pattern IDs.
// Overlapping windows on the same voice cannot prove an independent repetition.
func RepeatedPhrases(trace Trace) []PatternEvidence {
	phrases := melodicPhrases(trace)
	var keys []string
	for key := range phrases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []PatternEvidence
	motif := 0
	for _, key := range keys {
		candidates := phrases[key]
		for len(candidates) > 1 {
			first := candidates[0]
			group := []PatternEvidence{{Channel: first.channel, Start: first.start, End: first.notes[7].end}}
			remaining := candidates[:0]
			for _, next := range candidates[1:] {
				match := comparePhrase(first, next, trace.Rate, trace.Rate)
				if match.RhythmError > 0.12 {
					remaining = append(remaining, next)
					continue
				}
				overlaps := false
				for _, hit := range group {
					if hit.Channel == next.channel && hit.Start < next.notes[7].end && next.start < hit.End {
						overlaps = true
						break
					}
				}
				if !overlaps {
					group = append(group, PatternEvidence{Channel: next.channel, Start: next.start, End: next.notes[7].end, Distance: match.RhythmError})
				}
			}
			if len(group) >= 2 {
				motif++
				for i := range group {
					group[i].Motif = motif
				}
				out = append(out, group...)
			}
			candidates = remaining
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Channel < out[j].Channel
	})
	return out
}
