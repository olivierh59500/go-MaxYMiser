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
		if message.Data[0]&0xf0 == 0x90 {
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
		if message.Data[0]&0xf0 == 0x80 {
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
	for i := 0; i < 300; i++ {
		s.midi.push(0xf8)
	}
	var out [256]MIDIMessage
	count, dropped := s.DrainMIDI(out[:])
	if count != 256 || dropped != 44 {
		t.Fatalf("unbounded or hidden overflow: %d %d", count, dropped)
	}
}
