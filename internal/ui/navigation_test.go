package ui

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestRepeatedJamNavigationUsesTheQueuedTarget(t *testing.T) {
	app, err := New(model.Demo(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.action("jam")
	app.action("play")
	app.synth.Edit(func(e *replay.Engine) {
		for range 17 {
			e.Tick()
		}
	})
	app.moveSongPosition(1)
	app.moveSongPosition(1)
	e, _ := app.synth.Snapshot()
	if e.Position != 0 || e.NextPosition != 2 || !e.PositionQueued {
		t.Fatal("repeated navigation did not adjust the queued position")
	}
	old := app.pattern
	app.moveLivePattern(1)
	if app.pattern != old {
		t.Fatal("Jam song mode changed its pattern selection")
	}
	app.action("pattern")
	app.moveLivePattern(1)
	app.moveLivePattern(-1)
	e, _ = app.synth.Snapshot()
	p, ok := e.QueuedPattern(0)
	if !ok || p != e.Patterns[0] {
		t.Fatal("a queued pattern zero was mistaken for no selection")
	}
}
