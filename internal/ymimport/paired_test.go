package ymimport

import (
	"math"
	"path/filepath"
	"testing"
)

func pairFixture() (SourceScore, Trace) {
	s := SourceScore{Player: "test-player", Rate: 50, Instruments: []SourceInstrument{{ID: 0, Settings: make([]byte, 6)}, {ID: 1, Settings: make([]byte, 6)}}}
	trace := Trace{Rate: 50, Clock: 2000000, Frames: make([][14]byte, 1250)}
	for i := range trace.Frames {
		trace.Frames[i][7] = 63
	}
	for i := 0; i < 60; i++ {
		note, start, inst := 48+(i*7)%25, i*20, i%2
		s.Events = append(s.Events, SourceEvent{Channel: 0, Frame: start, Note: note, Instrument: inst, Retrigger: true})
		period := int(math.Round(125000 / (440 * math.Pow(2, float64(note-69)/12))))
		for j := 0; j < 20; j++ {
			r := &trace.Frames[start+7+j]
			r[0], r[1], r[7] = byte(period), byte(period>>8), 62
			if inst == 0 {
				r[8] = byte(max(1, 15-j))
			} else {
				r[8] = byte(5 + j%5)
			}
		}
	}
	return s, trace
}

func TestPairLearnsNativeLabelsWithChronologicalValidation(t *testing.T) {
	s, trace := pairFixture()
	p, err := LearnPair(s, trace, "test.ym")
	if err != nil {
		t.Fatal(err)
	}
	if p.Alignment.Offset != 7 || p.Alignment.Transpose != 0 || p.Alignment.PitchAgreement != 1 {
		t.Fatalf("wrong register alignment: %+v", p.Alignment)
	}
	if p.Validation.Known < 10 || p.Validation.Correct != p.Validation.Known {
		t.Fatalf("did not classify held-out native instruments: %+v", p.Validation)
	}
	if len(p.SourceEvidence(trace)) < 30 {
		t.Fatal("profile could not annotate independently detected YM events")
	}
	path := filepath.Join(t.TempDir(), "paired.json")
	if err := SavePairedProfile(p, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPairedProfile(path)
	if err != nil || len(loaded.Prototypes) != len(p.Prototypes) {
		t.Fatalf("labelled profile did not round trip: %v", err)
	}
}

func TestPairRejectsUnrelatedRecordingAndAbstainsOnAmbiguity(t *testing.T) {
	s, trace := pairFixture()
	for i := range trace.Frames {
		trace.Frames[i][0], trace.Frames[i][1] = 28, 1
	}
	if _, err := AlignPair(s, trace); err == nil {
		t.Fatal("accepted an unrelated constant-tone recording")
	}
	f := make([]int16, 80)
	p := PairedProfile{Prototypes: []PairedPrototype{{Instrument: 0, Features: f}, {Instrument: 1, Features: f}}}
	if _, _, _, ok := p.MatchSource(f); ok {
		t.Fatal("identical instrument sounds were reported as a certain source label")
	}
	unknown := make([]int16, 80)
	for i := range unknown {
		unknown[i] = 100
	}
	if _, _, _, ok := p.MatchSource(unknown); ok {
		t.Fatal("unrelated sound was assigned a source instrument")
	}
}
