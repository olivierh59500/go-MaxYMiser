package replay

import (
	"fmt"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type SongDuration struct {
	Duration                  time.Duration
	Ticks                     int
	RepeatPosition, RepeatRow int
	Jam                       bool
}

// MeasureSongDuration follows real row transitions until the transport first
// revisits a position/row with the same speed. Envelope state is independent:
// this is an arrangement traversal, not proof of a sample-perfect audio loop.
func MeasureSongDuration(project *model.Project) (SongDuration, error) {
	var result SongDuration
	if project == nil || project.Song.Length == 0 {
		return result, fmt.Errorf("replay: no arranged song to measure")
	}
	if project.Song.State[31]&1 != 0 {
		return result, fmt.Errorf("replay: arrangement duration cannot be inferred from an external clock")
	}
	e := New(project.Clone())
	e.Play(false)
	type position struct {
		position, row, speed int
		patterns             [4]byte
	}
	seen := map[position]bool{}
	limit := project.Song.TickRate() * 3600
	for tick := 0; tick < limit; tick++ {
		if e.TickInRow == 0 {
			// Row-zero tempo commands establish the state for this row. Parse
			// them before comparing transport states so the introductory speed
			// is not confused with a different loop through the same section.
			e.parseRow()
			key := position{e.Position, e.Row, e.Speed, e.Patterns}
			if seen[key] {
				result.Ticks = tick
				result.Duration = time.Duration(int64(tick) * int64(time.Second) / int64(project.Song.TickRate()))
				result.RepeatPosition, result.RepeatRow, result.Jam = e.Position, e.Row, e.Jam
				return result, nil
			}
			seen[key] = true
		}
		e.TickInRow++
		if e.TickInRow >= max(1, e.Speed) {
			e.TickInRow = 0
			e.advanceRow()
		}
	}
	return result, fmt.Errorf("replay: arranged traversal did not repeat within one hour")
}
