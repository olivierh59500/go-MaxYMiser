package replay

import (
	"math"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestPCMUsesNativeNoteRatesAndModeAllocation(t *testing.T) {
	for _, test := range []struct {
		note, mode byte
		rate       int
	}{{24, 2, 2072}, {48, 2, 8287}, {60, 2, 16574}, {72, 2, 16574}, {23, 2, 0}, {24, 3, 6258}, {48, 3, 25033}, {60, 3, 50066}} {
		if got := pcmRate(test.note, test.mode); got != test.rate {
			t.Fatalf("note %d mode %d rate %d, want %d", test.note, test.mode, got, test.rate)
		}
	}
	p := model.New()
	p.Bank.Samples[0].PCM = make([]byte, 1000)
	e := New(p)
	e.TriggerSample(0, 60, 1)
	e.TriggerSample(1, 60, 1)
	e.Tick()
	s := NewSynth(e, 48000)
	s.configure()
	if !s.pcm[0].active || !s.pcm[1].active || s.pcm[0].volume != 1 || math.Abs(s.pcm[0].step-16574.0/25033) > 1e-9 || math.Abs(s.pcm[0].dacStep-48000.0/25033) > 1e-9 {
		t.Fatal("two-voice mixing did not use native pitch and headroom")
	}
	p.Song.State[49] = 1
	e.TriggerSample(0, 60, 1)
	e.Tick()
	s.configure()
	if s.pcm[1].active || s.pcm[0].volume != 0 {
		t.Fatal("one-voice mode retained a second voice or mixing attenuation")
	}
	p.Song.State[49] = 0
	s.configure()
	if s.pcm[0].active {
		t.Fatal("disabled DMA mode still played samples")
	}
}

func TestPCMNoteOffAndVolumeOnlyRowsDoNotRestartSamples(t *testing.T) {
	e := New(model.New())
	e.parseDMA(model.Cell{Note: 72, Instrument: 1, Volume: 3, Effect1: 60, Parameter1: 2, Effect2: 4})
	if e.DMA[0].Note != 60 || e.DMA[0].Volume != 3 || !e.DMA[1].Triggered {
		t.Fatal("native note range or second PCM lane was lost")
	}
	e.DMA[0].Triggered, e.DMA[1].Triggered = false, false
	e.parseDMA(model.Cell{Volume: 5})
	if e.DMA[0].Triggered || e.DMA[0].Volume != 5 {
		t.Fatal("volume-only command restarted a sample")
	}
	e.parseDMA(model.Cell{Note: 1})
	if e.DMA[0].Sample != 0 || !e.DMA[0].Triggered {
		t.Fatal("PCM note-off retained an active sample")
	}
	e.parseDMA(model.Cell{Instrument: 2})
	if e.DMA[0].Volume != 0 {
		t.Fatal("new PCM sample did not reset column attenuation")
	}
}

func TestPCMUsesSTeDACHoldAndNativeModeSkipsInitialByte(t *testing.T) {
	p := model.New()
	p.Bank.Samples[0].PCM = []byte{10, 20, 30, 40, 50}
	e := New(p)
	e.TriggerSample(0, 48, 1)
	e.Tick()
	s := NewSynth(e, 48000)
	s.configure()
	if s.pcm[0].dacStep != 48000.0/25033 || s.pcm[0].step != 8287.0/25033 {
		t.Fatal("resampled PCM bypassed native STe DAC cadence")
	}
	p.Song.State[49] = 3
	e.TriggerSample(0, 48, 1)
	e.Tick()
	s.configure()
	if s.pcm[0].position != 1 || s.pcm[0].dacStep != 48000.0/25033 || s.pcm[0].step != 1 {
		t.Fatal("native PCM start/cadence does not match STe")
	}
}

func TestLivePCMTransposeAndTrackAttenuationDoNotRestartTheSample(t *testing.T) {
	p := model.New()
	p.Bank.Samples[0].PCM = make([]byte, 1000)
	e := New(p)
	e.TriggerSample(0, 48, 1)
	e.Tick()
	s := NewSynth(e, 48000)
	s.configure()
	s.pcm[0].position = 17
	e.DMA[0].Triggered = false
	e.DMA[0].Transpose = 12
	e.DMA[0].TrackVolume = 2
	s.configure()
	if s.pcm[0].position != 17 || s.pcm[0].volume != 3 || math.Abs(s.pcm[0].step-16574.0/25033) > 1e-9 {
		t.Fatal("PCM controller restarted the sample or ignored native pitch/attenuation")
	}
	e.TriggerSample(0, 48, 1)
	if e.DMA[0].Transpose != 12 || e.DMA[0].TrackVolume != 2 {
		t.Fatal("new live sample lost track controllers")
	}
}
