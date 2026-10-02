package ymimport

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type sourceMixerFrame struct{ mixer, noise byte }
type sourceMixerVoice struct {
	inst                    int
	phase, fixed, noiseMode bool
	held                    byte
	program                 []SourceNoiseStep
	step                    int
	noiseActive             bool
}

// Classic fixed-pitch voices alternate noise and tone at replay-call cadence.
// The noise shadow is shared: note/mixer commands from any voice can change it.
func classicMixerFrames(score SourceScore) ([]sourceMixerFrame, error) {
	if score.Player != madMaxClassic || score.Frames < 1 || score.Frames > 1000000 {
		return nil, fmt.Errorf("source: unverified classic mixer timeline")
	}
	controls := make([][]SourceControl, score.Frames)
	events := make([][]SourceEvent, score.Frames)
	for _, c := range score.Controls {
		if c.Frame < 0 || c.Frame >= score.Frames || c.Channel < 0 || c.Channel >= 3 {
			return nil, fmt.Errorf("source: invalid mixer control position")
		}
		controls[c.Frame] = append(controls[c.Frame], c)
	}
	for _, e := range score.Events {
		if e.Frame < 0 || e.Frame >= score.Frames || e.Channel < 0 || e.Channel >= 3 {
			return nil, fmt.Errorf("source: invalid mixer note position")
		}
		events[e.Frame] = append(events[e.Frame], e)
	}
	sourceMixerVoices := [3]sourceMixerVoice{{inst: -1}, {inst: -1}, {inst: -1}}
	// These initial shadows belong to the complete program identified by the
	// classic decoder's digest; they are not defaults for unrelated players.
	shadow, base := byte(25), byte(0x38)
	frames := make([]sourceMixerFrame, score.Frames)
	for frame := range frames {
		output := shadow
		for ch := 0; ch < 3; ch++ {
			v := &sourceMixerVoices[ch]
			mask := byte(9 << ch)
			var cs []SourceControl
			var es []SourceEvent
			for _, c := range controls[frame] {
				if c.Channel == ch {
					cs = append(cs, c)
				}
			}
			for _, e := range events[frame] {
				if e.Channel == ch {
					es = append(es, e)
				}
			}
			if len(es) > 0 || len(cs) > 0 {
				v.phase = false
				v.fixed = false
			}
			for _, c := range cs {
				switch {
				case c.Opcode >= 0xc0 && c.Opcode < 0xe0:
					v.inst = int(c.Opcode - 0xc0)
					if v.inst >= len(score.Instruments) {
						return nil, fmt.Errorf("source: mixer control references an invalid instrument")
					}
				case c.Opcode == 0x8a:
					base = (base &^ mask) | (mask & 0x38)
					v.noiseMode = false
				case c.Opcode == 0x8b:
					base = (base &^ mask) | (mask & 7)
					v.noiseMode = true
				case c.Opcode == 0x8c:
					base &^= mask
					v.noiseMode = true
				case c.Opcode == 0x8d:
					v.fixed = true
				}
			}
			for _, e := range es {
				if e.Rest {
					v.fixed = false
					continue
				}
				v.inst = e.Instrument
				if v.inst < 0 || v.inst >= len(score.Instruments) || len(score.Instruments[v.inst].Settings) != 6 {
					return nil, fmt.Errorf("source: mixer note references an invalid instrument")
				}
				if v.noiseMode {
					shadow = byte(e.NativeNote)
				}
				v.held = shadow
				base = (base &^ mask) | (mask & 0x38)
				v.fixed = v.fixed || score.Instruments[v.inst].Settings[0]&2 != 0
				v.program = score.Instruments[v.inst].NoiseProgram
				v.step = 0
				v.noiseActive = len(v.program) > 0
			}
		}
		mixer := byte(0x38)
		for ch := 0; ch < 3; ch++ {
			v := &sourceMixerVoices[ch]
			mask := byte(9 << ch)
			if v.noiseActive {
				v.step++
				if v.step >= len(v.program) {
					v.noiseActive = false
				} else {
					p := v.program[v.step]
					switch p.Mode {
					case 1:
						base = (base &^ mask) | (mask & 7)
						shadow = p.Period
						v.noiseMode = true
					case 2:
						base &^= mask
						shadow = p.Period
						v.noiseMode = true
					case 3:
						base = (base &^ mask) | (mask & 0x38)
						shadow = v.held
						v.noiseMode = false
					}
				}
			}
			v.phase = !v.phase
			value := base
			if v.fixed && v.phase {
				value = 7
				output = shadow ^ 8
			}
			mixer = (mixer &^ mask) | (value & mask)
		}
		frames[frame] = sourceMixerFrame{mixer: mixer & 63, noise: output & 31}
	}
	return frames, nil
}

