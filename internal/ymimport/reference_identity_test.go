package ymimport

import "testing"

func TestKnownPairUsesMusicalContentRatherThanTitlesOrFileNames(t *testing.T) {
	trace, _ := Decode(simpleYM3(128))
	profile := PairedProfile{ReferenceIdentity: registerIdentity(trace), ReferencePatterns: []PatternEvidence{{Channel: 0, Start: 10, End: 64, Patterns: []int{7}}}}
	copy := trace
	copy.Name, copy.Author = "Another filename", "Another author"
	hits, ok := profile.KnownPatternEvidence(copy, 0, 128)
	if !ok || len(hits) != 1 || !hits[0].Known || hits[0].Patterns[0] != 7 {
		t.Fatal("matching register recording did not retain its source pattern identity")
	}
	copy.Frames = append([][14]byte(nil), trace.Frames...)
	copy.Frames[20][0]++
	if _, ok := profile.KnownPatternEvidence(copy, 0, 128); ok {
		t.Fatal("changed recording was treated as the known source pair")
	}
	copy = trace
	copy.Rate = 60
	if _, ok := profile.KnownPatternEvidence(copy, 0, 128); ok {
		t.Fatal("different playback timing kept a source identity")
	}
	copy = trace
	copy.Effects = true
	if _, ok := profile.KnownPatternEvidence(copy, 0, 128); ok {
		t.Fatal("register-only identity claimed equality for timer/sample payloads")
	}
	if hits, _ := profile.KnownPatternEvidence(trace, 65, 128); len(hits) != 0 {
		t.Fatal("known patterns escaped the selected reference range")
	}
}
