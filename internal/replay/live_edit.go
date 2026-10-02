package replay

// RefreshSequence lets the editor replace a shared definition while preserving
// each sounding voice's sequence phase. Held tails become readable again at
// their current step; shortened sequences clamp to their new end.
func (e *Engine) RefreshSequence(id int) {
	if id < 0 || id >= len(e.Project.Bank.Sequences) {
		return
	}
	for i := range e.Voices {
		v := &e.Voices[i]
		for kind := range v.SeqDone {
			offset := 32 + kind
			if kind == 6 {
				offset = 39
			}
			if int(v.Parameters[offset]) != id {
				continue
			}
			length := int(e.Project.Bank.Sequences[id].Length)
			v.SeqIndex[kind] = max(0, min(v.SeqIndex[kind], max(0, length-1)))
			v.SeqDone[kind] = false
		}
	}
}

// RefreshInstrumentParameter applies one edited bank setting to voices using
// that instrument. Other pattern overrides and musical timing are retained.
func (e *Engine) RefreshInstrumentParameter(instrument, offset int) {
	if instrument < 0 || instrument >= len(e.Project.Bank.Instruments) || offset < 16 || offset > 55 {
		return
	}
	value := e.Project.Bank.Instruments[instrument][offset]
	if offset >= 48 {
		e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, int(value)+1)
	}
	for i := range e.Voices {
		v := &e.Voices[i]
		if int(v.Instrument) != instrument+1 {
			continue
		}
		v.Parameters[offset-16] = value
		if offset >= 48 {
			kind := offset - 48
			if kind == 7 {
				kind = 6
			}
			if kind == 6 && offset == 54 {
				continue
			}
			v.SeqIndex[kind] = 0
			v.SeqDone[kind] = false
		} else if offset >= 16 && offset <= 21 {
			for kind := range v.SeqDone {
				v.SeqDone[kind] = false
			}
		}
	}
}
