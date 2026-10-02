package ymimport

import "fmt"

func decodeSourceArpeggio(r sourceReader, table, id int) (SourceSequence, error) {
	var s SourceSequence
	at, err := r.pointer(table + id*4)
	if err != nil || at < 1 {
		return s, fmt.Errorf("invalid arpeggio pointer %d", id)
	}
	s.Offset, s.StepFrames = at, int(r.data[at-1])+1
	for n := 0; n < 256; n++ {
		if at+n >= len(r.data) {
			return s, fmt.Errorf("truncated arpeggio %d", id)
		}
		v := r.data[at+n]
		if v == 0x8f || v == 0x8e {
			if len(s.Values) == 0 {
				return s, fmt.Errorf("arpeggio %d has no values", id)
			}
			s.Repeat = 0
			if v == 0x8e {
				// 0x8e holds the previous byte; 0x8f repeats at the base.
				s.Repeat = len(s.Values) - 1
			}
			return s, nil
		}
		s.Values = append(s.Values, int(int8(v)))
	}
	return s, fmt.Errorf("unterminated arpeggio %d", id)
}

// The native noise-program pointer advances over the first pair before the
// first output call. The final 0xff marker leaves the previous mixer unchanged.
func decodeSourceNoiseProgram(data []byte, at int) ([]SourceNoiseStep, error) {
	var steps []SourceNoiseStep
	for count := 0; count < 64; count++ {
		if at < 0 || at >= len(data) {
			return nil, fmt.Errorf("truncated native noise program")
		}
		if data[at] == 255 {
			if len(steps) < 2 {
				return nil, fmt.Errorf("native noise program has no replay step")
			}
			return steps, nil
		}
		if at+1 >= len(data) || data[at] < 1 || data[at] > 3 {
			return nil, fmt.Errorf("unsupported native noise-program operation")
		}
		steps = append(steps, SourceNoiseStep{Mode: data[at], Period: data[at+1]})
		at += 2
	}
	return nil, fmt.Errorf("unterminated native noise program")
}
