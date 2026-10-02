package ymimport

// Classic 89 replaces the global signed-byte transpose. The initializer sets
// it to -12; all three already sounding voices use its final value on this call.
// Added pitch-only events retain envelope and modulation phase in the score.
func classicGlobalTranspose(events []SourceEvent, controls []SourceControl, frames int) []SourceEvent {
	var out []SourceEvent
	var held [3]SourceEvent
	var sounding [3]bool
	eventAt, controlAt, shift := 0, 0, 0
	for frame := 0; frame < frames; frame++ {
		previous := shift
		var transpose SourceControl
		for controlAt < len(controls) && controls[controlAt].Frame == frame {
			c := controls[controlAt]
			if c.Opcode == 0x89 && len(c.Operand) == 1 {
				shift, transpose = int(int8(c.Operand[0]))+12, c
			}
			controlAt++
		}
		var changed [3]bool
		for eventAt < len(events) && events[eventAt].Frame == frame {
			e := events[eventAt]
			held[e.Channel], sounding[e.Channel], changed[e.Channel] = e, !e.Rest, true
			eventAt++
		}
		for ch := range held {
			if !changed[ch] && (shift == previous || !sounding[ch]) {
				continue
			}
			e := held[ch]
			if !e.Rest {
				e.Note += shift
			}
			if !changed[ch] {
				e.Frame, e.Offset, e.Retrigger, e.PitchOnly = frame, transpose.Offset, false, true
			}
			out = append(out, e)
		}
	}
	return out
}
