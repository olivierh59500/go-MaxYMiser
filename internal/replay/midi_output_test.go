package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMIDIOutputClockAndPCMNotesFollowAudioTransport(t *testing.T) {
	p := model.New()
	p.Song.State[49] = 4
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60, Instrument: 2, Effect1: 64, Parameter1: 3}
	e := New(p)
	e.Play(false)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	pcm := make([]byte, 48000*4)
	s.Read(pcm)
	var messages [256]MIDIMessage
	count, dropped := s.DrainMIDI(messages[:])
	clocks, notes := 0, 0
	for _, message := range messages[:count] {
		if message.Data[0] == 0xf8 {
			clocks++
		}
		if message.Data[0]&0xf0 == 0x90 && message.Data[2] > 0 {
			notes++
		}
	}
	if clocks != 50 || notes != 2 || dropped != 0 {
		t.Fatalf("clock/notes output wrong: clocks=%d notes=%d dropped=%d", clocks, notes, dropped)
	}
	e.Stop()
	s.Read(pcm[:4000])
	count, _ = s.DrainMIDI(messages[:])
	offs, stop := 0, false
	for _, message := range messages[:count] {
		if message.Data[0]&0xf0 == 0x90 && message.Data[2] == 0 {
			offs++
		}
		stop = stop || message.Data[0] == 0xfc
	}
	if offs != 2 || !stop {
		t.Fatal("stop did not release both MIDI notes and transport")
	}
}

func TestMIDIOutputQueueOverflowIsBoundedAndReported(t *testing.T) {
	s := NewSynth(New(model.New()), 48000)
	s.EnableMIDIOutput(true)
	for i := 0; i < 2092; i++ {
		s.midi.push(0xf8)
	}
	var out [2048]MIDIMessage
	count, dropped := s.DrainMIDI(out[:])
	if count != 2048 || dropped != 44 {
		t.Fatalf("unbounded or hidden overflow: %d %d", count, dropped)
	}
}

func drainMessages(s *Synth) []MIDIMessage {
	var buffer [2048]MIDIMessage
	count, _ := s.DrainMIDI(buffer[:])
	return append([]MIDIMessage(nil), buffer[:count]...)
}

func assertMIDI(t *testing.T, s *Synth, expected ...[3]byte) {
	t.Helper()
	got := drainMessages(s)
	if len(got) != len(expected) {
		t.Fatalf("MIDI event count %d, want %d: %v", len(got), len(expected), got)
	}
	for i, want := range expected {
		if got[i].Size != 3 || got[i].Data != want {
			t.Fatalf("MIDI event %d: %v, want %v", i, got[i], want)
		}
	}
}

func TestMIDINoteValuesAndOrderingMatchCapturedNativeCases(t *testing.T) {
	p := model.New()
	p.Song.State[49] = 4
	e := New(p)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	e.DMA[0].Transpose, e.DMA[0].TrackVolume = 12, 1
	trigger := func(note, sample, volume byte) {
		e.TriggerSample(0, note, sample)
		e.DMA[0].Volume = volume
		e.Tick()
		s.midiNotes()
	}
	// Native capture: C4 + 12 semitones, attenuation 2 + 1 => 79.
	trigger(60, 0, 2)
	assertMIDI(t, s, [3]byte{0x93, 72, 79})
	trigger(64, 0, 2)
	assertMIDI(t, s, [3]byte{0x93, 76, 79}, [3]byte{0x93, 72, 0})
	trigger(67, 2, 2)
	assertMIDI(t, s, [3]byte{0x93, 76, 0}, [3]byte{0x93, 79, 79})
	trigger(1, 0, 2)
	assertMIDI(t, s, [3]byte{0x93, 79, 0})
	e.DMA[0].Transpose, e.DMA[0].TrackVolume = -12, 0
	trigger(12, 1, 0)
	assertMIDI(t, s, [3]byte{0x93, 0, 127})
	e.DMA[0].Transpose, e.DMA[0].TrackVolume = 0, 1
	trigger(24, 0, 7)
	assertMIDI(t, s, [3]byte{0x93, 24, 0}, [3]byte{0x93, 0, 0})
	if s.midi.active[0] {
		t.Fatal("native attenuation eight left a sounding MIDI voice")
	}
}

func TestMIDIChannelChangesAndDisconnectionReleaseTheActualTransposedNote(t *testing.T) {
	p := model.New()
	p.Song.State[49] = 4
	e := New(p)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	e.DMA[0].Transpose = -12
	e.TriggerSample(0, 12, 0)
	e.Tick()
	s.midiNotes()
	assertMIDI(t, s, [3]byte{0x93, 0, 127})
	p.Song.State[43] = 7
	e.TriggerSample(0, 24, 0)
	e.Tick()
	s.midiNotes()
	assertMIDI(t, s, [3]byte{0x93, 0, 0}, [3]byte{0x97, 12, 127})
	s.EnableMIDIOutput(false)
	assertMIDI(t, s, [3]byte{0x97, 12, 0})
}

