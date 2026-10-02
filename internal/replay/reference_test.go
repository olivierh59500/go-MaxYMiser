package replay

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"testing"
)

func TestPausingReferenceDoesNotRestartTheEditableNativeTransport(t *testing.T) {
	e := New(model.Demo())
	s := NewSynth(e, 48000)
	raw := make([]byte, 4+14*100)
	copy(raw, "YM3!")
	if err := s.LoadYM(raw); err != nil {
		t.Fatal(err)
	}
	e.Position, e.Row, e.TickInRow = 1, 17, 2
	s.PauseReference()
	r, ok := s.Reference()
	if !ok || r.Active || r.Playing || e.Position != 1 || e.Row != 17 || e.TickInRow != 2 {
		t.Fatal("pausing the register reference reset the native transport")
	}
}

func TestYMReferenceLoadsWithoutReconstructingTrackerPatterns(t *testing.T) {
	raw := append([]byte(nil), []byte("YM3!")...)
	regs := [14]byte{28, 1, 0, 0, 0, 0, 0, 62, 15, 0, 0, 0, 0, 0}
	for _, value := range regs {
		for range 4 {
			raw = append(raw, value)
		}
	}
	engine := New(model.Demo())
	synth := NewSynth(engine, 48000)
	if err := synth.LoadYM(raw); err != nil {
		t.Fatal(err)
	}
	before := engine.Project.Song.Patterns[0]
	pcm := make([]byte, 480*4)
	if _, err := synth.Read(pcm); err != nil {
		t.Fatal(err)
	}
	reference, ok := synth.Reference()
	if !ok || reference.Duration != 80 || reference.Registers[8] != 15 || engine.Ticks != 0 {
		t.Fatalf("YM playback did not use its own decoder: %+v ticks=%d", reference, engine.Ticks)
	}
	if before != engine.Project.Song.Patterns[0] {
		t.Fatal("YM import invented tracker patterns")
	}
	synth.ToggleYM()
	clear(pcm)
	synth.Read(pcm)
	for _, b := range pcm {
		if b != 0 {
			t.Fatal("paused YM reference still sounded")
		}
	}
	synth.CloseYM()
	if _, ok := synth.Reference(); ok {
		t.Fatal("YM reference was retained after close")
	}
}
