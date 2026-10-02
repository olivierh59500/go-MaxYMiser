package ymimport

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// sourceModulation follows the byte-sized phase counters and signed 16-bit
// period accumulator of the verified Mad Max player. Rates describe replay
// calls, not musical rows. It is independent of MaxYMiser's effect encoding.
type sourceModulation struct {
	depth, step, level, delay byte
	up, enabled, started      bool
	slideEnabled              bool
	slideRate                 int8
	slideDelay                byte
	slide                     int16
}

func (m *sourceModulation) instrument(settings []byte) {
	m.depth, m.level = settings[3], settings[3]
	m.step, m.delay = settings[2], settings[4]
	// Selecting a source instrument does not switch the pattern's vibrato
	// direction or enable flag. A following non-legato note resets its delay.
}

func (m *sourceModulation) note(delay byte, retrigger bool) {
	m.delay = delay
	if retrigger {
		m.started = false
	}
}

func (m *sourceModulation) vibrato(enabled bool) {
	m.enabled, m.up = enabled, false
}

func (m *sourceModulation) pitchSlide(rate int8, delay byte) {
	m.slideEnabled, m.slideRate, m.slideDelay, m.slide = true, rate, delay, 0
}

// periodDelta matches the native operation order: vibrato before pitch slide.
// effectiveNote is the index passed to the original period table, after its
// global transpose and arpeggio. The ordinary MIDI note is index + 24.
func (m *sourceModulation) periodDelta(effectiveNote int) int16 {
	delta := int16(0)
	if m.enabled {
		active := m.started
		if !active {
			previous := m.delay
			m.delay--
			active = previous == 0
		}
		if active {
			m.started = true
			rangeLimit := m.depth * 2
			if !m.up {
				if m.level < m.step {
					m.level, m.up = 0, true
				} else {
					m.level -= m.step
				}
			} else {
				m.level += m.step
				if m.level >= rangeLimit {
					m.level, m.up = rangeLimit, false
				}
			}
			delta = int16(int(m.level) - int(m.depth))
			index := int(byte(effectiveNote*2)) + 160
			if index < 256 {
				for index < 256 {
					delta *= 2
					index += 24
				}
			}
		}
	}
	if m.slideEnabled {
		m.slideDelay--
		if m.slideDelay == 0 {
			// The original routine retains a counter of one after its
			// initial delay, then accumulates the signed rate on every call.
			m.slideDelay = 1
			m.slide += int16(m.slideRate)
		}
		delta += m.slide
	}
	return delta
}

// sourcePitchDeltas derives modulation only for the translated ordinary tone
// programs. Hardware/noise programs and nonconstant source arpeggios retain
// explicit unsupported status instead of borrowing this simpler pitch model.
func sourcePitchDeltas(score SourceScore) ([][3]int16, error) {
	if score.Frames < 1 || score.Frames > 1000000 {
		return nil, fmt.Errorf("source: invalid modulation timeline length")
	}
	values := make([][3]int16, score.Frames)
	controls := make([][]SourceControl, score.Frames)
	events := make([][]SourceEvent, score.Frames)
	for _, control := range score.Controls {
		if control.Channel < 0 || control.Channel >= 3 || control.Frame < 0 || control.Frame >= score.Frames {
			return nil, fmt.Errorf("source: invalid pitch-control position")
		}
		controls[control.Frame] = append(controls[control.Frame], control)
	}
	for _, event := range score.Events {
		if event.Channel < 0 || event.Channel >= 3 || event.Frame < 0 || event.Frame >= score.Frames {
			return nil, fmt.Errorf("source: invalid modulation note position")
		}
		events[event.Frame] = append(events[event.Frame], event)
	}
	var state [3]sourceModulation
	instrument := [3]int{-1, -1, -1}
	note := [3]int{}
	eligible := [3]bool{}
	for frame := range values {
		for _, event := range events[frame] {
			state[event.Channel].slideEnabled = false
		}
		for _, control := range controls[frame] {
			m := &state[control.Channel]
			switch {
			case control.Opcode >= 0xc0 && control.Opcode < 0xe0:
				id := int(control.Opcode - 0xc0)
				if id >= len(score.Instruments) {
					return nil, fmt.Errorf("source: pitch control references an invalid instrument")
				}
				instrument[control.Channel] = id
				m.instrument(score.Instruments[id].Settings)
			case control.Opcode == 0x81:
				m.vibrato(false)
			case control.Opcode == 0x82:
				m.vibrato(true)
			case control.Opcode == 0x84:
				if len(control.Operand) != 2 {
					return nil, fmt.Errorf("source: truncated pitch-slide control")
				}
				m.pitchSlide(int8(control.Operand[0]), control.Operand[1])
			}
		}
		for _, event := range events[frame] {
			channel := event.Channel
			if event.Rest {
				continue
			}
			if instrument[channel] < 0 {
				instrument[channel] = event.Instrument
				state[channel].instrument(score.Instruments[event.Instrument].Settings)
			}
			definition := score.Instruments[event.Instrument]
			note[channel] = event.Note - 24
			eligible[channel] = definition.Settings[0] == 0 && len(definition.Arpeggio.Values) == 1 && definition.Arpeggio.Values[0] == 0
			state[channel].note(definition.Settings[4], event.Retrigger)
		}
		for channel := range state {
			delta := state[channel].periodDelta(note[channel])
			if eligible[channel] {
				values[frame][channel] = delta
			}
		}
	}
	return values, nil
}

func applySourceModulation(score SourceScore, rows [][3]model.Cell, bank *model.VoiceBank, start int) (int, error) {
	if len(score.Controls) == 0 {
		return 0, nil
	}
	values, err := sourcePitchDeltas(score)
	if err != nil {
		return 0, err
	}
	ids := map[model.Sequence]byte{}
	for id := 1; id < bank.SequenceCount; id++ {
		ids[bank.Sequences[id]] = byte(id)
	}
	add := func(sequence model.Sequence) (byte, error) {
		if id, ok := ids[sequence]; ok {
			return id, nil
		}
		if bank.SequenceCount == model.MaxSequences {
			return 0, fmt.Errorf("source: pitch modulation exceeds native sequence capacity; select a shorter excerpt")
		}
		id := byte(bank.SequenceCount)
		bank.SequenceCount++
		bank.Sequences[id] = sequence
		ids[sequence] = id
		return id, nil
	}
	segments := 0
	for channel := 0; channel < 3; channel++ {
		for at := 0; at < len(rows); {
			end := min(at+63, len(rows))
			for next := at + 1; next < end; next++ {
				if rows[next][channel].Instrument != 0 {
					end = next
					break
				}
			}
			sequence := model.Sequence{Length: byte(end - at), Repeat: byte(end - at - 1)}
			nonzero := false
			for frame := at; frame < end; frame++ {
				delta := values[start+frame][channel]
				sequence.Values[frame-at] = uint16(-delta)
				nonzero = nonzero || delta != 0
			}
			// Zero segments also terminate an earlier modulation program.
			if nonzero || at > 0 {
				id, err := add(sequence)
				if err != nil {
					return 0, err
				}
				rows[at][channel].Effect1, rows[at][channel].Parameter1 = 'V', id
				segments++
			}
			at = end
		}
	}
	for _, definition := range score.Instruments {
		if definition.Settings[0] == 0 && len(definition.Arpeggio.Values) == 1 && definition.Arpeggio.Values[0] == 0 {
			bank.Instruments[definition.ID][18] |= 4
		}
	}
	return segments, nil
}
