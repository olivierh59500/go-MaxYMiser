package sndh

import "testing"

func programDMABuffer(d *dma, start, end uint32, mode, control byte) {
	d.write8(3, byte(start>>16))
	d.write8(5, byte(start>>8))
	d.write8(7, byte(start))
	d.write8(0x0f, byte(end>>16))
	d.write8(0x11, byte(end>>8))
	d.write8(0x13, byte(end))
	d.write8(0x21, mode)
	d.write8(1, control)
}

func TestDMAMonoStopAndBufferEndInterrupt(t *testing.T) {
	var d dma
	var m mfp
	d.reset(6258)
	m.reset(6258)
	m.write8(0x07, 0x80)
	m.write8(0x13, 0x80)
	programDMABuffer(&d, 2, 6, 0x80, 1)
	ram := []byte{0, 0, 10, 236, 30, 216}
	for i, want := range []int{640, -1280, 1920, -2560} {
		if got := d.sample(ram, &m); got != want {
			t.Fatalf("sample %d = %d, want %d", i, got, want)
		}
		if m.tick()[mfpGPIP7] {
			t.Fatal("buffer ended before its last sample")
		}
	}
	if d.read8(0x0d) != 6 {
		t.Fatal("DMA current pointer did not advance")
	}
	if d.sample(ram, &m) != 0 || d.read8(1)&1 != 0 || !m.tick()[mfpGPIP7] {
		t.Fatal("one-shot DMA did not stop and signal its buffer end")
	}
}

func TestDMAStereoLoopReloadsNewBuffer(t *testing.T) {
	var d dma
	d.reset(6258)
	programDMABuffer(&d, 0, 2, 0, 3)
	ram := []byte{10, 20, 30, 40}
	if got := d.sample(ram, nil); got != 30*64 {
		t.Fatalf("stereo sample = %d", got)
	}
	// STE software can change the next buffer while the current one plays.
	d.write8(7, 2)
	d.write8(0x13, 4)
	if got := d.sample(ram, nil); got != 70*64 {
		t.Fatalf("looped buffer sample = %d, want %d", got, 70*64)
	}
	if d.read8(1)&1 == 0 {
		t.Fatal("loop mode stopped replay")
	}
}

func TestDMA50kPairsAllFourVoiceBytes(t *testing.T) {
	var d dma
	d.reset(50066)
	programDMABuffer(&d, 0, 4, 3, 3)
	ram := []byte{10, 20, 30, 40}
	if got := d.sample(ram, nil); got != 0 {
		t.Fatalf("half pair emitted %d", got)
	}
	if got := d.sample(ram, nil); got != (10+20+30+40)*32 {
		t.Fatalf("50 kHz pair = %d, want %d", got, (10+20+30+40)*32)
	}
}

func TestDMAMicrowireVolumeAndBusyLoop(t *testing.T) {
	var d dma
	d.reset(6258)
	d.write16(0x24, 0x07ff)
	// Device 2, register 3, master volume 20/40.
	d.write16(0x22, 2<<9|3<<6|20)
	if d.volume != 32 {
		t.Fatalf("microwire volume = %d, want 32", d.volume)
	}
	for i := 1; i < 16; i++ {
		if d.read16(0x24) == 0x07ff {
			t.Fatalf("microwire returned to its mask after only %d reads", i)
		}
	}
	if d.read16(0x24) != 0x07ff || d.read16(0x24) != 0x07ff {
		t.Fatal("microwire busy loop did not complete after sixteen reads")
	}
	programDMABuffer(&d, 0, 2, 0x80, 3)
	if got := d.sample([]byte{64, 64}, nil); got != 64*32 {
		t.Fatalf("volume-adjusted sample = %d", got)
	}
}

func TestDMAOutOfRAMAndBusWidths(t *testing.T) {
	var d dma
	d.reset(6258)
	programDMABuffer(&d, 0x100, 0x102, 0x80, 3)
	if got := d.sample([]byte{1}, nil); got != 0 {
		t.Fatalf("out-of-RAM fetch = %d", got)
	}
	d.write16(0x20, 0x1280)
	if got := d.read16(0x20); got != 0xff80 {
		t.Fatalf("DMA word read = %#x", got)
	}
	if d.read8(0x20) != 0xff || d.read16(0x21) != 0xffff {
		t.Fatal("DMA disconnected byte lane did not read as ones")
	}
}
