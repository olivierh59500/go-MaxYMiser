package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestJamPositionSelectionWaitsForExactlyTheNextPatternBoundary(t *testing.T) {
	p := model.Demo()
	p.Song.SetSpeed(2)
	e := New(p)
	e.Jam = true
	e.Play(false)
	for range 17 {
		e.Tick()
	}
	before := *e
	if !e.SelectPosition(2) || !e.PositionQueued || e.NextPosition != 2 || e.Position != before.Position || e.Row != before.Row || e.TickInRow != before.TickInRow || e.Voices != before.Voices || e.Registers != before.Registers {
		t.Fatal("queued song position interrupted the active pattern")
	}
	e.SelectPosition(1)
	for e.Row != 63 || e.TickInRow != 1 {
		e.Tick()
	}
	if e.Position != 0 {
		t.Fatal("queued position applied before the last pattern tick")
	}
	e.Tick()
	if e.Position != 1 || e.PositionQueued || e.Row != 0 || e.Patterns != p.Song.Orders[1] {
		t.Fatal("latest position request did not apply at the boundary")
	}
}

func TestImmediateSelectionAndStoppingDiscardPendingNavigation(t *testing.T) {
	p := model.Demo()
	e := New(p)
	e.Play(false)
	for range 17 {
		e.Tick()
	}
	e.SelectPosition(1)
	if e.Position != 1 || e.Row != 0 || e.TickInRow != 0 || e.PositionQueued {
		t.Fatal("ordinary position selection was not immediate")
	}
	e.Jam = true
	e.SelectPosition(2)
	e.QueuePattern(0, 3)
	e.Stop()
	e.Play(false)
	if e.PositionQueued {
		t.Fatal("stop retained a pending song jump")
	}
	if _, ok := e.QueuedPattern(0); ok {
		t.Fatal("stop retained a queued live pattern")
	}
	if e.SelectPosition(255) {
		t.Fatal("invalid song position accepted")
	}
}
