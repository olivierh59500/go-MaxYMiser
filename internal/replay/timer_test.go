package replay

import (
	"math"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func timerFixture(kind byte, words []uint16) *Synth {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Bank.Sequences[2].Values[0] = 0x100 | uint16(kind)
	if kind == 11 || kind == 12 || kind == 14 {
		p.Bank.Sequences[2].Values[0] |= 0xa0
	}
	p.Bank.Instruments[0][54] = 3
	p.Bank.Sequences[3].Length = byte(len(words))
	copy(p.Bank.Sequences[3].Values[:], words)
	e := New(p)
	e.Play(false)
	e.Tick()
	return NewSynth(e, 48000)
}

func TestSIDLevelsAndFrequencyMatchIsolatedNativeFixture(t *testing.T) {
	s := timerFixture(5, []uint16{0, 5, 15, 5})
	s.configure()
	// Native MFP trace: Timer A divider 2 (/10), data EB (235).
	if got, want := s.timers[0].frequency, 2457600.0/(10*235); math.Abs(got-want) > 1e-9 {
		t.Fatalf("SID MFP cadence %f, want %f; voice=%+v regs=%v mask=%d", got, want, s.Engine.Voices[0], s.Engine.Registers, s.Engine.TimerMask)
	}
	for _, expected := range []int{15, 10, 0, 10, 15} {
		s.timers[0].phase = 1
		s.runTimer(0)
		if got := int(s.Chip.ReadRegister(8)); got != expected {
			t.Fatalf("SID level %d, want native level %d", got, expected)
		}
	}
	if timerFrequency(478, 5, 1) == timerFrequency(478, 5, 2) {
		t.Fatal("one-step SID sequence was coerced into two steps")
	}
}

func TestPWMTwoDurationsAndLevelsMatchIsolatedNativeFixture(t *testing.T) {
	s := timerFixture(9, []uint16{0, 12})
	s.Engine.Voices[0].Parameters[17] = 100
	s.configure()
	// Native MFP trace: first interval divider 4 (/50), data 81 (129);
	// second divider 3 (/16), data B1 (177). Each interrupt changes interval.
	want := [2]float64{48000 * 50 * 129 / 2457600.0, 48000 * 16 * 177 / 2457600.0}
	for i, duration := range s.timers[0].pwmDurations {
		if math.Abs(duration-want[i]) > 1e-9 {
			t.Fatalf("PWM interval %d=%f, want native %f", i, duration, want[i])
		}
	}
	for _, expected := range []int{15, 3, 15, 3} {
		s.timers[0].pwmRemaining = 1
		s.runTimer(0)
		if got := int(s.Chip.ReadRegister(8)); got != expected {
			t.Fatalf("PWM level %d, want native level %d", got, expected)
		}
	}
}

func TestSyncBuzzerUsesTheNativeLowByteWaveforms(t *testing.T) {
	s := timerFixture(11, []uint16{0xa, 0xe, 0x8})
	s.configure()
	if got, want := s.timers[0].frequency, 2457600.0/(50*94); math.Abs(got-want) > 1e-9 {
		t.Fatalf("SyncBuzzer MFP cadence %f, want %f", got, want)
	}
	for _, expected := range []int{10, 14, 8, 10} {
		s.timers[0].phase = 1
		s.runTimer(0)
		if got := int(s.Chip.ReadRegister(13)); got != expected {
			t.Fatalf("SyncBuzzer shape %d, want %d", got, expected)
		}
	}
}

func TestFMPeriodStepsAndSyncBuzzerWordsMatchNativeFixtures(t *testing.T) {
	for _, kind := range []byte{1, 15} {
		s := timerFixture(kind, []uint16{0, 12, 7})
		s.configure()
		for step, expected := range []int{478, 239, 319} {
			s.applyTimerValue(0, []uint16{0, 12, 7}[step])
			if got := int(s.Chip.ReadRegister(0)) | int(s.Chip.ReadRegister(1))<<8; got != expected {
				t.Fatalf("kind %X step %d period %d, want native %d", kind, step, got, expected)
			}
		}
	}
	s := timerFixture(12, []uint16{0x000a, 0x0c0e, 0x0708})
	s.configure()
	for i, expected := range []int{30, 15, 20} {
		s.applyTimerValue(0, []uint16{0x000a, 0x0c0e, 0x0708}[i])
		period := int(s.Chip.ReadRegister(11)) | int(s.Chip.ReadRegister(12))<<8
		if period != expected || int(s.Chip.ReadRegister(13)) != []int{10, 14, 8}[i] {
			t.Fatalf("SyncBuzzer FM step %d: period=%d shape=%d", i, period, s.Chip.ReadRegister(13))
		}
	}
}

func TestTimerOwnedRegistersAreNotOverwrittenByMainReplayTicks(t *testing.T) {
	s := timerFixture(15, []uint16{0, 12, 7})
	s.configure()
	s.applyTimerValue(0, 12)
	s.Engine.Voices[0].Triggered = false
	s.configure()
	if got := int(s.Chip.ReadRegister(0)) | int(s.Chip.ReadRegister(1))<<8; got != 239 {
		t.Fatalf("FM step was overwritten by the unmodulated period: %d", got)
	}
	s = timerFixture(5, []uint16{0, 5, 15, 5})
	s.configure()
	s.applyTimerValue(0, 5)
	s.Engine.Voices[0].Triggered = false
	s.configure()
	if s.Chip.ReadRegister(8) != 10 {
		t.Fatal("SID output was overwritten by the base volume between interrupts")
	}
}
