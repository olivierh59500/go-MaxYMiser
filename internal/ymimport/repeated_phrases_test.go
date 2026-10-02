package ymimport

import "testing"

func TestRepeatedYMPhrasesFindTransposedTempoVariantsWithoutAProfile(t *testing.T) {
	notes := []int{60, 64, 62, 67, 65, 69, 66, 71}
	a := melodyTrace(notes, 12, 0, 0, false)
	b := melodyTrace([]int{72, 76, 74, 79, 77, 81, 78, 83}, 18, 0, 2, false)
	for range 24 {
		a.Frames = append(a.Frames, [14]byte{7: 63})
	}
	a.Frames = append(a.Frames, b.Frames...)
	hits := RepeatedPhrases(a)
	if len(hits) != 2 || hits[0].Motif == 0 || hits[0].Motif != hits[1].Motif || hits[0].Channel != 0 || hits[1].Channel != 2 || hits[1].Start != 120 || len(hits[0].Patterns) != 0 || hits[0].Known || hits[1].Known {
		t.Fatalf("repeated melodic phrase was lost or given an invented source ID: %+v", hits)
	}
	_, report, err := ReconstructSelection(a, ReconstructionOptions{FramesPerRow: 1})
	if err != nil || len(report.SourcePatterns) != 2 || report.SourceLabelRate != a.Rate {
		t.Fatalf("reconstruction omitted independently detected passages: %v", err)
	}
}

func TestRepeatedYMPhrasesRejectOverlapsAndConstantTones(t *testing.T) {
	one := melodyTrace([]int{60, 64, 62, 67, 65, 69, 66, 71}, 12, 0, 0, false)
	if len(RepeatedPhrases(one)) != 0 {
		t.Fatal("one occurrence was treated as an independent repetition")
	}
	constant, _ := Decode(simpleYM3(512))
	if len(RepeatedPhrases(constant)) != 0 {
		t.Fatal("a held tone was identified as a repeating melody")
	}
}
