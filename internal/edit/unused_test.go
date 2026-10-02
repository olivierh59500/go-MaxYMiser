package edit

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestUnusedPatternProtectsStoredMaterialAndReferencesBeyondSongLength(t *testing.T) {
	p := model.New()
	p.Song.Patterns = append(p.Song.Patterns, model.Pattern{}, model.Pattern{}, model.Pattern{}, model.Pattern{})
	p.Song.Patterns[3][31].Note = 72
	p.Song.Orders[200][0] = 4
	p.Song.State[52] = 5
	before := p.Clone()
	id, err := UnusedPattern(p, 2, []byte{6})
	if err != nil || id != 7 || !reflect.DeepEqual(p, before) {
		t.Fatalf("unused pattern overwrote data or references: id=%d err=%v", id, err)
	}
	id, err = UnusedPattern(p, 239, nil)
	if err != nil || id != 6 {
		t.Fatalf("wrapped search did not reach the first safe blank slot: %d %v", id, err)
	}
}

func TestUnusedSequenceProtectsSharedEffectsLiveOverridesAndSilentTiming(t *testing.T) {
	p := model.New()
	p.Bank.Sequences[3].Values[4] = 7
	p.Bank.Instruments[31][55] = 4
	p.Song.Patterns[0][40] = model.Cell{Effect1: 'V', Parameter1: 5, Effect2: '8', Parameter2: 6}
	p.Bank.Sequences[7].Length = 12
	before := p.Clone()
	id, err := UnusedSequence(p, 2, []byte{8})
	if err != nil || id != 9 || !reflect.DeepEqual(p, before) {
		t.Fatalf("unused sequence ignored a shared definition: id=%d err=%v", id, err)
	}
	id, err = UnusedSequence(p, 255, nil)
	if err != nil || id != 8 {
		t.Fatalf("wrapped search reused zero or a protected definition: %d %v", id, err)
	}
}

func TestFullBanksReportExhaustionWithoutChangingTheProject(t *testing.T) {
	p := model.New()
	p.Song.Patterns = make([]model.Pattern, 240)
	for id := range p.Song.Patterns {
		p.Song.Patterns[id][0].Note = 60
	}
	for id := range p.Bank.Sequences {
		p.Bank.Sequences[id].Values[0] = 1
	}
	before := p.Clone()
	if _, err := UnusedPattern(p, 100, nil); err == nil {
		t.Fatal("occupied pattern bank reported a free slot")
	}
	if _, err := UnusedSequence(p, 100, nil); err == nil {
		t.Fatal("occupied sequence bank reported a free slot")
	}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("capacity search changed musical data")
	}
}
