package replay

import (
	"strings"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestRegisterVerifierRejectsMismatchAndDoesNotAlterProject(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1}
	e := New(p)
	e.Play(false)
	e.Tick()
	ticks := []RegisterTick{{Registers: e.Registers, EnvelopeWrite: e.EnvelopeWrite}}
	before := p.Clone()
	if err := VerifyRegisterTrace(p, ticks); err != nil {
		t.Fatal(err)
	}
	if p.Song.Patterns[0] != before.Song.Patterns[0] || p.Bank.Instruments != before.Bank.Instruments {
		t.Fatal("verification modified the editable project")
	}
	ticks[0].Registers[0]++
	if err := VerifyRegisterTrace(p, ticks); err == nil || !strings.Contains(err.Error(), "tick 0 R0") {
		t.Fatalf("register mismatch was not reported precisely: %v", err)
	}
	ticks[0].Registers[0]--
	ticks[0].EnvelopeWrite = !ticks[0].EnvelopeWrite
	if err := VerifyRegisterTrace(p, ticks); err == nil || !strings.Contains(err.Error(), "envelope write") {
		t.Fatal("envelope retrigger mismatch was ignored")
	}
	if err := VerifyRegisterTrace(p, nil); err == nil {
		t.Fatal("empty evidence was accepted")
	}
}

func TestTraceWriteMaskExcludesRegistersOwnedByTimerInterrupts(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1}
	e := New(p)
	e.Play(false)
	e.Tick()
	mask := [14]bool{}
	mask[0], mask[1], mask[7] = true, true, true
	registers := e.Registers
	registers[8] = 0
	fixture := []RegisterTick{{Registers: registers, EnvelopeWrite: e.EnvelopeWrite, Written: &mask}}
	if err := VerifyRegisterTrace(p, fixture); err != nil {
		t.Fatal("timer-owned register falsely failed main-call comparison:", err)
	}
	fixture[0].Registers[0]++
	if err := VerifyRegisterTrace(p, fixture); err == nil {
		t.Fatal("mask suppressed an actual captured write mismatch")
	}
}
