package sndh

// The MFP timer model is adapted from AtariAudio's Mk68901.cpp at
// https://github.com/arnaud-carre/AtariAudio/tree/0e9059cbd6c29a5c073996adc40160b719857a5d.
//
// MIT License
// Copyright (c) 2026 Arnaud Carré
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

const (
	mfpClock    = 2457600
	mfpCPUClock = 8012800 // PAL ST: 512 cycles per line, 313 lines, 50 Hz.
)

const (
	mfpTimerA = iota
	mfpTimerB
	mfpTimerC
	mfpTimerD
	mfpGPIP7
)

var mfpPrescale = [8]uint32{0, mfpClock / 4, mfpClock / 10, mfpClock / 16, mfpClock / 50, mfpClock / 64, mfpClock / 100, mfpClock / 200}

// mfp advances the Atari MFP timers during CPU execution and between output
// samples. CPU time is credited against following samples so both clocks cover
// the same elapsed time. Interrupt flags are returned in timer A, B, C, D,
// GPIP7 order; the machine handles their vectors and CPU interrupt priority.
type mfp struct {
	sampleRate  uint32
	regs        [256]byte
	timers      [5]mfpTimer
	cycleCredit uint64
	pending     [5]bool
}

type mfpTimer struct {
	enabled       bool
	maskedIn      bool
	control       byte
	data          byte
	reload        byte
	clock         uint64
	externalEvent bool
}

func (m *mfp) reset(sampleRate int) {
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	*m = mfp{sampleRate: uint32(sampleRate)}
	// TOS enables timer C even before a song programs its counter.
	m.timers[mfpTimerC].enabled = true
	m.timers[mfpTimerC].maskedIn = true
	// GPIP7 receives each DMA buffer-end event, without a programmable count.
	m.timers[mfpGPIP7].control = 8
	m.timers[mfpGPIP7].reload = 1
	m.timers[mfpGPIP7].data = 1
}

func (t *mfpTimer) restart() {
	t.clock = 0
	t.data = t.reload
}

func (t *mfpTimer) setEnabled(enabled bool) {
	if !t.enabled && enabled && t.control&7 != 0 && t.control&8 == 0 {
		t.restart()
	}
	t.enabled = enabled
}

func (m *mfp) read8(offset uint32) byte {
	port := offset & 255
	if port&1 == 0 {
		return 0xff
	}
	switch port {
	case 0x01:
		return m.regs[port] | 0x80
	case 0x1f, 0x21, 0x23, 0x25:
		return m.timers[(port-0x1f)/2].data
	default:
		return m.regs[port]
	}
}

func (m *mfp) read16(offset uint32) uint16 {
	return 0xff00 | uint16(m.read8(offset+1))
}

func (m *mfp) write8(offset uint32, value byte) {
	port := offset & 255
	if port&1 == 0 {
		return
	}
	switch port {
	case 0x19:
		m.timers[mfpTimerA].control = value & 15
	case 0x1b:
		m.timers[mfpTimerB].control = value & 15
	case 0x1d:
		m.timers[mfpTimerC].control = value >> 4 & 7
		m.timers[mfpTimerD].control = value & 7
	case 0x1f, 0x21, 0x23, 0x25:
		timer := &m.timers[(port-0x1f)/2]
		timer.reload = value
		if timer.control == 0 {
			timer.restart()
		}
	case 0x07:
		m.timers[mfpTimerA].setEnabled(value&0x20 != 0)
		m.timers[mfpTimerB].setEnabled(value&0x01 != 0)
		m.timers[mfpGPIP7].setEnabled(value&0x80 != 0)
	case 0x09:
		m.timers[mfpTimerC].setEnabled(value&0x20 != 0)
		m.timers[mfpTimerD].setEnabled(value&0x10 != 0)
	case 0x13:
		m.timers[mfpTimerA].maskedIn = value&0x20 != 0
		m.timers[mfpTimerB].maskedIn = value&0x01 != 0
		m.timers[mfpGPIP7].maskedIn = value&0x80 != 0
	case 0x15:
		m.timers[mfpTimerC].maskedIn = value&0x20 != 0
		m.timers[mfpTimerD].maskedIn = value&0x10 != 0
	}
	m.regs[port] = value
}

func (m *mfp) write16(offset uint32, value uint16) {
	m.write8(offset+1, byte(value))
}

func (m *mfp) steEvent() {
	m.timers[mfpTimerA].externalEvent = true
	m.timers[mfpGPIP7].externalEvent = true
}

// advanceCycles keeps timer data reads live inside player routines, including
// handlers that wait for a newly written reload value to reach the counter.
// Time units are CPU cycles multiplied by sampleRate; one output sample is
// mfpCPUClock units. Timer phases use the common sampleRate*CPUClock divisor.
func (m *mfp) advanceCycles(cycles int) {
	if cycles <= 0 {
		return
	}
	elapsed := uint64(cycles) * uint64(m.sampleRate)
	m.advanceTime(elapsed)
	m.cycleCredit += elapsed
}

// resetCycleCredit starts the output clock after initialization without
// carrying startup CPU time into playback. Counter values, phases and pending
// interrupts retain the state established by the executed init routine.
func (m *mfp) resetCycleCredit() {
	m.cycleCredit = 0
}

func (m *mfp) advanceTime(elapsed uint64) {
	threshold := uint64(m.sampleRate) * mfpCPUClock
	for i := range m.timers {
		t := &m.timers[i]
		if !t.enabled || t.control&8 != 0 || t.control&7 == 0 {
			continue
		}
		t.clock += uint64(mfpPrescale[t.control&7]) * elapsed
		for t.clock >= threshold {
			if t.decrement() && t.maskedIn {
				m.pending[i] = true
			}
			t.clock -= threshold
		}
	}
}

func (m *mfp) tick() (interrupts [5]bool) {
	elapsed := uint64(mfpCPUClock)
	if m.cycleCredit >= elapsed {
		m.cycleCredit -= elapsed
		elapsed = 0
	} else {
		elapsed -= m.cycleCredit
		m.cycleCredit = 0
	}
	m.advanceTime(elapsed)
	interrupts = m.pending
	m.pending = [5]bool{}
	for i := range m.timers {
		t := &m.timers[i]
		if !t.enabled {
			continue
		}
		if t.control&8 != 0 {
			if t.externalEvent {
				interrupts[i] = t.decrement() && t.maskedIn
				t.externalEvent = false
			}
		}
	}
	return interrupts
}

func (t *mfpTimer) decrement() bool {
	// Byte wraparound implements the MFP's zero data value of 256 ticks.
	t.data--
	if t.data == 0 {
		t.data = t.reload
		return true
	}
	return false
}
