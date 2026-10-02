package ymimport

import "testing"

func TestSourcePatternEvidenceFindsTransposedPhraseOnAnotherChannel(t *testing.T) {
	trace := melodyTrace([]int{60, 64, 62, 67, 65, 69, 66, 71}, 12, 0, 0, false)
	features := patternFeatures(trace, 0, 0, len(trace.Frames))
	profile := PairedProfile{Patterns: []PatternPrototype{{Pattern: 4, Frames: len(trace.Frames), Features: features}, {Pattern: 9, Frames: len(trace.Frames), Features: features}}}
	target := melodyTrace([]int{72, 76, 74, 79, 77, 81, 78, 83}, 12, 0, 2, false)
	var leading [17][14]byte
	for i := range leading {
		leading[i][7] = 63
	}
	target.Frames = append(leading[:], target.Frames...)
	hits := profile.PatternEvidence(target)
	found := false
	for _, hit := range hits {
		if hit.Channel == 2 && abs(hit.Start-17) <= 2 && len(hit.Patterns) == 2 && hit.Patterns[0] == 4 && hit.Patterns[1] == 9 {
			found = true
		}
	}
	if !found {
		t.Fatalf("transposed source phrase or its ambiguous identity was lost: %+v", hits)
	}
	constant, _ := Decode(simpleYM3(256))
	if len(profile.PatternEvidence(constant)) != 0 {
		t.Fatal("constant tones were reported as a known source phrase")
	}
}

func TestPatternPrototypeTrainingExcludesCrossingAndHeldOutPassages(t *testing.T) {
	s, trace := pairFixture()
	for i := range s.Events {
		s.Events[i].Order = i / 6
		s.Events[i].Pattern = i / 6
	}
	prototypes := learnPatternPrototypes(s, trace, PairAlignment{Offset: 7}, 500)
	if len(prototypes) == 0 {
		t.Fatal("source order boundaries did not yield pattern examples")
	}
	for _, p := range prototypes {
		if p.SourceFrame >= 500 || p.SourceFrame+p.Frames > 500 {
			t.Fatal("pattern training consumed the held-out timeline")
		}
	}
}

func TestNearbyPatternSamplingOffsetsAreOneAmbiguousPassage(t *testing.T) {
	hits := []PatternEvidence{{Channel: 0, Start: 10, End: 106, Patterns: []int{3}, Distance: 0}, {Channel: 0, Start: 11, End: 108, Patterns: []int{4}, Distance: 0.02}, {Channel: 1, Start: 11, End: 108, Patterns: []int{5}, Distance: 0}}
	compact := compactPatternEvidence(hits)
	if len(compact) != 2 || len(compact[0].Patterns) != 2 || compact[0].Patterns[0] != 3 || compact[0].Patterns[1] != 4 {
		t.Fatalf("sampling offsets inflated evidence or merged channels: %+v", compact)
	}
}
