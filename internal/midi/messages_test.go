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
