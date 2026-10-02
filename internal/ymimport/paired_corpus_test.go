package ymimport

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func corpusPairFixture(group string, variant int, reverse bool) PairedSong {
	s, trace := pairFixture()
	s.Frames = 1200
	for i := range s.Instruments {
		s.Instruments[i].Settings[2] = byte(i + 1)
		s.Instruments[i].Settings[1] = byte(variant + 3)
		s.Instruments[i].Arpeggio = SourceSequence{Offset: 100 + variant, StepFrames: 1, Values: []int{0}, Repeat: 0}
	}
	for i := range s.Events {
		e := &s.Events[i]
		e.Note = 48 + (i*(5+variant*2)+variant*i*i+3*variant)%25
		period := int(math.Round(125000 / (440 * math.Pow(2, float64(e.Note-69)/12))))
		for frame := e.Frame + 7; frame < e.Frame+27; frame++ {
			trace.Frames[frame][0], trace.Frames[frame][1] = byte(period), byte(period>>8)
		}
		if reverse {
			e.Instrument = 1 - e.Instrument
		}
	}
	if reverse {
		s.Instruments[0], s.Instruments[1] = s.Instruments[1], s.Instruments[0]
	}
	return PairedSong{Group: group, Name: group, Score: s, Trace: trace}
}

func TestPairedCorpusUsesDefinitionsInsteadOfBankLocalNumbers(t *testing.T) {
	songs := []PairedSong{corpusPairFixture("alpha", 0, false), corpusPairFixture("beta", 1, true), corpusPairFixture("gamma", 2, false)}
	model, err := LearnPairedCorpus(songs)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Source.Instruments) != 2 || len(model.Corpus.Groups) != 3 || len(model.Source.Events) != 0 || len(model.Patterns) != 0 || model.ReferenceIdentity != "" {
		t.Fatalf("local IDs or source timelines leaked into the corpus model: %+v", model.Corpus)
	}
	if songs[0].Score.Instruments[0].Settings[1] != 3 {
		t.Fatal("canonicalization modified the original bank")
	}
	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := SavePairedProfile(model, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPairedProfile(path)
	if err != nil || loaded.Corpus == nil || len(loaded.Corpus.Sources) != 3 {
		t.Fatalf("corpus profile did not round trip: %v", err)
	}
	for _, recipe := range loaded.Recipes {
		if recipe.TrainingSource == "" {
			t.Fatal("cross-song recipe lost its source identity")
		}
	}
	if len(loaded.SourceEvidence(songs[1].Trace)) == 0 {
		t.Fatal("corpus model did not annotate independently detected YM events")
	}
	wrongClock := songs[1].Trace
	wrongClock.Clock++
	if len(loaded.SourceEvidence(wrongClock)) != 0 {
		t.Fatal("corpus was applied with an unverified chip clock")
	}
}

func TestCrossSongValidationExcludesEntireCompositionGroups(t *testing.T) {
	songs := []PairedSong{corpusPairFixture("alpha", 0, false), corpusPairFixture("beta", 1, true), corpusPairFixture("alpha", 2, false)}
	report, err := ValidatePairedCorpus(songs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Folds) != 2 || report.SourceBounds.Known < 150 || report.SourceBounds.Correct != report.SourceBounds.Known {
		t.Fatalf("wrong cross-song labels: %+v", report)
	}
	for _, fold := range report.Folds {
		for _, group := range fold.Training {
			if group == fold.HeldOutGroup {
				t.Fatal("target composition contributed training examples")
			}
		}
	}
	if report.YMOnsets.Events != report.SourceBounds.Events || report.YMOnsets.Detected == 0 || report.YMOnsets.Matched == 0 {
		t.Fatal("independent onset evaluation omitted source coverage")
	}
}

func TestPairedCorpusRejectsDuplicateMusicInDifferentGroups(t *testing.T) {
	a := corpusPairFixture("alpha", 0, false)
	b := corpusPairFixture("beta", 0, true)
	b.Trace.Clock++ // Source timeline still catches a changed recording wrapper.
	if _, err := ValidatePairedCorpus([]PairedSong{a, b}, nil); err == nil {
		t.Fatal("mixed clocks were accepted")
	}
	b.Trace.Clock = a.Trace.Clock
	b.Trace.Frames = append([][14]byte(nil), b.Trace.Frames...)
	b.Trace.Frames[0][6]++
	if _, err := ValidatePairedCorpus([]PairedSong{a, b}, nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate timeline became independent validation: %v", err)
	}
}

func TestInstrumentIdentityRetainsSynthesisAndTimingDifferences(t *testing.T) {
	a := corpusPairFixture("alpha", 0, false).Score.Instruments[0]
	b := cloneCorpusInstrument(a)
	b.Offset, b.ID, b.Arpeggio.Offset, b.Settings[1] = 8000, 17, 7000, 19
	key := sourceInstrumentIdentity("player", 50, a)
	if key != sourceInstrumentIdentity("player", 50, b) {
		t.Fatal("relocation or local arpeggio ID changed sound identity")
	}
	changes := []func(*SourceInstrument){
		func(i *SourceInstrument) { i.Settings[2]++ },
		func(i *SourceInstrument) { i.Arpeggio.StepFrames++ },
		func(i *SourceInstrument) { i.VolumeSequence = []byte{15, 0} },
		func(i *SourceInstrument) { i.NoiseProgram = []SourceNoiseStep{{Mode: 1, Period: 5}} },
	}
	for _, change := range changes {
		b = cloneCorpusInstrument(a)
		change(&b)
		if key == sourceInstrumentIdentity("player", 50, b) {
			t.Fatal("different synthesis definition shared a label")
		}
	}
	if key == sourceInstrumentIdentity("other-player", 50, a) || key == sourceInstrumentIdentity("player", 100, a) {
		t.Fatal("player semantics or cadence were ignored")
	}
}

func TestCrossSongCountsIncludeUnknownAcceptedDefinitions(t *testing.T) {
	songs := []PairedSong{corpusPairFixture("alpha", 0, false), corpusPairFixture("beta", 1, false)}
	// An altered native setting leaves these synthetic register features alike.
	// The evaluator must count accepted unknown definitions rather than hide them.
	songs[1].Score.Instruments[0].Settings[4]++
	report, err := ValidatePairedCorpus(songs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceBounds.Unknown == 0 || report.SourceBounds.UnknownAccepted == 0 || report.SourceBounds.Known+report.SourceBounds.Unknown != report.SourceBounds.Events {
		t.Fatalf("unknown source definitions were hidden: %+v", report.SourceBounds)
	}
}