func TestExternalClockRelayPreservesEveryIntermediateNoteInAnInputBatch(t *testing.T) {
	p := model.New()
	p.Song.State[31], p.Song.State[49] = 1, 4
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60}
	p.Song.Patterns[2][1] = model.Cell{Note: 64}
	e := New(p)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	s.Edit(func(e *Engine) { e.Play(false) })
	s.Edit(func(e *Engine) {
		for range 7 {
			e.ClockPulse()
		}
	})
	s.Read(make([]byte, 400))
	got := drainMessages(s)
	if len(got) != 11 || got[0].Data[0] != 0xfa || got[1].Data[0] != 0xf8 || got[2].Data != [3]byte{0x93, 60, 127} || got[8].Data[0] != 0xf8 || got[9].Data != [3]byte{0x93, 64, 127} || got[10].Data != [3]byte{0x93, 60, 0} {
		t.Fatalf("batched clock/notes lost native ordering: %v", got)
	}
	s.Read(make([]byte, 8000))
	if got := drainMessages(s); len(got) != 0 {
		t.Fatalf("internal callbacks duplicated externally relayed events: %v", got)
	}
}

func TestMIDITransportKeepsContinueAndRestartDistinctWithinOneAudioBuffer(t *testing.T) {
	e := New(model.New())
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	s.Edit(func(e *Engine) { e.Play(false) })
	s.Edit(func(e *Engine) { e.Stop() })
	s.Edit(func(e *Engine) { e.Continue() })
	s.Edit(func(e *Engine) { e.Play(false) })
	got := drainMessages(s)
	if len(got) != 4 || got[0].Data[0] != 0xfa || got[1].Data[0] != 0xfc || got[2].Data[0] != 0xfb || got[3].Data[0] != 0xfa {
		t.Fatalf("transport events were collapsed or Continue became Start: %v", got)
	}
}

func TestMaximumNativeCompensationFitsTheOutputQueueWithDensePCMNotes(t *testing.T) {
	p := model.New()
	p.Song.State[31], p.Song.State[57], p.Song.State[49] = 1, 255, 4
	p.Song.SetSpeed(1)
	p.Song.Orders[0][3] = 2
	for row := range p.Song.Patterns[2] {
		p.Song.Patterns[2][row] = model.Cell{Note: byte(60 + row%12), Effect1: byte(72 + row%12)}
	}
	e := New(p)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	s.Edit(func(e *Engine) { e.Play(false); e.CompensateClockLatency() })
	s.Read(make([]byte, 400))
	var messages [2048]MIDIMessage
	count, dropped := s.DrainMIDI(messages[:])
	clocks := 0
	for _, message := range messages[:count] {
		if message.Data[0] == 0xf8 {
			clocks++
		}
	}
	if clocks != 255 || dropped != 0 || count != 1274 {
		t.Fatalf("full native compensation lost clock/note events: clocks=%d count=%d dropped=%d", clocks, count, dropped)
	}
}

func TestStartingAwayFromTheBeginningUsesSongPointerAndContinue(t *testing.T) {
	e := New(model.Demo())
	e.SelectPosition(1)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	s.Edit(func(e *Engine) { e.Play(false) })
	got := drainMessages(s)
	if len(got) != 2 || got[0].Data != [3]byte{0xf2, 64, 0} || got[0].Size != 3 || got[1].Data[0] != 0xfb || got[1].Size != 1 {
		t.Fatalf("nonzero song position was followed by a resetting Start: %v", got)
	}
}

func TestRegisterReferencePlaybackReleasesMidiNotesWithoutAnotherTrackerTick(t *testing.T) {
	p := model.New()
	p.Song.State[49] = 4
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60}
	e := New(p)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	s.Edit(func(e *Engine) { e.Play(false) })
	s.Read(make([]byte, 400))
	drainMessages(s)
	raw := make([]byte, 4+14*100)
	copy(raw, "YM3!")
	if err := s.LoadYM(raw); err != nil {
		t.Fatal(err)
	}
	defer s.CloseYM()
	s.Read(make([]byte, 400))
	got := drainMessages(s)
	if len(got) != 2 || got[0].Data != [3]byte{0x93, 60, 0} || got[1].Data[0] != 0xfc {
		t.Fatalf("switching to the register reference retained sounding MIDI notes: %v", got)
	}
}

func TestExternalOutputAudioPassDoesNotAllocate(t *testing.T) {
	p := model.New()
	p.Song.State[31], p.Song.State[49] = 1, 4
	p.Song.Orders[0][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60}
	e := New(p)
	e.Play(false)
	s := NewSynth(e, 48000)
	s.EnableMIDIOutput(true)
	buffer := make([]byte, 3840)
	allocations := testing.AllocsPerRun(20, func() {
		e.ClockPulse()
		s.Read(buffer)
	})
	if allocations != 0 {
		t.Fatalf("external-clock output allocated in the audio callback: %f", allocations)
	}
}