// prepareClassicMixerScore keeps fixed-pitch source flags in the inspection but
// converts their base envelope/arpeggio through the existing bank translator.
// Their actual mixer/noise behavior is supplied by the generated score below.
func prepareClassicMixerScore(score SourceScore) (SourceScore, []int) {
	if score.Player != madMaxClassic {
		return score, nil
	}
	// The additional native noise sweep has not been verified by this model.
	for _, control := range score.Controls {
		if control.Opcode == 0x8f {
			return score, nil
		}
	}
	edited := score
	edited.Instruments = append([]SourceInstrument(nil), score.Instruments...)
	var ids []int
	for index := range edited.Instruments {
		definition := &edited.Instruments[index]
		if len(definition.Settings) != 6 || definition.Settings[0] != 2 {
			continue
		}
		definition.Settings = append([]byte(nil), definition.Settings...)
		definition.Settings[0] = 0
		ids = append(ids, definition.ID)
	}
	return edited, ids
}

func applyClassicMixer(score SourceScore, rows [][3]model.Cell, bank *model.VoiceBank, start int, candidates []int) ([]int, int, error) {
	var ids []int
	for _, id := range candidates {
		if id >= 0 && id < len(bank.Instruments) && bank.Instruments[id][48] != 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, 0, nil
	}
	frames, err := classicMixerFrames(score)
	if err != nil {
		return nil, 0, err
	}
	selected := map[int]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	sequenceIDs := map[uint16]byte{}
	for id := 1; id < bank.SequenceCount; id++ {
		sequence := bank.Sequences[id]
		if sequence.Length == 1 && sequence.Repeat == 0 {
			sequenceIDs[sequence.Values[0]] = byte(id)
		}
	}
	sequence := func(value uint16) (byte, error) {
		if id, ok := sequenceIDs[value]; ok {
			return id, nil
		}
		if bank.SequenceCount == model.MaxSequences {
			return 0, fmt.Errorf("source: mixer conversion exceeds native sequence capacity")
		}
		id := byte(bank.SequenceCount)
		bank.SequenceCount++
		bank.Sequences[id] = model.Sequence{Length: 1, Values: [63]uint16{value}}
		sequenceIDs[value] = id
		return id, nil
	}
	insert := func(cell *model.Cell, code, parameter byte) error {
		if cell.Effect1 == 0 {
			cell.Effect1, cell.Parameter1 = code, parameter
		} else if cell.Effect2 == 0 {
			cell.Effect2, cell.Parameter2 = code, parameter
		} else {
			return fmt.Errorf("source: mixer conversion needs a free native effect column")
		}
		return nil
	}
	events := make([][]SourceEvent, score.Frames)
	for _, event := range score.Events {
		events[event.Frame] = append(events[event.Frame], event)
	}
	active := [3]int{-1, -1, -1}
	lastMixer, lastNoise := [3]int{-1, -1, -1}, [3]int{-1, -1, -1}
	changes := 0
	for frame := 0; frame < start+len(rows); frame++ {
		for _, event := range events[frame] {
			if event.Rest {
				active[event.Channel] = -1
			} else {
				active[event.Channel] = event.Instrument
			}
		}
		if frame < start {
			continue
		}
		for channel, id := range active {
			if !selected[id] {
				lastMixer[channel], lastNoise[channel] = -1, -1
				continue
			}
			cell := &rows[frame-start][channel]
			// These definitions have no pitch-mask program. A zero V segment
			// from a preceding voice is inert here and leaves room for M/N.
			if cell.Effect1 == 'V' && bank.Instruments[id][18]&4 == 0 {
				cell.Effect1, cell.Parameter1 = 0, 0
			}
			mix := uint16(0)
			if frames[frame].mixer&(1<<channel) == 0 {
				mix |= 0x100
			}
			if frames[frame].mixer&(1<<(channel+3)) == 0 {
				mix |= 0x1000
			}
			if lastMixer[channel] != int(mix) || cell.Instrument != 0 {
				sequenceID, err := sequence(mix)
				if err != nil {
					return nil, 0, err
				}
				if err := insert(cell, 'M', sequenceID); err != nil {
					return nil, 0, err
				}
				lastMixer[channel] = int(mix)
				changes++
			}
			noise := int(frames[frame].noise)
			if mix&0x1000 != 0 && (lastNoise[channel] != noise || cell.Instrument != 0) {
				sequenceID, err := sequence(uint16(noise))
				if err != nil {
					return nil, 0, err
				}
				if err := insert(cell, 'N', sequenceID); err != nil {
					return nil, 0, err
				}
				lastNoise[channel] = noise
				changes++
			}
		}
	}
	return ids, changes, nil
}
