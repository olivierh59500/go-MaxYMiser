package midi

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestNativeControllerEnableAndGlobalScope(t *testing.T) {
	p := model.New()
	e := replay.New(p)
	ApplyMapped(e, []byte{0xbf, 21, 80})
	if e.Speed != p.Song.Speed() {
		t.Fatal("disabled native controllers changed the tempo")
	}
	p.Song.State[31] |= 4
	ApplyMapped(e, []byte{0xbf, 21, 80})
	if e.Speed != 12 {
		t.Fatal("global speed controller did not use native seven-bit quantization")
	}
	ApplyMapped(e, []byte{0xbf, 16, 0})
	if e.Mutes&1 == 0 {
		t.Fatal("native low controller value did not mute the sequencer")
	}
	ApplyMapped(e, []byte{0xbf, 16, 127})
	if e.Mutes&1 != 0 {
		t.Fatal("native high controller value did not unmute the sequencer")
	}
	ApplyMapped(e, []byte{0xbf, 33, 60})
	if e.Voices[0].TrackTranspose != -2 {
		t.Fatal("signed semitone controller scale was not preserved")
	}
	ApplyMapped(e, []byte{0xbf, 36, 88})
	ApplyMapped(e, []byte{0xbf, 42, 90})
	if e.DMA[0].Transpose != 12 || e.DMA[0].TrackVolume != 2 {
		t.Fatal("global MIDI controls did not reach the PCM track")
	}
}

func TestInstrumentControllersEditTheAssignedDefinitionWithNativeScales(t *testing.T) {
	for _, test := range []struct{ code, value, offset, want byte }{{60, 126, 48, 63}, {67, 126, 55, 63}, {109, 0, 38, 15}, {109, 127, 38, 0}, {110, 127, 32, 32}, {111, 127, 36, 8}, {112, 0, 37, 151}, {112, 127, 37, 24}, {113, 127, 34, 15}, {114, 127, 35, 1}, {115, 60, 39, 252}, {116, 68, 40, 4}, {117, 127, 33, 254}} {
		p := model.New()
		p.Song.State[31] = 4
		p.Song.State[40] = 7
		p.Song.State[32] = 3
		e := replay.New(p)
		e.Trigger(0, 69, 3)
		ApplyMapped(e, []byte{0xb7, test.code, test.value})
		if p.Bank.Instruments[2][test.offset] != test.want || e.Voices[0].Parameters[test.offset-16] != test.want {
			t.Fatalf("CC %d: bank/live %d/%d, want %d", test.code, p.Bank.Instruments[2][test.offset], e.Voices[0].Parameters[test.offset-16], test.want)
		}
		if p.Bank.Instruments[0][test.offset] == test.want && test.want != model.New().Bank.Instruments[0][test.offset] {
			t.Fatal("controller changed an unrelated instrument")
		}
	}
}

func TestNativeDDPercussionUsesMiddleCAndReleasesItsMappedSound(t *testing.T) {
	p := model.New()
	p.Song.State[32] = 0xdd
	e := replay.New(p)
	ApplyMapped(e, []byte{0x90, 61, 127})
	if e.Voices[0].Note != 60 || e.Voices[0].Instrument != 30 {
		t.Fatal("native DD mode did not map the key to a middle-C instrument")
	}
	ApplyMapped(e, []byte{0x80, 61, 0})
	if e.Voices[0].Note != 0 {
		t.Fatal("DD note-off retained the mapped percussion note")
	}
}

func TestLiveTrackControlsSurviveInstrumentNotesAndStop(t *testing.T) {
	p := model.New()
	p.Song.State[31] = 4
	e := replay.New(p)
	ApplyMapped(e, []byte{0xb0, 33, 88})
	ApplyMapped(e, []byte{0xb0, 29, 60})
	ApplyMapped(e, []byte{0xb0, 39, 80})
	ApplyMapped(e, []byte{0x90, 69, 127})
	e.Tick()
	if e.Voices[0].TrackTranspose != 12 || e.Voices[0].TrackNoiseTranspose != 2 || e.Registers[0] != 142 || e.Registers[8] != 10 {
		t.Fatal("new instrument trigger discarded live track pitch/noise/volume controls")
	}
	ApplyMapped(e, []byte{0x80, 69, 0})
	if e.Voices[0].TrackTranspose != 12 || e.Voices[0].TrackVolume != 5 {
		t.Fatal("note-off discarded track controls")
	}
	e.Stop()
	if e.Voices[0].TrackNoiseTranspose != 2 || e.Voices[0].TrackVolume != 5 {
		t.Fatal("Stop discarded native live track controls")
	}
	e.Reset()
	if e.Voices[0].TrackVolume != 0 {
		t.Fatal("a new project retained old live controls")
	}
}
