package midi

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"testing"
)

func TestRunningStatusWithRealtimeClock(t *testing.T) {
	var d Decoder
	var out [][]byte
	d.Feed([]byte{0x90, 60, 0xf8, 100, 64}, func(b []byte) { out = append(out, append([]byte(nil), b...)) })
	d.Feed([]byte{90}, func(b []byte) { out = append(out, append([]byte(nil), b...)) })
	if len(out) != 3 || out[0][0] != 0xf8 || out[1][1] != 60 || out[2][1] != 64 {
		t.Fatalf("incorrect MIDI framing: %v", out)
	}
}
func TestNotesControllersAndTransportReachTracker(t *testing.T) {
	e := replay.New(model.New())
	Apply(e, []byte{0x90, 69, 127})
	e.Tick()
	if e.Voices[0].Note != 69 || e.Registers[8] != 15 {
		t.Fatal("MIDI note did not reach YM")
	}
	Apply(e, []byte{0xb0, 38, 80})
	if e.MasterVolume != 80 {
		t.Fatal("global volume controller failed")
	}
	Apply(e, []byte{0xfa})
	if !e.Playing {
		t.Fatal("MIDI start failed")
	}
	Apply(e, []byte{0xfc})
	if e.Playing {
		t.Fatal("MIDI stop failed")
	}
}

func TestExternalClockMovesRowsOnlyAfterSixPulses(t *testing.T) {
	p := model.New()
	p.Song.State[31] = 1
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Song.Patterns[0][1] = model.Cell{Note: 64, Instrument: 1}
	e := replay.New(p)
	Apply(e, []byte{0xfa})
	for i := 0; i < 100; i++ {
		e.Tick()
	}
	if e.Row != 0 || e.Voices[0].Note != 60 {
		t.Fatal("internal replay calls advanced externally clocked rows")
	}
	for i := 0; i < 5; i++ {
		Apply(e, []byte{0xf8})
		e.Tick()
	}
	if e.Row != 0 {
		t.Fatal("fewer than six pulses advanced the row")
	}
	Apply(e, []byte{0xf8})
	e.Tick()
	if e.Row != 1 || e.Voices[0].Note != 64 {
		t.Fatal("sixth pulse did not reach the next musical row")
	}
	Apply(e, []byte{0xfc})
	Apply(e, []byte{0xf8})
	Apply(e, []byte{0xfb})
	e.Tick()
	if e.Row != 1 {
		t.Fatal("stop/continue reset the row or counted clocks while stopped")
	}
}

func TestSongPointerAndRealtimeMessagesPreserveMIDIFraming(t *testing.T) {
	var decoder Decoder
	var messages [][]byte
	decoder.Feed([]byte{0xf2, 67, 0xf8, 0, 10, 0xf1, 4, 0x90, 60, 100, 64, 90}, func(data []byte) { messages = append(messages, append([]byte(nil), data...)) })
	if len(messages) != 5 || messages[0][0] != 0xf8 || messages[1][0] != 0xf2 || messages[2][0] != 0xf1 || messages[4][1] != 64 {
		t.Fatalf("system-common/running-status framing failed: %v", messages)
	}
	p := model.Demo()
	e := replay.New(p)
	Apply(e, messages[1])
	if e.Position != 1 || e.Row != 3 || e.Patterns != p.Song.Orders[1] {
		t.Fatal("Song Position Pointer did not select its sixteenth-note location")
	}
}

func TestNextPatternControllersWaitForThePatternBoundary(t *testing.T) {
	p := model.Demo()
	p.Song.SetSpeed(1)
	e := replay.New(p)
	e.Play(false)
	before := e.Patterns[0]
	Apply(e, []byte{0xb0, 44, 3})
	if e.Patterns[0] != before {
		t.Fatal("next-pattern controller interrupted the current pattern")
	}
	for i := 0; i < 64; i++ {
		e.Tick()
	}
	if e.Patterns[0] != 3 {
		t.Fatal("queued pattern did not apply at the boundary")
	}
}

func TestNativeChannelAssignmentsAllocateYMPolyphonyAndTwoPCMVoices(t *testing.T) {
	p := model.New()
	p.Song.State[40], p.Song.State[41] = 7, 7
	p.Song.State[32], p.Song.State[33] = 1, 2
	e := replay.New(p)
	ApplyMapped(e, []byte{0x97, 60, 100})
	ApplyMapped(e, []byte{0x97, 64, 100})
	if e.Voices[0].Note != 60 || e.Voices[1].Note != 64 || e.Voices[1].Instrument != 2 {
		t.Fatal("shared native MIDI assignment did not allocate a free voice")
	}
	ApplyMapped(e, []byte{0x87, 60, 0})
	if e.Voices[0].Note != 0 || e.Voices[1].Note != 64 {
		t.Fatal("note-off stopped the wrong polyphonic voice")
	}
	ApplyMapped(e, []byte{0x93, 72, 127})
	ApplyMapped(e, []byte{0x94, 60, 127})
	if e.DMA[0].Note != 60 || e.DMA[1].Note != 60 || e.DMA[0].Sample != 1 || e.DMA[1].Sample != 1 {
		t.Fatal("MIDI did not trigger both native PCM voices")
	}
	ApplyMapped(e, []byte{0x83, 72, 0})
	if e.DMA[0].Note != 1 || e.DMA[1].Note != 60 {
		t.Fatal("PCM octave-clipped MIDI note-off did not target its voice")
	}
}
