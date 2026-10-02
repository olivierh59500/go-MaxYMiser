package ymimport

import (
	"math"
	"testing"
)

func melodyTrace(notes []int, frames, transpose, channel int, noise bool) Trace {
	trace := Trace{Clock: 2000000, Rate: 50}
	for _, note := range notes {
		period := int(math.Round(float64(trace.Clock) / (16 * 440 * math.Pow(2, float64(note+transpose-69)/12))))
		for i := 0; i < frames; i++ {
			var r [14]byte
			r[7] = 63 &^ (1 << channel)
			r[2*channel], r[2*channel+1] = byte(period), byte(period>>8)
			r[8+channel] = 15
			if noise {
				r[7] &^= 1 << (channel + 3)
				r[6] = 12
			}
			trace.Frames = append(trace.Frames, r)
		}
	}
	return trace
}

func TestCompareFollowsMelodyAcrossTempoTranspositionAndChannels(t *testing.T) {
	notes := []int{60, 64, 62, 67, 65, 69, 66, 71, 68, 73, 70, 74, 71, 67, 72, 68, 75, 70, 73, 69}
	left := melodyTrace(notes, 10, 0, 0, false)
	right := melodyTrace(notes, 20, 7, 2, true)
	comparison := CompareRecordings(left, right, "Original.ym", "Another name.ym")
	if comparison.RhythmMatches < 4 || comparison.LeftCoverage != 1 || comparison.TitleScore != 0 {
		t.Fatalf("melodic evidence was missed or confused with title evidence: %+v", comparison)
	}
	if comparison.MedianTempoRatio != 2 || comparison.MedianTimbreDistance <= 0 {
		t.Fatalf("tempo or timbre differences were lost: %+v", comparison)
	}
	if comparison.LongPhraseMatches == 0 {
		t.Fatal("extended melodic agreement was not detected")
	}
	for _, example := range comparison.Examples {
		if example.Transpose != 7 || example.LeftChannel != 0 || example.RightChannel != 2 || example.RhythmError != 0 {
			t.Fatalf("wrong alignment: %+v", example)
		}
	}
}

func TestMatchingTitlesDoNotConfirmDifferentMusic(t *testing.T) {
	left := melodyTrace([]int{60, 64, 62, 67, 65, 69, 66, 71, 68, 73}, 10, 0, 0, false)
	right := melodyTrace([]int{60, 61, 63, 64, 68, 69, 71, 72, 76, 77}, 10, 0, 0, false)
	comparison := CompareRecordings(left, right, "Warhawk.ym", "Big - Warhawk remix.ym")
	if comparison.TitleScore != 1 || comparison.RhythmMatches != 0 || comparison.SharedPhrases != 0 {
		t.Fatalf("filename match became musical confirmation: %+v", comparison)
	}
}

func TestNoiseOnlyAndGenericPhrasesAreNotMelodicEvidence(t *testing.T) {
	trace := melodyTrace([]int{60, 64, 62, 67, 65, 69, 66, 71}, 10, 0, 0, true)
	for i := range trace.Frames {
		trace.Frames[i][7] |= 1
	}
	if len(melodicPhrases(trace)) != 0 {
		t.Fatal("inactive tone periods were mistaken for a melody")
	}
	scale := melodyTrace([]int{60, 62, 64, 66, 68, 70, 72, 74}, 10, 0, 0, false)
	if len(melodicPhrases(scale)) != 0 {
		t.Fatal("generic interval repetitions should not identify a song")
	}
}
