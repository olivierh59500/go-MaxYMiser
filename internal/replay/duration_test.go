package replay

import (
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMeasuredDurationIncludesTempoChangesAndPatternBreaks(t *testing.T) {
	p := model.New()
	p.Song.Length = 2
	p.Song.SetSpeed(6)
	p.Song.Orders[1] = p.Song.Orders[0]
	p.Song.Patterns[0][0] = model.Cell{Effect1: 'S', Parameter1: 2}
	p.Song.Patterns[0][3] = model.Cell{Effect1: 'B'}
	result, err := MeasureSongDuration(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ticks != 16 || result.Duration != 320*time.Millisecond || result.RepeatPosition != 0 {
		t.Fatalf("wrong traversal with speed and break effects: %+v", result)
	}
}

func TestDurationMeasuresJamSectionAndRejectsAnExternalClock(t *testing.T) {
	p := model.New()
	p.Song.Length = 3
	p.Song.State[39] = 255
	p.Song.Orders[2] = [4]byte{253, 255, 255, 255}
	p.Song.Orders[1] = p.Song.Orders[0]
	p.Song.SetSpeed(1)
	result, err := MeasureSongDuration(p)
	if err != nil || result.Ticks != 128 || !result.Jam {
		t.Fatalf("Jam traversal was not bounded at its section: %+v %v", result, err)
	}
	p.Song.State[31] = 1
	if _, err := MeasureSongDuration(p); err == nil {
		t.Fatal("externally timed music was assigned an invented duration")
	}
}
