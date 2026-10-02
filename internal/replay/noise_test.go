package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestNoisePeriodUsesNativeSignedByteAdditionBeforeClamping(t *testing.T) {
	for _, c := range []struct {
		name         string
		word         uint16
		voice, track int
		want         byte
	}{
		{"ordinary", 12, 2, 3, 17},
		{"upper word is ignored", 0x0105, 0, 0, 5},
		{"negative sequence", 175, 0, 0, 0},
		{"above YM range", 30, 5, 0, 31},
		{"signed overflow", 120, 5, 5, 0},
		{"wrap back to positive", 254, 3, 0, 1},
		{"negative transpose", 0, -1, 0, 0},
		{"both transposes wrap", 255, -1, 3, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New(model.New())
			e.Voices[0] = Voice{Note: 60, NoiseTranspose: c.voice, TrackNoiseTranspose: c.track}
			e.Voices[0].Values[0], e.Voices[0].Values[3], e.Voices[0].Values[4] = 15, 0x1000, c.word
			e.configure()
			if e.Registers[6] != c.want || e.Registers[7]&8 != 0 {
				t.Fatalf("noise period %d, want %d; mixer %02x", e.Registers[6], c.want, e.Registers[7])
			}
		})
	}
}

func TestZeroTonePeriodRetainsTheNativeMixerUntilNoteOff(t *testing.T) {
	e := New(model.New())
	e.Voices[0].Note = 14
	e.Voices[0].Parameters[4] = 4
	e.Voices[0].Values[0], e.Voices[0].Values[3] = 15, 0x0100
	e.configure()
	if e.Registers[0] != 0 || e.Registers[1] != 0 || e.Registers[7]&1 != 0 || e.Registers[8] != 15 {
		t.Fatal("zero fixed period disabled an otherwise sounding native tone")
	}
	e.Voices[0].Note = 0
	e.configure()
	if e.Registers[7]&1 == 0 || e.Registers[8] != 0 {
		t.Fatal("note off did not silence the zero-period tone")
	}
}
