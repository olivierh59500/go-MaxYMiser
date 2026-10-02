package replay

import (
	"encoding/binary"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"testing"
)

func TestNativeA4PeriodAndMixer(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1}
	e := New(p)
	e.Play(false)
	e.Tick()
	period := uint16(e.Registers[0]) | uint16(e.Registers[1])<<8
	if period != 284 || e.Registers[8] != 15 || e.Registers[7]&1 != 0 {
		t.Fatalf("native A4/square setup is wrong: period=%d regs=%v", period, e.Registers)
	}
}
func TestIndependentSequencesAndTwoEffectColumns(t *testing.T) {
	p := model.New()
	p.Bank.Sequences[3] = model.Sequence{Values: [63]uint16{0, 12}, Length: 2, Repeat: 0}
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1, Effect1: 'A', Parameter1: 3, Effect2: 'Z', Parameter2: 42}
	e := New(p)
	e.Play(false)
	e.Tick()
	first := e.Period(&e.Voices[0], 2)
	e.Tick()
	second := e.Period(&e.Voices[0], 2)
	if first != 284 || second != 142 || e.Zync != 42 {
		t.Fatalf("sequence/effects incorrect: %d %d zync=%d", first, second, e.Zync)
	}
}
func TestTrackerVolumeZeroEncodingAndNoteOff(t *testing.T) {
	p := model.New()
	p.Song.SetSpeed(2)
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1, Volume: 16}
	p.Song.Patterns[0][1] = model.Cell{Note: 1}
	e := New(p)
	e.Play(false)
	e.Tick()
	if e.Registers[8] != 15 {
		t.Fatal("displayed volume zero was treated as silence")
	}
	e.Tick()
	e.Tick()
	if e.Registers[8] != 0 {
		t.Fatal("note-off did not silence the channel")
	}
}
func TestLiveTriggerStartsInstrumentWithoutSongPlayback(t *testing.T) {
	p := model.New()
	e := New(p)
	e.Trigger(0, 69, 1)
	e.Tick()
	if e.Playing || !e.Voices[0].Triggered || e.Registers[8] != 15 {
		t.Fatal("live trigger did not reach the synthesizer")
	}
}
func TestAudioReaderProducesRealPCMAndStableClock(t *testing.T) {
	p := model.Demo()
	e := New(p)
	e.Play(false)
	s := NewSynth(e, 48000)
	pcm := make([]byte, 48000*4)
	n, err := s.Read(pcm)
	if err != nil || n != len(pcm) {
		t.Fatal("PCM render failed")
	}
	peak := int16(0)
	for i := 0; i < len(pcm); i += 2 {
		value := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if value > peak {
			peak = value
		}
	}
	if peak < 100 || e.Ticks != 50 {
		t.Fatalf("silent output or wrong tick rate: peak=%d ticks=%d", peak, e.Ticks)
	}
}

func TestStopAndResetDiscardPendingPreviewTriggers(t *testing.T) {
	e := New(model.New())
	e.Trigger(0, 69, 1)
	e.Stop()
	e.Tick()
	if e.Voices[0].Triggered || e.Registers[8] != 0 {
		t.Fatal("stopped preview was triggered again")
	}
	e.Trigger(0, 69, 1)
	e.Reset()
	e.Tick()
	if e.Voices[0].Triggered || e.Registers[8] != 0 {
		t.Fatal("reset retained a preview from the previous project")
	}
}

func TestJamMarkersLoopTheSelectedSection(t *testing.T) {
	p := model.New()
	p.Song.Length = 5
	p.Song.Orders[1] = [4]byte{model.LoopPattern, 255, 255, 255}
	p.Song.Orders[2] = [4]byte{2, 255, 255, 255}
	p.Song.Orders[3] = [4]byte{1, 255, 255, 255}
	p.Song.Orders[4] = [4]byte{model.LoopPattern, 255, 255, 255}
	e := New(p)
	e.Jam = true
	e.Position = 4
	e.Play(false)
	if e.Position != 2 || e.Patterns[0] != 2 {
		t.Fatalf("jam marker did not return to its section: position=%d patterns=%v", e.Position, e.Patterns)
	}
	e.Jam = false
	e.Position = 1
	e.Play(false)
	if e.Position != 2 {
		t.Fatal("normal transport did not skip a jam marker")
	}
}
