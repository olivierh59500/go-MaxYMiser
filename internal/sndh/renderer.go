package sndh

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Renderer executes an SNDH player's real 68000 routines and hardware writes.
// It owns its machine and must be serialized by its caller, like an io.Reader.
type Renderer struct {
	machine                   *machine
	file                      *File
	subtune, sampleRate, rate int
	position                  uint64
	tickPhase                 int64
	scratch                   [1]int16
	err                       error
	closed                    bool
}

func NewRenderer(file *File, subtune, sampleRate int) (*Renderer, error) {
	if file == nil || len(file.Data) < 16 {
		return nil, fmt.Errorf("sndh: no executable")
	}
	if subtune < 1 || subtune > file.Metadata.Subtunes {
		return nil, fmt.Errorf("sndh: subtune %d outside 1–%d", subtune, file.Metadata.Subtunes)
	}
	if sampleRate < 8000 || sampleRate > 192000 {
		return nil, fmt.Errorf("sndh: sample rate outside 8000–192000 Hz")
	}
	if file.Metadata.Rate < 1 || file.Metadata.Rate > 2000 {
		return nil, fmt.Errorf("sndh: invalid player rate")
	}
	m, err := newMachine(file.Data, sampleRate)
	if err != nil {
		return nil, err
	}
	r := &Renderer{machine: m, file: file, subtune: subtune, sampleRate: sampleRate, rate: file.Metadata.Rate}
	if err := m.call(uploadAddress, uint32(subtune), false, initCycleLimit); err != nil {
		return nil, fmt.Errorf("sndh: init subtune %d: %w", subtune, err)
	}
	m.mfp.resetCycleCredit()
	return r, nil
}

func (r *Renderer) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.EOF
	}
	if r.err != nil {
		return 0, r.err
	}
	frames := len(p) / 4
	for i := 0; i < frames; i++ {
		if r.tickPhase <= 0 {
			if err := r.machine.doSoundTick(); err != nil {
				r.err = err
				return i * 4, err
			}
			if err := r.machine.call(uploadAddress+8, 0, false, playCycleLimit); err != nil {
				r.err = fmt.Errorf("sndh: replay: %w", err)
				return i * 4, r.err
			}
			r.tickPhase += int64(r.sampleRate)
		}
		r.tickPhase -= int64(r.rate)
		r.machine.chip.Update(r.scratch[:], 1)
		value := int(r.scratch[0]) + r.machine.dma.sample(r.machine.ram, &r.machine.mfp)
		value = max(-32768, min(32767, value))
		for timer, active := range r.machine.mfp.tick() {
			if !active {
				continue
			}
			r.machine.insideIRQ = true
			err := r.machine.call(r.machine.Read32(timerVectors[timer]), 0, true, irqCycleLimit)
			r.machine.insideIRQ = false
			if err != nil {
				r.err = fmt.Errorf("sndh: timer %d: %w", timer, err)
				return i * 4, r.err
			}
		}
		binary.LittleEndian.PutUint16(p[i*4:], uint16(int16(value)))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(int16(value)))
		r.position++
	}
	return frames * 4, nil
}

func (r *Renderer) PositionSamples() uint64 { return r.position }
func (r *Renderer) Registers() (registers [14]byte) {
	copy(registers[:], r.machine.registers[:14])
	return
}
func (r *Renderer) HasEffects() bool { return r.machine.effects }

func (r *Renderer) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.err != nil {
		return nil
	}
	return r.machine.call(uploadAddress+4, 0, false, playCycleLimit)
}

var _ io.Reader = (*Renderer)(nil)
