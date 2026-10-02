package ymimport

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestPairedRecipesImprovePitchWithoutChangingLevelOrSharedOccurrences(t *testing.T) {
	trace, _ := Decode(simpleYM3(128))
	// One detuned passage shares the same rounded-pitch frame pattern with the
	// later, equal-tempered passage before recipe insertion.
	for i := 0; i < 64; i++ {
		trace.Frames[i][0] = 29
	}
	trace.Frames[64][8] = 0
	events := ExtractEvents(trace, 0)
	sequence := model.Sequence{Values: [63]uint16{15}, Length: 1}
	r := InstrumentRecipe{SourceInstrument: 0, Frames: 1, Note: 69, Instrument: model.Instrument{17: 4, 18: 4, 32: 1}, Sequences: [5]model.Sequence{sequence, {Values: [63]uint16{0}, Length: 1}, {Values: [63]uint16{65535}, Length: 1}, {Values: [63]uint16{0x100}, Length: 1}, {Length: 1}}}
	profile := PairedProfile{Version: 1, Source: SourceScore{Player: "test-detune", Rate: 50, Instruments: []SourceInstrument{{ID: 0}}}, Prototypes: []PairedPrototype{{Instrument: 0, Features: events[0].Features}}, Recipes: []InstrumentRecipe{r}}
	p, report, err := ReconstructSelection(trace, ReconstructionOptions{FramesPerRow: 1})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := reconstructionFrames(p, 128)
	if err := ApplyPairedRecipes(p, &report, trace, profile); err != nil {
		t.Fatal(err)
	}
	after, _ := reconstructionFrames(p, 128)
	if len(report.RecipeApplications) == 0 {
		t.Fatal("verified pitch correction was not applied")
	}
	for i := 0; i < 128; i++ {
		if before[i][8] != after[i][8] {
			t.Fatal("pitch correction changed the volume envelope")
		}
		if i < 64 && after[i][0] != 29 {
			t.Fatalf("frame %d did not use translated detune: %v", i, after[i])
		}
		if i >= 64 && (after[i][8] != before[i][8] || after[i][7] != before[i][7] || i > 64 && after[i][0] != before[i][0]) {
			t.Fatalf("shared later passage was changed at %d", i)
		}
	}
}

func TestPairedRecipeRejectsVolumeRegressionAndLeavesCoarseGridAlone(t *testing.T) {
	trace, _ := Decode(simpleYM3(64))
	events := ExtractEvents(trace, 0)
	profile := PairedProfile{Source: SourceScore{Player: "test", Rate: 50, Instruments: []SourceInstrument{{ID: 0}}}, Prototypes: []PairedPrototype{{Instrument: 0, Features: events[0].Features}}, Recipes: []InstrumentRecipe{{SourceInstrument: 0, Note: 69, Frames: 1, Instrument: model.Instrument{32: 1}, Sequences: [5]model.Sequence{{Values: [63]uint16{14}, Length: 1}, {Length: 1}, {Length: 1}, {Values: [63]uint16{0x100}, Length: 1}, {Length: 1}}}}}
	for _, step := range []int{1, 2} {
		p, report, err := ReconstructSelection(trace, ReconstructionOptions{FramesPerRow: step})
		if err != nil {
			t.Fatal(err)
		}
		before := p.Clone()
		if err := ApplyPairedRecipes(p, &report, trace, profile); err != nil {
			t.Fatal(err)
		}
		if len(report.RecipeApplications) != 0 || p.Bank.Instruments != before.Bank.Instruments || len(p.Song.Patterns) != len(before.Song.Patterns) {
			t.Fatal("unverified recipe changed the score")
		}
	}
}

func TestLearningRecipesDoesNotUseHeldOutFrames(t *testing.T) {
	s, trace := pairFixture()
	p, err := LearnPair(s, trace, "test.ym")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Recipes) == 0 {
		t.Fatal("labelled training examples did not produce editable instruments")
	}
	for _, r := range p.Recipes {
		if r.SourceFrame >= p.TrainingEnd || r.RegisterError > 0.5 {
			t.Fatal("recipe used held-out or inaccurate training data")
		}
		project := model.New()
		project.Bank.Instruments[0] = r.Instrument
		for i, off := range []int{48, 49, 50, 51, 52} {
			project.Bank.Sequences[i+1] = r.Sequences[i]
			project.Bank.Instruments[0][off] = byte(i + 1)
		}
		e := replay.New(project)
		e.Trigger(0, byte(r.Note), 1)
		e.Tick()
		if e.Registers[8] == 0 {
			t.Fatal("learned native instrument is silent")
		}
	}
}
