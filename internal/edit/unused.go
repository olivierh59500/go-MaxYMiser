package edit

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// UnusedPattern finds a blank, unreferenced definition after the selected ID,
// wrapping through the native capacity. Stored material and reserved song rows
// remain protected even when they are outside the current arrangement length.
// Live/queued pattern IDs supplied by the caller are also protected.
func UnusedPattern(p *model.Project, after int, live []byte) (int, error) {
	if p == nil || len(p.Song.Patterns) > model.MaxPatterns {
		return 0, fmt.Errorf("edit: invalid pattern bank")
	}
	var used [model.MaxPatterns]bool
	mark := func(id byte) {
		if int(id) < len(used) {
			used[id] = true
		}
	}
	for _, order := range p.Song.Orders {
		for _, id := range order {
			mark(id)
		}
	}
	for _, base := range []int{4, 52} {
		for _, id := range p.Song.State[base : base+4] {
			mark(id)
		}
	}
	for _, id := range live {
		mark(id)
	}
	for offset := 1; offset <= model.MaxPatterns; offset++ {
		id := (max(-1, min(model.MaxPatterns-1, after)) + offset) % model.MaxPatterns
		if used[id] {
			continue
		}
		if id >= len(p.Song.Patterns) || p.Song.Patterns[id] == (model.Pattern{}) {
			return id, nil
		}
	}
	return 0, fmt.Errorf("edit: no blank unreferenced pattern remains")
}

// UnusedSequence reserves sequence zero and every instrument/effect reference.
// Only the default one-step zero definition is reusable; a longer all-zero
// envelope still represents deliberate timing and is preserved.
func UnusedSequence(p *model.Project, after int, live []byte) (int, error) {
	if p == nil {
		return 0, fmt.Errorf("edit: no sequence bank")
	}
	var used [model.MaxSequences]bool
	used[0] = true
	for _, inst := range p.Bank.Instruments {
		for _, id := range inst[48:56] {
			used[id] = true
		}
	}
	sequenceCommand := func(code byte) bool {
		switch code {
		case 'L', 'A', 'V', 'M', 'N', 'F', 'I', '8':
			return true
		}
		return false
	}
	for _, pattern := range p.Song.Patterns {
		for _, cell := range pattern {
			if sequenceCommand(cell.Effect1) {
				used[cell.Parameter1] = true
			}
			if sequenceCommand(cell.Effect2) {
				used[cell.Parameter2] = true
			}
		}
	}
	for _, id := range live {
		used[id] = true
	}
	for offset := 1; offset <= model.MaxSequences; offset++ {
		id := (max(-1, min(model.MaxSequences-1, after)) + offset) % model.MaxSequences
		sequence := p.Bank.Sequences[id]
		if !used[id] && sequence.Length <= 1 && sequence.Repeat == 0 && sequence.Values == ([63]uint16{}) {
			return id, nil
		}
	}
	return 0, fmt.Errorf("edit: no blank unreferenced sequence remains")
}
