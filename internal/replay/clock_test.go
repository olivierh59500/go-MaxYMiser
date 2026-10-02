package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestExternalPulsesAdvanceTheWholeReplayerAndRetainItsSixCallRows(t *testing.T) {
	p := model.New()
	p.Song.State[31] = 1
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 'S', Parameter1: 2}
	p.Song.Patterns[0][1] = model.Cell{Note: 64, Instrument: 1}
	p.Bank.Sequences[1].Length, p.Bank.Sequences[1].Repeat = 8, 7
	p.Bank.Sequences[1].Values = [63]uint16{15, 14, 13, 12, 11, 10, 9, 8}
	e := New(p)
	e.Play(false)
	for pulse := 0; pulse < 6; pulse++ {
		e.ClockPulse()
		e.Tick()
		if e.Registers[8] != byte(15-pulse) || e.Voices[0].Note != 60 {
			t.Fatalf("native envelope call %d: volume=%d note=%d", pulse+1, e.Registers[8], e.Voices[0].Note)
		}
		registers, voices, ticks := e.Registers, e.Voices, e.Ticks
		for range 20 {
			e.Tick()
		}
		if e.Registers != registers || e.Voices[0].SeqIndex != voices[0].SeqIndex || e.Ticks != ticks {
			t.Fatal("internal callbacks advanced the externally clocked instrument")
		}
	}
	if e.Row != 1 || e.TickInRow != 0 || e.Speed != 6 {
		t.Fatal("external rows applied a disabled S command")
	}
	e.ClockPulse()
	e.Tick()
	if e.Voices[0].Note != 64 || e.Registers[8] != 15 {
		t.Fatal("seventh replay call did not initialize the following row")
	}
}

func TestCompensationUsesAllEightBitsAndCrossesOrderBoundaries(t *testing.T) {
	p := model.Demo()
	p.Song.State[31], p.Song.State[57] = 1, 255
	e := New(p)
	e.Play(false)
	e.CompensateClockLatency()
	e.Tick()
	if e.Position != 0 || e.Row != 42 || e.TickInRow != 3 || e.Ticks != 255 {
		t.Fatal("255 compensation pulses disagree with the original editor's captured start")
	}
	e.Reset()
	e.Play(false)
	e.SetSongPointer(63)
	e.CompensateClockLatency()
	e.Tick()
	if e.Position != 1 || e.Row != 41 || e.TickInRow != 3 || e.Patterns != p.Song.Orders[1] || e.Ticks != 255 {
		t.Fatalf("full-range native compensation lost a boundary: position=%d row=%d pulse=%d ticks=%d", e.Position, e.Row, e.TickInRow, e.Ticks)
	}
}

func TestQueuedExternalPulsesRetainAudioTriggersUntilConsumed(t *testing.T) {
	p := model.New()
	p.Song.State[31] = 1
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60, Instrument: 1}
	e := New(p)
	e.Play(false)
	for range 5 {
		e.ClockPulse()
	}
	e.Tick()
	if !e.DMA[0].Triggered || e.Row != 0 || e.TickInRow != 5 {
		t.Fatal("queued compensation pulses discarded the PCM trigger")
	}
	e.Tick()
	if e.DMA[0].Triggered {
		t.Fatal("a callback without a pulse retriggered the PCM sample")
	}
}

func TestCompensatedPCMPlaybackStartsOnceWhenTheAudioPassConsumesItsPulses(t *testing.T) {
	p := model.New()
	p.Song.State[31], p.Song.State[57], p.Song.State[49] = 1, 7, 1
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60, Instrument: 1}
	p.Bank.Samples[0].PCM = make([]byte, 1000)
	e := New(p)
	e.Play(false)
	e.CompensateClockLatency()
	s := NewSynth(e, 48000)
	s.Read(make([]byte, 400))
	if !s.pcm[0].active || e.Ticks != 7 || e.Row != 1 || e.TickInRow != 1 {
		t.Fatal("compensation ran before the audio pass and lost its sample trigger")
	}
	position := s.pcm[0].position
	s.Read(make([]byte, 7200))
	if s.pcm[0].position <= position || e.Ticks != 7 {
		t.Fatal("internal audio callbacks retriggered the sample or advanced clocked music")
	}
}
