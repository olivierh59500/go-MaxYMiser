package sndh

// The STE DAC model is adapted from AtariAudio's SteDac.cpp at
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

var dmaRates = [4]uint32{50066 / 8, 50066 / 4, 50066 / 2, 50066}

// dma models the STE's signed eight-bit PCM playback and the master volume
// command of its microwire interface. Output uses the same signed 16-bit scale
// as the YM mixer.
type dma struct {
	sampleRate    uint32
	regs          [256]byte
	pointer       uint32
	end           uint32
	clock         uint32
	microwireMask uint16
	microwireData uint16
	microwireBits int
	volume        int
	pairPending   bool
	pairLevel     int
	level         int
}

func (d *dma) reset(sampleRate int) {
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	*d = dma{sampleRate: uint32(sampleRate), volume: 64}
}

func (d *dma) fetchPointers() {
	d.pointer = uint32(d.regs[3])<<16 | uint32(d.regs[5])<<8 | uint32(d.regs[7]&0xfe)
	d.end = uint32(d.regs[0x0f])<<16 | uint32(d.regs[0x11])<<8 | uint32(d.regs[0x13]&0xfe)
}

func (d *dma) write8(offset uint32, value byte) {
	port := offset & 255
	if port&1 == 0 {
		return
	}
	switch port {
	case 1:
		if value&1 != 0 && d.regs[1]&1 == 0 {
			d.fetchPointers()
		}
	case 7, 0x0d:
		value &= 0xfe
	case 0x21:
		if value&3 != d.regs[port]&3 {
			d.pairLevel = 0
			d.pairPending = false
		}
	}
	d.regs[port] = value
}

func (d *dma) write16(offset uint32, value uint16) {
	port := offset & 255
	if port&1 != 0 {
		return
	}
	switch port {
	case 0x22:
		d.microwireData = value
		d.microwireCommand()
		d.microwireBits = 16
	case 0x24:
		d.microwireMask = value
	default:
		d.write8(port+1, byte(value))
	}
}

func (d *dma) read8(offset uint32) byte {
	port := offset & 255
	if port&1 == 0 {
		return 0xff
	}
	switch port {
	case 9:
		return byte(d.pointer >> 16)
	case 0x0b:
		return byte(d.pointer >> 8)
	case 0x0d:
		return byte(d.pointer)
	default:
		return d.regs[port]
	}
}

func (d *dma) read16(offset uint32) uint16 {
	port := offset & 255
	if port&1 != 0 {
		return 0xffff
	}
	switch port {
	case 0x22:
		return d.microwireData
	case 0x24:
		if d.microwireBits > 0 {
			d.microwireMask = d.microwireMask<<1 | d.microwireMask>>15
			d.microwireBits--
		}
		return d.microwireMask
	default:
		return 0xff00 | uint16(d.read8(port+1))
	}
}

func (d *dma) sample(ram []byte, mfp *mfp) int {
	if d.regs[1]&1 == 0 {
		d.level = 0
		return 0
	}
	d.clock += dmaRates[d.regs[0x21]&3]
	stereo := d.regs[0x21]&0x80 == 0
	pair := d.regs[0x21]&3 == 3
	for d.clock >= d.sampleRate {
		if d.pointer == d.end {
			if mfp != nil {
				mfp.steEvent()
			}
			d.fetchPointers()
			if d.regs[1]&2 == 0 {
				d.regs[1] &^= 1
				d.level = 0
				break
			}
		}
		level := dmaFetch(ram, d.pointer)
		if stereo {
			level += dmaFetch(ram, d.pointer+1)
		}
		if pair {
			// Averaging consecutive 50 kHz frames preserves all four bytes
			// used by Tao's MS3 and other interleaved voice drivers.
			d.pairLevel += level
			d.pairPending = !d.pairPending
			if !d.pairPending {
				d.level = d.pairLevel * d.volume >> 1
				d.pairLevel = 0
			}
		} else {
			d.level = level * d.volume
		}
		d.pointer++
		if stereo {
			d.pointer++
		}
		d.clock -= d.sampleRate
	}
	return d.level
}

func dmaFetch(ram []byte, address uint32) int {
	if uint64(address) >= uint64(len(ram)) {
		return 0
	}
	return int(int8(ram[address]))
}

func (d *dma) microwireCommand() {
	var value uint16
	bits := 0
	for i := 0; i < 16; i++ {
		if d.microwireMask&(1<<i) != 0 {
			if d.microwireData&(1<<i) != 0 {
				value |= 1 << bits
			}
			bits++
		}
	}
	// An eleven-bit LMC1992 command uses device address 2 and register 3
	// for the master volume, from 0 (mute) through 40 (full volume).
	if bits == 11 && value>>9 == 2 && value>>6&7 == 3 {
		volume := int(value & 0x3f)
		if volume > 40 {
			d.volume = 64
		} else {
			d.volume = volume * 64 / 40
		}
	}
}
