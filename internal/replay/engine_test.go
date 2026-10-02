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

func TestReusableSnapshotDoesNotAliasEditedSourceData(t *testing.T) {
	p := model.Demo()
	p.Bank.Samples[0].PCM = []byte{1, 2, 3}
	s := NewSynth(New(p), 48000)
	var view model.Project
	first, _ := s.SnapshotInto(&view)
	s.Edit(func(e *Engine) {
		e.Project.Song.Patterns[0][0].Note = 70
		e.Project.Bank.Sequences[3].Values[1] = 8
		e.Project.Bank.Samples[0].PCM[0] = 9
	})
	if first.Project.Song.Patterns[0][0].Note == 70 || first.Project.Bank.Sequences[3].Values[1] == 8 || first.Project.Bank.Samples[0].PCM[0] == 9 {
		t.Fatal("editor view retained mutable source data")
	}
	second, _ := s.SnapshotInto(&view)
	if second.Project.Song.Patterns[0][0].Note != 70 || second.Project.Bank.Samples[0].PCM[0] != 9 {
		t.Fatal("reused view did not refresh edited values")
	}
	if allocations := testing.AllocsPerRun(20, func() { s.SnapshotInto(&view) }); allocations != 0 {
		t.Fatalf("steady-state editor snapshots allocate: %f", allocations)
	}
}

func TestLiveInstrumentPreviewReloadsEditedParameters(t *testing.T) {
	p := model.New()
	e := New(p)
	e.Trigger(0, 69, 1)
	p.Bank.Instruments[0][33] = 64
	e.Trigger(0, 69, 1)
	if e.Voices[0].Parameters[17] != 64 {
		t.Fatal("preview retained stale instrument settings")
	}
}

func TestInstrumentRetriggerRestoresParametersChangedByEffects(t *testing.T) {
	p := model.New()
	p.Song.SetSpeed(1)
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1, Effect1: '9', Parameter1: 64}
	p.Song.Patterns[0][1] = model.Cell{Note: 69, Instrument: 1}
	e := New(p)
	e.Play(false)
	e.Tick()
	if e.Voices[0].Parameters[17] != 64 {
		t.Fatal("effect did not update pulse width")
	}
	e.Tick()
	if e.Voices[0].Parameters[17] != p.Bank.Instruments[0][33] {
		t.Fatal("same-instrument retrigger retained an effect-modified parameter")
	}
}

func TestBothPCMSampleVoicesCanBePreviewedWithoutSongPlayback(t *testing.T) {
	p := model.New()
	p.Bank.Samples[0].PCM = make([]byte, 2000)
	for i := range p.Bank.Samples[0].PCM {
		p.Bank.Samples[0].PCM[i] = 64
	}
	e := New(p)
	e.TriggerSample(1, 60, 1)
	s := NewSynth(e, 48000)
	pcm := make([]byte, 4000)
	if _, err := s.Read(pcm); err != nil {
		t.Fatal(err)
	}
	if e.Playing || !s.pcm[1].active || binary.LittleEndian.Uint16(pcm[:2]) == 0 {
		t.Fatal("second PCM preview was silent or required song transport")
	}
	e.Stop()
	s.Read(pcm)
	if s.pcm[1].active {
		t.Fatal("stop did not silence a previewed PCM voice")
	}
}

func TestExtraArpeggioAddsToTheInstrumentSequenceAndStartsAtBase(t *testing.T) {
	p := model.New()
	p.Bank.Instruments[0][49] = 3
	p.Bank.Sequences[3] = model.Sequence{Length: 1, Values: [63]uint16{2}}
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 'X', Parameter1: 0x47}
	e := New(p)
	e.Play(false)
	for tick, expected := range []uint16{2, 6, 9, 2, 6, 9} {
		e.Tick()
		if e.Voices[0].Values[1] != expected {
			t.Fatalf("tick %d: arpeggio=%d, want %d", tick, e.Voices[0].Values[1], expected)
		}
	}
	p.Song.Patterns[0][0].Parameter1 = 0xfc
	e.Reset()
	e.Play(false)
	for tick, expected := range []uint16{2, 14, 2, 14} {
		e.Tick()
		if e.Voices[0].Values[1] != expected {
			t.Fatalf("two-step tick %d: arpeggio=%d, want %d", tick, e.Voices[0].Values[1], expected)
		}
	}
}

func TestDigiDrumUsesNativeDACLevelsAndAttenuationWithoutTriggerAllocations(t *testing.T) {
	p := model.New()
	p.Bank.Samples[0].PCM = []byte{128, 0, 83, 255}
	p.Bank.Instruments[0][36] = 1
	p.Bank.Sequences[2].Values[0] = 13
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	e := New(p)
	e.Play(false)
	s := NewSynth(e, 48000)
	e.Tick()
	s.configure()
	timer := &s.timers[0]
	for i, expected := range []int{0, 13, 15, 13} {
		timer.digiPosition = float64(i)
		s.runTimer(0)
		if got := int(s.Chip.ReadRegister(8)); got != expected {
			t.Fatalf("sample %d: DAC level=%d, want %d", i, got, expected)
		}
	}
	timer.digiPosition, timer.digiVolume = 2, 10
	s.runTimer(0)
	if s.Chip.ReadRegister(8) != 10 {
		t.Fatal("native drum attenuation was not applied")
	}
	timer.digiPosition = float64(len(p.Bank.Samples[0].PCM))
	s.runTimer(0)
	if timer.digi || s.Chip.ReadRegister(8) != 8 {
		t.Fatal("sample ending did not stop at the attenuated central DAC level")
	}
	if allocations := testing.AllocsPerRun(20, func() {
		e.Voices[0].Triggered = true
		s.configure()
	}); allocations != 0 {
		t.Fatalf("drum retriggers allocate in the audio callback: %f", allocations)
	}
}

func TestFixedPeriodsStillReceiveComponentMaskedVibrato(t *testing.T) {
	e := New(model.New())
	v := Voice{Note: 69}
	v.Parameters[4], v.Parameters[2] = 7, 7
	v.Values[5], v.Values[2] = 1000, 16
	for component, expected := range []uint16{984, 999, 984} {
		if got := e.Period(&v, component); got != expected {
			t.Fatalf("fixed component %d=%d, want %d", component, got, expected)
		}
	}
	v.Parameters[5], v.Parameters[23], v.Parameters[24] = 7, 1, 244
	if got := e.Period(&v, 2); got != 484 {
		t.Fatalf("scalar fixed frequency bypassed vibrato: %d", got)
	}
	v.Values[5], v.Parameters[5] = 65535, 0
	if got := e.Period(&v, 2); got != 268 {
		t.Fatalf("FFFF fixed sentinel did not restore pitched frequency: %d", got)
	}
}

func TestBuzzerResolutionAppliesToSquareAndTimerWithCoarseDetune(t *testing.T) {
	e := New(model.New())
	v := Voice{Note: 69}
	v.Parameters[25] = 1
	for _, component := range []int{0, 2} {
		if got := e.Period(&v, component); got != envelopePeriods[69]*16 {
			t.Fatalf("component %d did not use buzzer resolution: %d", component, got)
		}
	}
	v.Parameters[5], v.Parameters[23] = 5, 12
	expected := uint16(int(envelopePeriods[69]) * 16 * int(tuningScale[24]) >> 8)
	if got := e.Period(&v, 0); got != expected {
		t.Fatalf("quantized coarse detune used an unrelated pitch table: %d want %d", got, expected)
	}
}
