package ymimport

import (
	"fmt"
	"math"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

// preserveToneCurves keeps readable note columns while retaining register-level
// pitch in editable V sequences. A failed allocation leaves the candidate intact.
// Curves target the Go synthesizer's 2 MHz clock; envelope periods are separate.
func preserveToneCurves(original *model.Project, trace Trace) (*model.Project, error) {
	if trace.Clock == 0 {
		return nil, fmt.Errorf("missing chip clock")
	}
	p := original.Clone()
	rows := make([][3]model.Cell, len(trace.Frames))
	for at := range rows {
		for ch := 0; ch < 3; ch++ {
			rows[at][ch] = p.Song.Patterns[p.Song.Orders[at/64][ch]][at%64]
		}
	}
	ids := map[model.Sequence]byte{}
	for i := 1; i < p.Bank.SequenceCount; i++ {
		ids[p.Bank.Sequences[i]] = byte(i)
	}
	add := func(s model.Sequence) (byte, error) {
		if id, ok := ids[s]; ok {
			return id, nil
		}
		if p.Bank.SequenceCount >= model.MaxSequences {
			return 0, fmt.Errorf("pitch curves exceed native sequence capacity; choose a shorter range")
		}
		id := byte(p.Bank.SequenceCount)
		p.Bank.SequenceCount++
		p.Bank.Sequences[id], ids[s] = s, id
		return id, nil
	}
	for ch := 0; ch < 3; ch++ {
		deltas := make([]uint16, len(rows))
		note := byte(0)
		for at, r := range trace.Frames {
			cell := rows[at][ch]
			if cell.Note != 0 {
				note = cell.Note
			}
			if note < 2 || r[7]&(1<<ch) != 0 || r[8+ch]&31 == 0 {
				continue
			}
			period := int(r[2*ch]) | int(r[2*ch+1]&15)<<8
			period = int(math.Round(float64(period) * 2000000 / float64(trace.Clock)))
			if period > 4095 {
				return nil, fmt.Errorf("pitch curve exceeds the Go chip's period range")
			}
			deltas[at] = uint16(int16(int(replay.TonePeriod(int(note))) - period))
		}
		for at := 0; at < len(rows); {
			end := min(at+63, len(rows))
			for next := at + 1; next < end; next++ {
				if rows[next][ch].Instrument != 0 {
					end = next
					break
				}
			}
			s := model.Sequence{Length: byte(end - at), Repeat: byte(end - at - 1)}
			copy(s.Values[:], deltas[at:end])
			id, err := add(s)
			if err != nil {
				return nil, err
			}
			rows[at][ch].Effect1, rows[at][ch].Parameter1 = 'V', id
			at = end
		}
	}
	// Curves affect square-wave pitch only; the hardware envelope approximation
	// must not inherit its independent tone correction.
	for i := range p.Bank.Instruments {
		p.Bank.Instruments[i][18] = 4
	}
	p.Song.Patterns = nil
	patterns := map[model.Pattern]byte{}
	for pos := 0; pos < int(p.Song.Length); pos++ {
		for ch := 0; ch < 3; ch++ {
			var pattern model.Pattern
			for row := 0; row < 64 && pos*64+row < len(rows); row++ {
				pattern[row] = rows[pos*64+row][ch]
			}
			id, ok := patterns[pattern]
			if !ok {
				if len(p.Song.Patterns) >= model.MaxPatterns {
					return nil, fmt.Errorf("pitch curves exceed native pattern capacity; choose a shorter range")
				}
				id = byte(len(p.Song.Patterns))
				patterns[pattern] = id
				p.Song.Patterns = append(p.Song.Patterns, pattern)
			}
			p.Song.Orders[pos][ch] = id
		}
	}
	return p, nil
}
