package edit

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestPackPreservesPCMReferencesInStoredPositionsAndEditorState(t *testing.T) {
	for _, reference := range []string{"inactive position", "disabled PCM", "current editor", "saved editor"} {
		t.Run(reference, func(t *testing.T) {
			p := model.New()
			p.Song.Patterns = append(p.Song.Patterns, model.Pattern{})
			p.Song.Patterns[3][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 'M', Parameter1: 7}
			p.Bank.Sequences[7] = p.Bank.Sequences[2]
			for _, base := range []int{4, 52} {
				p.Song.State[base+3] = model.EmptyPattern
			}
			switch reference {
			case "inactive position":
				p.Song.Orders[180][3] = 3
			case "disabled PCM":
				p.Song.State[49] = 0
				p.Song.Orders[0][3] = 3
			case "current editor":
				p.Song.State[7] = 3
			case "saved editor":
				p.Song.State[55] = 3
			}
			if _, err := PackProject(p); err != nil {
				t.Fatal(err)
			}
			var id byte
			switch reference {
			case "inactive position":
				id = p.Song.Orders[180][3]
			case "disabled PCM":
				id = p.Song.Orders[0][3]
			case "current editor":
				id = p.Song.State[7]
			case "saved editor":
				id = p.Song.State[55]
			}
			if p.Song.Patterns[id][0].Parameter1 != 7 {
				t.Fatal("packing rewrote the second PCM sample as a YM sequence")
			}
			encoded, err := native.EncodeSong(p.Song)
			if err != nil {
				t.Fatal(err)
			}
			song, err := native.DecodeSong(encoded)
			if err != nil || song.Patterns[id][0].Parameter1 != 7 {
				t.Fatalf("saved PCM reference changed: %v", err)
			}
			// Activate the previously stored pattern after packing. Its second
			// note must still select sample 7, independent of sequence remapping.
			p.Song.State[49] = 2
			p.Song.Orders[0][3] = id
			e := replay.New(p)
			e.Play(false)
			e.Tick()
			if e.DMA[1].Sample != 7 {
				t.Fatalf("prepared PCM note selected sample %d after packing", e.DMA[1].Sample)
			}
		})
	}
}

func TestPackRejectsAmbiguousSavedRoleWithoutChangingTheProject(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Effect1: 'M', Parameter1: 7}
	p.Bank.Sequences[7] = p.Bank.Sequences[2]
	p.Song.Orders[190][3] = 0
	before := p.Clone()
	if _, err := PackProject(p); err == nil {
		t.Fatal("shared stored PCM/YM role was silently rewritten")
	}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("rejected compaction changed project data")
	}
}

func TestPackPreservesIndependentLivePatternRoles(t *testing.T) {
	p := model.New()
	p.Song.Patterns = append(p.Song.Patterns, model.Pattern{})
	p.Song.Patterns[3][0] = model.Cell{Effect1: 'M', Parameter1: 7}
	p.Bank.Sequences[7] = p.Bank.Sequences[2]
	if _, err := PackProjectWithSelections(p, [4]byte{255, 255, 255, 3}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pattern := range p.Song.Patterns {
		if pattern[0].Effect1 == 'M' {
			found = true
			if pattern[0].Parameter1 != 7 {
				t.Fatal("live PCM pattern was remapped as YM command data")
			}
		}
	}
	if !found {
		t.Fatal("live pattern definition was discarded")
	}
}

func TestPackKeepsSharedPatternWhenSequenceNumbersStayUnchanged(t *testing.T) {
	p := model.New()
	p.Bank.SequenceCount = 3
	p.Song.Patterns[0][0] = model.Cell{Effect1: 'M', Parameter1: 2}
	p.Song.Orders[50][3] = 0
	if _, err := PackProject(p); err != nil {
		t.Fatal(err)
	}
	if p.Song.Patterns[p.Song.Orders[0][0]][0].Parameter1 != 2 {
		t.Fatal("shared pattern with unchanged sequence IDs was modified")
	}
}

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

func TestPackSubstitutesNativePresetsWithoutChangingYMOrPCMPlayback(t *testing.T) {
	p := model.New()
	p.Song.Patterns = make([]model.Pattern, 5)
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Song.Patterns[1][0] = model.Cell{Note: model.NoteOff}
	p.Song.Patterns[3][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 64, Parameter1: 2}
	p.Song.Patterns[4][0] = model.Cell{Note: model.NoteOff, Effect1: model.NoteOff}
	p.Song.Length = 3
	p.Song.Orders[0] = [4]byte{0, 2, 2, 3}
	p.Song.Orders[1] = [4]byte{1, 2, 2, 4}
	p.Song.Orders[2] = [4]byte{2, 2, 2, 2}
	for _, base := range []int{4, 52} {
		copy(p.Song.State[base:base+4], p.Song.Orders[0][:])
	}
	p.Song.Orders[200] = [4]byte{1, 2, 1, 4}
	before := p.Clone()
	result, err := PackProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.PatternsAfter != 2 || p.Song.Orders[1][0] != model.NoteOffPattern || p.Song.Orders[1][3] != model.NoteOffPattern || p.Song.Orders[2][0] != model.EmptyPattern || p.Song.Orders[200][3] != model.NoteOffPattern {
		t.Fatalf("exact presets were not substituted throughout the stored song: %+v", result)
	}
	left, right := replay.New(before), replay.New(p)
	left.Play(false)
	right.Play(false)
	for tick := 0; tick < 1300; tick++ {
		left.Tick()
		right.Tick()
		if left.Registers != right.Registers || left.EnvelopeWrite != right.EnvelopeWrite || left.DMA != right.DMA {
			t.Fatalf("preset packing changed YM/PCM playback at tick %d", tick)
		}
	}
	raw, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := native.DecodeSong(raw); err != nil {
		t.Fatal(err)
	}
}

func TestPackKeepsPartialOrSharedNoteOffDefinitions(t *testing.T) {
	for _, shared := range []bool{false, true} {
		p := model.New()
		p.Song.Patterns[0][0] = model.Cell{Note: model.NoteOff}
		p.Song.Orders[0][3] = 0
		if !shared {
			p.Song.Orders[0][0] = model.EmptyPattern
		}
		if _, err := PackProject(p); err != nil {
			t.Fatal(err)
		}
		id := p.Song.Orders[0][3]
		if id >= model.MaxPatterns || p.Song.Patterns[id][0].Note != model.NoteOff || p.Song.Patterns[id][0].Effect1 != 0 {
			t.Fatal("partial PCM or shared note-off acquired a second-lane stop")
		}
	}
}

func TestPackKeepsUnassignedNoteOffMaterialForFutureTrackSelection(t *testing.T) {
	p := model.New()
	p.Song.Patterns = append(p.Song.Patterns, model.Pattern{})
	p.Song.Patterns[3][0].Note = model.NoteOff
	if _, err := PackProject(p); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pattern := range p.Song.Patterns {
		if pattern[0].Note == model.NoteOff && pattern[0].Effect1 == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("unassigned first-lane stop was converted into a future two-lane PCM stop")
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
