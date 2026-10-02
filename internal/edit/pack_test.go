package edit

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestPackKeepsRegisterPlaybackAndSequenceCommands(t *testing.T) {
	p := model.Demo()
	p.Bank.SequenceCount = 20
	p.Bank.Sequences[18] = p.Bank.Sequences[3]
	p.Bank.Instruments[1][49] = 18
	p.Song.Patterns[4][1] = model.Cell{Effect1: 'A', Parameter1: 18, Effect2: 'M', Parameter2: 2}
	p.Song.Patterns = append(p.Song.Patterns, p.Song.Patterns[0])
	p.Song.Orders[0][0] = 8
	p.Song.State[4] = 8
	before := p.Clone()
	result, err := PackProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.PatternsAfter >= result.PatternsBefore || result.SequencesAfter >= result.SequencesBefore || p.Song.Orders[0][0] != p.Song.State[4] {
		t.Fatalf("packing did not compact and remap: %+v", result)
	}
	left, right := replay.New(before), replay.New(p)
	left.Play(false)
	right.Play(false)
	for tick := 0; tick < 1600; tick++ {
		left.Tick()
		right.Tick()
		if left.Registers != right.Registers || left.EnvelopeWrite != right.EnvelopeWrite {
			t.Fatalf("packed project changed playback at tick %d", tick)
		}
	}
}

func TestPackDoesNotRewritePCMSamplesAsYMSequences(t *testing.T) {
	p := model.New()
	p.Song.Patterns[2][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 'M', Parameter1: 2}
	p.Song.Orders[0] = [4]byte{0, 1, 255, 2}
	if _, err := PackProject(p); err != nil {
		t.Fatal(err)
	}
	pcm := p.Song.Patterns[p.Song.Orders[0][3]][0]
	if pcm.Effect1 != 'M' || pcm.Parameter1 != 2 {
		t.Fatal("packing interpreted a PCM note as a sequence command")
	}
}

func TestPackRejectsMissingActiveMusicAndPreservesDisabledPCMReferences(t *testing.T) {
	p := model.New()
	p.Song.Orders[0][0] = 17
	before := p.Clone()
	if _, err := PackProject(p); err == nil || p.Song.Orders != before.Song.Orders || p.Bank.Instruments != before.Bank.Instruments {
		t.Fatal("packing accepted or changed a missing active pattern")
	}
	p = model.New()
	p.Song.State[49] = 0
	p.Song.Orders[0][3] = 173
	if _, err := PackProject(p); err != nil {
		t.Fatal(err)
	}
	if p.Song.Orders[0][3] != 173 {
		t.Fatal("packing rewrote an unused native PCM reference")
	}
}
