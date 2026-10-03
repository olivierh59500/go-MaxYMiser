package sndh

import (
	"encoding/binary"
	"fmt"

	m68k "github.com/olivierh59500/go-MaxYMiser/internal/sndh/m68k"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

// The machine layout and OS compatibility calls follow AtariAudio 1.25 by
// Arnaud Carré (MIT), pinned at 0e9059cbd6c29a5c073996adc40160b719857a5d.
// The CPU and synthesis execute entirely in Go.
const (
	ramSize        = 4 << 20
	uploadAddress  = 0x10002
	returnAddress  = 0x502
	faultAddress   = 0x504
	initCycleLimit = 80_000_000
	playCycleLimit = 8_000_000
	irqCycleLimit  = 160_256
)

var timerVectors = [5]uint32{0x134, 0x120, 0x114, 0x110, 0x13c}

type machine struct {
	ram                        []byte
	cpu                        *m68k.CPU
	chip                       *stsound.CYm2149Ex
	mfp                        mfp
	dma                        dma
	selected                   byte
	registers                  [16]byte
	effects                    bool
	insideIRQ                  bool
	nextMalloc                 uint32
	doSound                    uint32
	doSoundValue, doSoundDelay byte
	returned                   bool
}

func newMachine(data []byte, sampleRate int) (*machine, error) {
	if len(data) == 0 || len(data) > ramSize-uploadAddress-0x100000 {
		return nil, fmt.Errorf("sndh: executable does not fit emulated RAM")
	}
	m := &machine{ram: make([]byte, ramSize), nextMalloc: ramSize - 0x100000}
	copy(m.ram[uploadAddress:], data)
	m.chip = stsound.NewYm2149Ex(2000000, 1, stsound.YmU32(sampleRate))
	m.chip.SetFilter(true)
	m.mfp.reset(sampleRate)
	m.dma.reset(sampleRate)
	m.registers[7] = 0x3f
	m.chip.WriteRegister(7, 0x3f)
	for vector := uint32(2); vector < 256; vector++ {
		m.Write32(vector*4, 0x500)
	}
	for vector := uint32(2); vector <= 11; vector++ {
		m.Write32(vector*4, faultAddress)
	}
	m.Write32(5*4, 0x500) // Older players intentionally divide by zero.
	m.Write16(0x500, 0x4e73)
	m.Write16(returnAddress, 0x4e70)
	m.Write16(faultAddress, 0x4afc)
	m.Write32(0x900, 0x5f534e44)
	m.Write32(0x904, 3)
	m.Write32(0x908, 0x5f4d4348)
	m.Write32(0x90c, 0x00010000)
	m.Write32(0x910, 0)
	m.Write32(0x5a0, 0x900)
	m.Write32(0, ramSize-4)
	m.Write32(4, uploadAddress)
	// AtariAudio permits unaligned data transfers. A few archived DMA drivers
	// depend on this compatibility behavior; instruction fetch remains strict.
	m.cpu = m68k.NewWithConfig(m, m68k.Config{AllowUnalignedData: true})
	return m, nil
}

func (m *machine) Read8(address uint32) byte {
	address &= 0xffffff
	if address < ramSize {
		return m.ram[address]
	}
	switch {
	case address >= 0xff8800 && address < 0xff8900:
		if address&2 == 0 && m.selected < 16 {
			return m.registers[m.selected]
		}
	case address >= 0xfffa00 && address < 0xfffa26:
		return m.mfp.read8(address - 0xfffa00)
	case address >= 0xff8900 && address < 0xff8926:
		return m.dma.read8(address - 0xff8900)
	case address == 0xff8260:
		return 0
	case address == 0xff820a:
		return 2
	}
	return 0xff
}

func (m *machine) Read16(address uint32) uint16 {
	address &= 0xffffff
	if address < ramSize-1 {
		return binary.BigEndian.Uint16(m.ram[address:])
	}
	if address >= 0xff8900 && address < 0xff8926 {
		return m.dma.read16(address - 0xff8900)
	}
	if address >= 0xfffa00 && address < 0xfffa26 {
		return m.mfp.read16(address - 0xfffa00)
	}
	if address >= 0xff8800 && address < 0xff8900 {
		return uint16(m.Read8(address)) << 8
	}
	return uint16(m.Read8(address))<<8 | uint16(m.Read8(address+1))
}

func (m *machine) Read32(address uint32) uint32 {
	return uint32(m.Read16(address))<<16 | uint32(m.Read16(address+2))
}

func (m *machine) Write8(address uint32, value byte) {
	address &= 0xffffff
	if address < ramSize {
		m.ram[address] = value
		return
	}
	switch {
	case address >= 0xff8800 && address < 0xff8900:
		if address&2 == 0 {
			m.selected = value
		} else {
			m.writeYM(m.selected, value)
		}
	case address >= 0xfffa00 && address < 0xfffa26:
		m.mfp.write8(address-0xfffa00, value)
	case address >= 0xff8900 && address < 0xff8926:
		if address == 0xff8901 && value&1 != 0 {
			m.effects = true
		}
		m.dma.write8(address-0xff8900, value)
	}
}

func (m *machine) Write16(address uint32, value uint16) {
	address &= 0xffffff
	if address < ramSize-1 {
		binary.BigEndian.PutUint16(m.ram[address:], value)
		return
	}
	switch {
	case address >= 0xff8800 && address < 0xff8900:
		m.Write8(address, byte(value>>8))
	case address >= 0xfffa00 && address < 0xfffa26:
		m.mfp.write16(address-0xfffa00, value)
	case address >= 0xff8900 && address < 0xff8926:
		if address == 0xff8900 && value&1 != 0 {
			m.effects = true
		}
		m.dma.write16(address-0xff8900, value)
	default:
		m.Write8(address, byte(value>>8))
		m.Write8(address+1, byte(value))
	}
}

func (m *machine) Write32(address uint32, value uint32) {
	m.Write16(address, uint16(value>>16))
	m.Write16(address+2, uint16(value))
}

func (m *machine) Reset() { m.returned = true }

func (m *machine) writeYM(register, value byte) {
	if register >= 16 {
		return
	}
	m.registers[register] = value
	if register < 14 {
		if m.insideIRQ {
			m.effects = true
		}
		m.chip.WriteRegister(stsound.YmInt(register), stsound.YmInt(value))
		m.registers[register] = byte(m.chip.ReadRegister(stsound.YmInt(register)))
	}
}

func (m *machine) call(address, d0 uint32, interrupt bool, cycleLimit uint64) error {
	if address&1 != 0 || address >= ramSize {
		return fmt.Errorf("sndh: invalid routine address $%06x", address)
	}
	regs := m.cpu.Registers()
	regs.PC, regs.SR, regs.SSP = address, 0x2700, ramSize-4
	if !interrupt {
		regs.D[0] = d0
	}
	m.Write32(ramSize-4, returnAddress)
	if interrupt {
		regs.SSP -= 2
		m.Write16(regs.SSP, 0x2300)
	}
	m.cpu.SetState(regs)
	m.returned = false
	var cycles uint64
	for instructions := uint64(0); instructions < cycleLimit/4+1; instructions++ {
		regs = m.cpu.Registers()
		if regs.PC == returnAddress || m.returned {
			return nil
		}
		if regs.PC == faultAddress || regs.PC >= ramSize || regs.PC&1 != 0 || m.cpu.Halted() {
			return fmt.Errorf("sndh: CPU fault at $%06x after %d instructions", regs.PC, instructions)
		}
		opcode := m.Read16(regs.PC)
		if opcode == 0x4e41 || opcode == 0x4e4e || opcode == 0x4e4d {
			if err := m.trap(opcode&15, regs); err != nil {
				return err
			}
			cycles += 34
			m.mfp.advanceCycles(34)
		} else {
			cost := m.cpu.Step()
			if cost <= 0 || m.cpu.Halted() {
				return fmt.Errorf("sndh: CPU halted at $%06x opcode $%04x", regs.PC, opcode)
			}
			cycles += uint64(cost)
			m.mfp.advanceCycles(cost)
		}
		if cycles > cycleLimit {
			return fmt.Errorf("sndh: execution limit at $%06x opcode $%04x (%d cycles)", regs.PC, opcode, cycles)
		}
	}
	return fmt.Errorf("sndh: instruction limit at $%06x", m.cpu.Registers().PC)
}

func (m *machine) trap(number uint16, regs m68k.Registers) error {
	stack := regs.A[7]
	if stack > ramSize-2 {
		return fmt.Errorf("sndh: OS call stack outside RAM")
	}
	function := m.Read16(stack)
	argumentBytes := uint32(2)
	if number == 1 && function == 0x48 || number == 14 && (function == 32 || function == 38) {
		argumentBytes = 6
	}
	if number == 14 && function == 31 {
		argumentBytes = 12
	}
	if number == 13 && function == 5 {
		argumentBytes = 8
	}
	if argumentBytes > uint32(ramSize)-stack {
		return fmt.Errorf("sndh: OS call arguments outside RAM")
	}
	regs.PC += 2
	if regs.SR&0x2000 != 0 {
		regs.SSP = stack
	} else {
		regs.USP = stack
	}
	switch number {
	case 1:
		switch function {
		case 0x48:
			size := m.Read32(stack + 2)
			if size > uint32(ramSize)-m.nextMalloc {
				return fmt.Errorf("sndh: emulated GEMDOS allocation exceeds RAM")
			}
			regs.D[0] = m.nextMalloc
			m.nextMalloc = (m.nextMalloc + size + 1) &^ 1
		case 0x30, 0x20, 0x49, 0x4a:
			regs.D[0] = 0
		default:
			return fmt.Errorf("sndh: unsupported GEMDOS function $%x", function)
		}
	case 14:
		switch function {
		case 31:
			timer := m.Read16(stack + 2)
			if timer > 3 {
				return fmt.Errorf("sndh: invalid Xbtimer number %d", timer)
			}
			control, data := byte(m.Read16(stack+4)), byte(m.Read16(stack+6))
			m.Write32(timerVectors[timer], m.Read32(stack+8))
			ctrl, dr, er, bit, preserve := uint32(0x19), uint32(0x1f), uint32(7), byte(5), byte(0)
			switch timer {
			case 1:
				ctrl, dr, bit = 0x1b, 0x21, 0
			case 2:
				ctrl, dr, er, preserve, control = 0x1d, 0x23, 9, 15, control<<4
			case 3:
				ctrl, dr, er, bit, preserve = 0x1d, 0x25, 9, 4, 0xf0
			}
			back := m.mfp.read8(ctrl) & preserve
			m.mfp.write8(ctrl, back)
			m.mfp.write8(dr, data)
			m.mfp.write8(ctrl, back|control)
			m.mfp.write8(er, m.mfp.read8(er)|(1<<bit))
			m.mfp.write8(er+12, m.mfp.read8(er+12)|(1<<bit))
		case 32:
			m.doSound = m.Read32(stack + 2)
		case 38:
			callback := m.Read32(stack + 2)
			if stack < 4 {
				return fmt.Errorf("sndh: Supexec stack underflow")
			}
			stack -= 4
			m.Write32(stack, regs.PC)
			regs.PC = callback
			regs.SSP = stack
			regs.SR |= 0x2000
		case 2, 3:
			regs.D[0] = 0xe0000
		case 4:
			regs.D[0] = 0
		case 5: // Screen configuration has no effect on audio.
		default:
			return fmt.Errorf("sndh: unsupported XBIOS function %d", function)
		}
	case 13:
		if function != 5 {
			return fmt.Errorf("sndh: unsupported BIOS function %d", function)
		}
		vector := uint32(m.Read16(stack+2)) * 4
		value := m.Read32(stack + 4)
		regs.D[0] = m.Read32(vector)
		if value != 0xffffffff {
			m.Write32(vector, value)
		}
	}
	m.cpu.SetState(regs)
	return nil
}

func (m *machine) doSoundTick() error {
	if m.doSound == 0 {
		return nil
	}
	if m.doSoundDelay != 0 {
		m.doSoundDelay--
		return nil
	}
	for count := 0; count < 1024; count++ {
		if m.doSound > ramSize-4 {
			return fmt.Errorf("sndh: DoSound program outside RAM")
		}
		command := m.Read8(m.doSound)
		switch {
		case command < 0x80:
			m.writeYM(command&15, m.Read8(m.doSound+1))
			m.doSound += 2
		case command == 0x80:
			m.doSoundValue = m.Read8(m.doSound + 1)
			m.doSound += 2
		case command == 0x81:
			m.doSoundValue += m.Read8(m.doSound + 2)
			m.writeYM(m.Read8(m.doSound+1)&15, m.doSoundValue)
			if m.doSoundValue == m.Read8(m.doSound+3) {
				m.doSound += 4
			}
			return nil
		default:
			m.doSoundDelay = m.Read8(m.doSound + 1)
			if m.doSoundDelay == 0 {
				m.doSound = 0
			}
			return nil
		}
	}
	return fmt.Errorf("sndh: DoSound command limit")
}
