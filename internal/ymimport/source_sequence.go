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
