package edit

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type PackResult struct {
	PatternsBefore, PatternsAfter, SequencesBefore, SequencesAfter int
}

// PackProject removes duplicate patterns and sequences while retaining every
// stored definition. It rewrites song positions, editor pattern state,
// instrument links and both sequence-select effect columns. Sample and
// instrument data are not removed. This is a transaction on a cloned project.
func PackProject(project *model.Project) (PackResult, error) {
	p := project.Clone()
	result := PackResult{PatternsBefore: len(p.Song.Patterns), SequencesBefore: p.Bank.SequenceCount}
	if len(p.Song.Patterns) > model.MaxPatterns || p.Bank.SequenceCount < 1 || p.Bank.SequenceCount > model.MaxSequences {
		return result, fmt.Errorf("edit: invalid project dimensions")
	}
	var patternMap [240]byte
	var sourceRoles [240]byte
	for _, order := range p.Song.Orders[:p.Song.Length] {
		for channel, id := range order {
			if channel == 3 && p.Song.State[49] == 0 {
				continue
			}
			if id < 240 {
				if int(id) >= len(p.Song.Patterns) {
					return result, fmt.Errorf("edit: active song track refers to missing pattern %02X", id)
				}
				if channel == 3 {
					sourceRoles[id] |= 2
				} else {
					sourceRoles[id] |= 1
				}
			}
		}
	}
	type patternKey struct {
		pattern model.Pattern
		role    byte
	}
	uniquePatterns := map[patternKey]byte{}
	patterns := make([]model.Pattern, 0, len(p.Song.Patterns))
	for id, pattern := range p.Song.Patterns {
		key := patternKey{pattern, sourceRoles[id]}
		mapped, ok := uniquePatterns[key]
		if !ok {
			mapped = byte(len(patterns))
			patterns = append(patterns, pattern)
			uniquePatterns[key] = mapped
		}
		patternMap[id] = mapped
	}
	remapPattern := func(id byte) byte {
		if int(id) < len(p.Song.Patterns) {
			return patternMap[id]
		}
		return id
	}
	for pos := range p.Song.Orders {
		for ch, id := range p.Song.Orders[pos] {
			p.Song.Orders[pos][ch] = remapPattern(id)
		}
	}
	for _, base := range []int{4, 52} {
		for ch := 0; ch < 4; ch++ {
			p.Song.State[base+ch] = remapPattern(p.Song.State[base+ch])
		}
	}
	p.Song.Patterns = patterns
	var sequenceMap [256]byte
	var sequences [256]model.Sequence
	sequences[0] = p.Bank.Sequences[0]
	uniqueSequences := map[model.Sequence]byte{}
	count := 1
	for id := 1; id < p.Bank.SequenceCount; id++ {
		sequence := p.Bank.Sequences[id]
		mapped, ok := uniqueSequences[sequence]
		if !ok {
			mapped = byte(count)
			count++
			sequences[mapped] = sequence
			uniqueSequences[sequence] = mapped
		}
		sequenceMap[id] = mapped
	}
	// An ID above the serialized count still denotes an empty definition in
	// memory. Keep it stable rather than silently reusing it for another sound.
	for id := p.Bank.SequenceCount; id < 256; id++ {
		sequenceMap[id] = byte(id)
		sequences[id] = p.Bank.Sequences[id]
	}
	for id := range p.Bank.Instruments {
		for slot := 48; slot < 56; slot++ {
			p.Bank.Instruments[id][slot] = sequenceMap[p.Bank.Instruments[id][slot]]
		}
	}
	selectSequence := func(code byte) bool {
		switch code {
		case 'L', 'A', 'V', 'M', 'N', 'F', 'I', '8':
			return true
		}
		return false
	}
	// A pattern shared between YM and PCM lanes is ambiguous: its bytes can
	// be notes/samples in one lane and commands in another. Reject packing
	// rather than changing a PCM sample ID that resembles a sequence command.
	roles := map[byte]byte{}
	for _, order := range p.Song.Orders[:p.Song.Length] {
		for channel, id := range order {
			if channel == 3 && p.Song.State[49] == 0 {
				continue
			}
			if id >= 240 {
				continue
			}
			role := byte(1)
			if channel == 3 {
				role = 2
			}
			roles[id] |= role
		}
	}
	for id := range p.Song.Patterns {
		if roles[byte(id)]&2 != 0 {
			if roles[byte(id)] == 3 {
				for _, cell := range p.Song.Patterns[id] {
					if selectSequence(cell.Effect1) || selectSequence(cell.Effect2) {
						return result, fmt.Errorf("edit: pattern %02X shares YM and PCM command data; separate it before packing", id)
					}
				}
			}
			continue
		}
		for row := range p.Song.Patterns[id] {
			cell := &p.Song.Patterns[id][row]
			if selectSequence(cell.Effect1) {
				cell.Parameter1 = sequenceMap[cell.Parameter1]
			}
			if selectSequence(cell.Effect2) {
				cell.Parameter2 = sequenceMap[cell.Parameter2]
			}
		}
	}
	p.Bank.Sequences, p.Bank.SequenceCount = sequences, count
	result.PatternsAfter, result.SequencesAfter = len(patterns), count
	*project = *p
	return result, nil
}
