package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMutedYMTrackStillAllowsLiveNotesAndSkipsItsRecordedNotes(t *testing.T) {
	p := model.New()
	p.Song.State[37] = 1
	p.Song.SetSpeed(1)
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Song.Patterns[0][1] = model.Cell{Note: 64, Instrument: 1}
	e := New(p)
	e.Play(false)
	e.Trigger(0, 69, 1)
	e.Tick()
	if e.Voices[0].Note != 69 || e.Registers[8] != 15 {
		t.Fatal("mute silenced the live keyboard or replayed the muted score")
	}
	e.Tick()
	if e.Voices[0].Note != 69 || e.Registers[8] != 15 {
		t.Fatal("muted sequencing overwrote the live note")
	}
	e.Trigger(0, 1, 0)
	e.Tick()
	if e.Registers[8] != 0 {
		t.Fatal("muted live note-off was ignored")
	}
}

func TestMutedPCMTrackAllowsSamplePreviewWithoutSequencingTheScore(t *testing.T) {
	p := model.New()
	p.Song.State[37] = 8
	p.Bank.Samples[0].PCM = make([]byte, 1000)
	p.Song.Patterns[0][0] = model.Cell{Note: 24, Instrument: 2}
	e := New(p)
	e.Patterns[3] = 0
	e.Play(true)
	e.TriggerSample(0, 60, 1)
	e.Tick()
	s := NewSynth(e, 48000)
	s.configure()
	if e.DMA[0].Note != 60 || e.DMA[0].Sample != 1 || !s.pcm[0].active {
		t.Fatal("mute prevented live PCM preview or replaced it from the score")
	}
	e.TriggerSample(0, 1, 0)
	e.Tick()
	s.configure()
	if s.pcm[0].active {
		t.Fatal("live sample note-off did not stop a muted-track preview")
	}
}
