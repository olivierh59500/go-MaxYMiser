package m68k

import "testing"

func TestUnalignedDataCompatibility(t *testing.T) {
	for _, test := range []struct {
		name    string
		program []uint16
		setup   func(*testBus, *Registers)
		check   func(*testing.T, *testBus, Registers)
	}{
		{
			name:    "word read",
			program: []uint16{0x3010}, // MOVE.W (A0),D0
			setup: func(bus *testBus, regs *Registers) {
				regs.D[0] = 0xaaaa0000
				bus.Write16(0x2001, 0x8123)
			},
			check: func(t *testing.T, bus *testBus, regs Registers) {
				if regs.D[0] != 0xaaaa8123 {
					t.Fatalf("word read D0 = %#x", regs.D[0])
				}
			},
		},
		{
			name:    "word write",
			program: []uint16{0x3080}, // MOVE.W D0,(A0)
			setup: func(bus *testBus, regs *Registers) {
				regs.D[0] = 0x56789abc
			},
			check: func(t *testing.T, bus *testBus, regs Registers) {
				if got := bus.Read16(0x2001); got != 0x9abc {
					t.Fatalf("word write = %#x", got)
				}
			},
		},
		{
			name:    "MOVEM long read",
			program: []uint16{0x4cd8, 0x0003}, // MOVEM.L (A0)+,D0-D1
			setup: func(bus *testBus, regs *Registers) {
				bus.Write32(0x2001, 0x12345678)
				bus.Write32(0x2005, 0xffeeddcc)
			},
			check: func(t *testing.T, bus *testBus, regs Registers) {
				if regs.D[0] != 0x12345678 || regs.D[1] != 0xffeeddcc || regs.A[0] != 0x2009 {
					t.Fatalf("MOVEM read D0=%#x D1=%#x A0=%#x", regs.D[0], regs.D[1], regs.A[0])
				}
			},
		},
		{
			name:    "MOVEM long write",
			program: []uint16{0x48d0, 0x0003}, // MOVEM.L D0-D1,(A0)
			setup: func(bus *testBus, regs *Registers) {
				regs.D[0], regs.D[1] = 0x12345678, 0xffeeddcc
			},
			check: func(t *testing.T, bus *testBus, regs Registers) {
				if bus.Read32(0x2001) != 0x12345678 || bus.Read32(0x2005) != 0xffeeddcc {
					t.Fatalf("MOVEM write = %#x %#x", bus.Read32(0x2001), bus.Read32(0x2005))
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, compatibility := range []bool{false, true} {
				name := "strict"
				if compatibility {
					name = "compatible"
				}
				t.Run(name, func(t *testing.T) {
					bus := &testBus{}
					var cpu *CPU
					if compatibility {
						cpu = NewWithConfig(bus, Config{AllowUnalignedData: true})
					} else {
						cpu = New(bus)
					}
					for i, word := range test.program {
						bus.Write16(0x1000+uint32(i*2), word)
					}
					regs := Registers{PC: 0x1000, SR: 0x2700, SSP: 0x10000}
					regs.A[0] = 0x2001
					test.setup(bus, &regs)
					cpu.SetState(regs)
					cpu.Step()
					if cpu.Halted() == compatibility {
						t.Fatalf("halted = %v, compatibility = %v", cpu.Halted(), compatibility)
					}
					if compatibility {
						test.check(t, bus, cpu.Registers())
					}
				})
			}
		})
	}
}

func TestUnalignedDataConfigSurvivesResetAndSetState(t *testing.T) {
	bus := &testBus{}
	bus.Write32(0, 0x10000)
	bus.Write32(4, 0x1000)
	bus.Write16(0x1000, 0x3039) // MOVE.W ($002001).L,D0
	bus.Write32(0x1002, 0x2001)
	bus.Write16(0x2001, 0x1357)
	cpu := NewWithConfig(bus, Config{AllowUnalignedData: true})
	for _, prepare := range []struct {
		name string
		run  func()
	}{
		{"constructor", func() {}},
		{"reset", cpu.Reset},
		{"set state", func() { cpu.SetState(Registers{PC: 0x1000, SR: 0x2700, SSP: 0x10000}) }},
	} {
		t.Run(prepare.name, func(t *testing.T) {
			prepare.run()
			cpu.Step()
			if cpu.Halted() || cpu.Registers().D[0] != 0x1357 {
				t.Fatal("unaligned compatibility was lost")
			}
		})
	}
}

func TestUnalignedDataConfigStillRejectsOddPC(t *testing.T) {
	for _, compatibility := range []bool{false, true} {
		for _, jump := range []bool{false, true} {
			bus := &testBus{}
			cpu := NewWithConfig(bus, Config{AllowUnalignedData: compatibility})
			pc := uint32(0x1001)
			if jump {
				pc = 0x1000
				bus.Write16(pc, 0x4ef9) // JMP ($001001).L
				bus.Write32(pc+2, 0x1001)
			}
			cpu.SetState(Registers{PC: pc, SR: 0x2700, SSP: 0x10000})
			cpu.Step()
			if !cpu.Halted() {
				t.Fatalf("odd PC accepted: compatibility=%v jump=%v", compatibility, jump)
			}
		}
	}
}
