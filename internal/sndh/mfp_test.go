package sndh

import "testing"

func TestMFPCounterZeroMeans256(t *testing.T) {
	var m mfp
	m.reset(mfpClock / 4)
	m.write8(0x1f, 0)
	m.write8(0x19, 1)
	m.write8(0x07, 0x20)
	m.write8(0x13, 0x20)
	for i := 1; i < 256; i++ {
		if m.tick()[mfpTimerA] {
			t.Fatalf("zero data interrupted after %d counts, want 256", i)
		}
	}
	if !m.tick()[mfpTimerA] || m.read8(0x1f) != 0 {
		t.Fatal("timer did not interrupt and reload after 256 counts")
	}
}

func TestMFPMaskedCounterKeepsCounting(t *testing.T) {
	var m mfp
	m.reset(mfpClock / 4)
	m.write8(0x1f, 2)
	m.write8(0x19, 1)
	m.write8(0x07, 0x20)
	if m.tick()[mfpTimerA] || m.read8(0x1f) != 1 {
		t.Fatal("timer must count while masked")
	}
	m.write8(0x13, 0x20)
	if !m.tick()[mfpTimerA] {
		t.Fatal("unmasking reset the running counter")
	}
	m.write8(0x07, 0)
	m.tick()
	if m.read8(0x1f) != 2 {
		t.Fatal("disabled timer continued counting")
	}
}

func TestMFPDMAEvents(t *testing.T) {
	var m mfp
	m.reset(44100)
	m.write8(0x1f, 2)
	m.write8(0x19, 8)
	m.write8(0x07, 0xa0)
	m.write8(0x13, 0xa0)
	m.steEvent()
	first := m.tick()
	if first[mfpTimerA] || !first[mfpGPIP7] || m.read8(0x1f) != 1 {
		t.Fatalf("first DMA event = %v, counter = %d", first, m.read8(0x1f))
	}
	if got := m.tick(); got != [5]bool{} {
		t.Fatalf("DMA event repeated without another buffer end: %v", got)
	}
	m.steEvent()
	second := m.tick()
	if !second[mfpTimerA] || !second[mfpGPIP7] {
		t.Fatalf("second DMA event = %v", second)
	}
}

func TestMFPTOSCDefaultAndBusWidths(t *testing.T) {
	var m mfp
	m.reset(mfpClock / 4)
	m.write16(0x22, 0xab01)
	m.write8(0x1d, 0x10)
	if !m.tick()[mfpTimerC] {
		t.Fatal("TOS timer C default enable and mask were lost")
	}
	if got := m.read16(0x22); got != 0xff01 {
		t.Fatalf("MFP word read = %#x, want %#x", got, 0xff01)
	}
	m.write8(0, 0)
	if m.read8(0) != 0xff || m.read8(1) != 0x80 {
		t.Fatal("MFP even byte or GPIP7 pull-up is wrong")
	}
}

func TestMFPResetClearsReloadAndPendingEvent(t *testing.T) {
	var m mfp
	m.reset(44100)
	m.write8(0x1f, 42)
	m.steEvent()
	m.reset(44100)
	if m.timers[mfpTimerA].reload != 0 || m.timers[mfpTimerA].externalEvent {
		t.Fatal("reset retained timer state from previous song")
	}
}

func TestMFPActiveReloadProgressesDuringCPUExecution(t *testing.T) {
	var m mfp
	m.reset(mfpClock / 4)
	m.write8(0x1f, 3)
	m.write8(0x19, 1)
	m.write8(0x07, 0x20)
	m.write8(0x13, 0x20)
	m.write8(0x1f, 9)
	if m.read8(0x1f) != 3 {
		t.Fatal("writing an active timer replaced its current count")
	}
	// Three timer counts take about 39 CPU cycles. A routine can now poll
	// TADR until the reload applies without waiting for an output sample.
	m.advanceCycles(40)
	if got := m.read8(0x1f); got != 9 {
		t.Fatalf("CPU execution did not apply the active reload: %d", got)
	}
	for i := 0; i < 3; i++ {
		interrupt := m.tick()[mfpTimerA]
		if interrupt != (i == 0) {
			t.Fatalf("CPU overflow interrupt on sample %d = %v", i, interrupt)
		}
		if got := m.read8(0x1f); got != 9 {
			t.Fatalf("sample %d counted CPU time twice: %d", i, got)
		}
	}
	m.tick()
	if got := m.read8(0x1f); got != 8 {
		t.Fatalf("counter did not resume after consuming CPU credit: %d", got)
	}
}

func TestMFPDMAEventsIgnoreCPUTimeCredit(t *testing.T) {
	var m mfp
	m.reset(44100)
	m.write8(0x1f, 1)
	m.write8(0x19, 8)
	m.write8(0x07, 0xa0)
	m.write8(0x13, 0xa0)
	m.advanceCycles(10000)
	m.steEvent()
	got := m.tick()
	if !got[mfpTimerA] || !got[mfpGPIP7] {
		t.Fatalf("CPU credit suppressed DMA buffer-end events: %v", got)
	}
	if got := m.tick(); got != [5]bool{} {
		t.Fatalf("DMA event was delivered twice: %v", got)
	}
}

func TestMFPResetCycleCreditRetainsInitializedTimer(t *testing.T) {
	var m mfp
	m.reset(mfpClock / 4)
	m.write8(0x1f, 9)
	m.write8(0x19, 1)
	m.write8(0x07, 0x20)
	m.advanceCycles(40)
	data, phase := m.timers[mfpTimerA].data, m.timers[mfpTimerA].clock
	m.resetCycleCredit()
	if m.timers[mfpTimerA].data != data || m.timers[mfpTimerA].clock != phase {
		t.Fatal("clearing startup credit changed the initialized counter")
	}
	m.tick()
	if got := m.read8(0x1f); got != data-1 {
		t.Fatalf("startup credit still delayed playback: %d", got)
	}
}
