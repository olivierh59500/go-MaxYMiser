package midi

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestMicrowireControllersMatchEveryCapturedNativeCommand(t *testing.T) {
	// The fixture contains the big-endian D0 words emitted by the original
	// STe paths for CC48 through CC51, each with values 0 through 127.
	// Pan emits two words; the other controls emit one. See replay verification.
	raw, err := os.ReadFile("testdata/microwire-1.67.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1280 {
		t.Fatalf("native command fixture contains %d bytes, want 1280", len(raw))
	}
	at := 0
	for code := byte(48); code <= 51; code++ {
		for value := byte(0); value < 128; value++ {
			p := model.New()
			p.Song.State[31] |= 4
			p.Song.State[49] = 2
			got, want := replay.New(p), replay.New(p.Clone())
			commands := 1
			if code == 49 {
				commands = 2
			}
			for range commands {
				want.SetMicrowire(binary.BigEndian.Uint16(raw[at:]))
				at += 2
			}
			ApplyMapped(got, []byte{0xbf, code, value})
			if got.MicrowireGain != want.MicrowireGain || got.MicrowireLeft != want.MicrowireLeft || got.MicrowireRight != want.MicrowireRight || got.Bass != want.Bass || got.Treble != want.Treble {
				t.Fatalf("CC%d value %d differs from its captured native commands", code, value)
			}
		}
	}
}

func TestMicrowireControllersUseNativeSTeLevels(t *testing.T) {
	// Expected decibel levels follow the 1.67 editor's STe controller paths,
	// including the two center pan plateaus and integer tone-control division.
	for _, test := range []struct {
		code, value                       byte
		master, left, right, bass, treble int
	}{
		{48, 0, -80, 0, 0, 6, 6},
		{48, 31, -62, 0, 0, 6, 6},
		{48, 63, -40, 0, 0, 6, 6},
		{48, 64, -40, 0, 0, 6, 6},
		{48, 126, 0, 0, 0, 6, 6},
		{48, 127, 0, 0, 0, 6, 6},
		{49, 0, 0, 0, -40, 6, 6},
		{49, 32, 0, 0, -20, 6, 6},
		{49, 63, 0, 0, 0, 6, 6},
		{49, 64, 0, 0, 0, 6, 6},
		{49, 95, 0, -20, 0, 6, 6},
		{49, 127, 0, -40, 0, 6, 6},
		{50, 9, 0, 0, 0, 0, 6},
		{50, 10, 0, 0, 0, 1, 6},
		{50, 20, 0, 0, 0, 2, 6},
		{50, 69, 0, 0, 0, 6, 6},
		{50, 70, 0, 0, 0, 7, 6},
		{50, 127, 0, 0, 0, 12, 6},
		{51, 10, 0, 0, 0, 6, 1},
		{51, 70, 0, 0, 0, 6, 7},
		{51, 127, 0, 0, 0, 6, 12},
	} {
		t.Run(fmt.Sprintf("CC%d_%d", test.code, test.value), func(t *testing.T) {
			p := model.New()
			p.Song.State[31] |= 4
			p.Song.State[49] = 2
			e := replay.New(p)
			e.MasterVolume, e.Pan = 93, -3
			ApplyMapped(e, []byte{0xbf, test.code, test.value})
			for _, level := range []struct {
				name string
				got  float64
				db   int
			}{{"master", e.MicrowireGain, test.master}, {"left", e.MicrowireLeft, test.left}, {"right", e.MicrowireRight, test.right}} {
				if want := math.Pow(10, float64(level.db)/20); math.Abs(level.got-want) > 1e-12 {
					t.Errorf("%s gain = %g, want %g (%d dB)", level.name, level.got, want, level.db)
				}
			}
			if e.Bass != test.bass || e.Treble != test.treble {
				t.Errorf("tone levels = %d/%d, want %d/%d", e.Bass, e.Treble, test.bass, test.treble)
			}
			if e.MasterVolume != 93 || e.Pan != -3 {
				t.Fatal("hardware controllers replaced independent editor controls")
			}
		})
	}
}

func TestMicrowireControllersRespectDMAAndControllerEnable(t *testing.T) {
	for _, enableControllers := range []bool{false, true} {
		for _, enableDMA := range []bool{false, true} {
			if enableControllers && enableDMA {
				continue
			}
			p := model.New()
			p.Song.State[31], p.Song.State[49] = 0, 0
			if enableControllers {
				p.Song.State[31] = 4
			}
			if enableDMA {
				p.Song.State[49] = 2
			}
			e := replay.New(p)
			e.MasterVolume, e.Pan = 91, 2
			for code := byte(48); code <= 51; code++ {
				ApplyMapped(e, []byte{0xbf, code, 0})
			}
			if e.MasterVolume != 91 || e.Pan != 2 || e.MicrowireGain != 1 || e.MicrowireLeft != 1 || e.MicrowireRight != 1 || e.Bass != 6 || e.Treble != 6 {
				t.Fatalf("disabled hardware controls changed audio: controllers=%v DMA=%v", enableControllers, enableDMA)
			}
		}
	}
}
