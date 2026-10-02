package midi

import "github.com/olivierh59500/go-MaxYMiser/internal/replay"

// instrumentController follows the native editor's seven-bit controller scales
// and edits the assigned bank definition, so changes can be heard and saved.
func instrumentController(e *replay.Engine, ch int, code, value byte) {
	if ch < 0 || ch >= 3 {
		return
	}
	id := e.Project.Song.State[32+ch]
	if id < 1 || id > 32 {
		return
	}
	inst := &e.Project.Bank.Instruments[id-1]
	offset, converted := 0, value
	switch {
	case code >= 60 && code <= 67:
		offset = 48 + int(code-60)
		converted = value >> 1
	case code == 109:
		offset = 38
		converted = 15 - (value >> 3)
	case code == 110:
		offset = 32
		converted = (value >> 2) + 1
	case code == 111:
		offset = 36
		converted = value / 15
	case code == 112:
		offset = 37
		converted = 151 - value
	case code == 113:
		offset = 34
		converted = value >> 3
	case code == 114:
		offset = 35
		converted = value >> 6
	case code == 115:
		offset = 39
		converted = value - 64
	case code == 116:
		offset = 40
		converted = value - 64
	case code == 117:
		offset = 33
		converted = value * 2
	default:
		return
	}
	inst[offset] = converted
	if offset >= 48 {
		e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, int(converted)+1)
	}
	for i := range e.Voices {
		v := &e.Voices[i]
		if v.Instrument != id {
			continue
		}
		v.Parameters[offset-16] = converted
		v.SeqClock = 0
		v.SeqIndex = [7]int{}
		v.SeqDone = [7]bool{}
	}
}
