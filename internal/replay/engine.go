// Package replay translates MaxYMiser's pattern and instrument sequencing.
package replay

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type Voice struct {
	Parameters                         [48]byte
	Note, Instrument                   byte
	SeqIndex                           [7]int
	SeqDone                            [7]bool
	SeqClock                           int
	Values                             [7]uint16
	ColumnVolume, TrackVolume          int
	Transpose, NoiseTranspose          int
	SlideRate, Slide, PortaRate, Porta int16
	Extra                              [2]byte
	ExtraStep                          int
	PWMRate                            int8
	PWMOffset                          int16
	PWMLocked                          bool
	Triggered                          bool
}
type PCMVoice struct {
	Sample, Note, Volume byte
	Triggered            bool
}

type Engine struct {
	Project                         *model.Project
	Voices                          [3]Voice
	DMA                             [2]PCMVoice
	Registers                       [14]byte
	EnvelopeWrite                   bool
	Position, Row, TickInRow, Speed int
	Playing, PatternMode, Jam       bool
	Patterns                        [4]byte
	Mutes, TimerMask, Zync          byte
	Loops                           int
	Break                           bool
	Ticks                           uint64
	MasterVolume, Pan, Bass, Treble int
	pending                         [3]bool
}

func New(p *model.Project) *Engine {
	e := &Engine{Project: p}
	e.Reset()
	return e
}
func (e *Engine) Reset() {
	e.Voices = [3]Voice{}
	e.DMA = [2]PCMVoice{}
	e.Registers = [14]byte{}
	e.Registers[7] = 255
	e.Position, e.Row, e.TickInRow = 0, 0, 0
	e.Speed = e.Project.Song.Speed()
	e.Playing, e.PatternMode, e.Break = false, false, false
	e.Mutes = e.Project.Song.State[37]
	e.TimerMask = e.Project.Song.State[36] & 7
	e.MasterVolume, e.Pan, e.Bass, e.Treble = 127, 0, 6, 6
	e.Loops = 0
	e.Ticks = 0
	e.Patterns = e.Project.Song.Orders[0]
}
func (e *Engine) Play(pattern bool) {
	e.Playing = true
	e.PatternMode = pattern
	e.TickInRow = 0
	e.Row = 0
}
func (e *Engine) Stop() {
	e.Playing = false
	e.Voices = [3]Voice{}
	e.DMA = [2]PCMVoice{}
	e.Registers[7] = 255
	e.Registers[8], e.Registers[9], e.Registers[10] = 0, 0, 0
}
func (e *Engine) Tick() {
	e.Ticks++
	e.EnvelopeWrite = false
	for i := range e.Voices {
		e.Voices[i].Triggered = e.pending[i]
		e.pending[i] = false
	}
	for i := range e.DMA {
		e.DMA[i].Triggered = false
	}
	if e.Playing && e.TickInRow == 0 {
		for ch := range 3 {
			e.parse(ch, e.cell(e.Patterns[ch], e.Row), e.Mutes&(1<<ch) != 0)
		}
		e.parseDMA(e.cell(e.Patterns[3], e.Row))
	}
	for ch := range 3 {
		e.sequence(&e.Voices[ch])
	}
	e.configure()
	for ch := range 3 {
		v := &e.Voices[ch]
		v.PWMOffset = max(-255, min(255, v.PWMOffset+int16(v.PWMRate)))
		if v.Parameters[0] != 0 {
			v.Slide += v.SlideRate
			if v.PortaRate != 0 {
				old := v.Porta
				v.Porta -= v.PortaRate
				if old >= 0 && v.Porta <= 0 || old < 0 && v.Porta >= 0 {
					v.Porta, v.PortaRate = 0, 0
				}
			}
		}
	}
	if e.Playing {
		e.TickInRow++
		if e.TickInRow >= max(1, e.Speed) {
			e.TickInRow = 0
			e.Row++
			if e.Break || e.Row >= 64 {
				e.Break = false
				e.Row = 0
				if !e.PatternMode {
					e.Position++
					if e.Position >= int(e.Project.Song.Length) {
						e.Position = int(e.Project.Song.Repeat)
						e.Loops++
					}
					e.Patterns = e.Project.Song.Orders[e.Position]
				}
			}
		}
	}
}
func (e *Engine) cell(id byte, row int) model.Cell {
	if id == model.NoteOffPattern && row == 0 {
		return model.Cell{Note: 1}
	}
	if id >= 240 || int(id) >= len(e.Project.Song.Patterns) {
		return model.Cell{}
	}
	return e.Project.Song.Patterns[id][row%64]
}
func (e *Engine) Trigger(channel int, note, instrument byte) {
	if channel < 0 || channel >= 3 {
		return
	}
	e.parse(channel, model.Cell{Note: note, Instrument: instrument}, false)
	e.pending[channel] = true
}
func (e *Engine) parse(ch int, cell model.Cell, muted bool) {
	v := &e.Voices[ch]
	if !muted {
		porta := cell.Effect1 == 'P' || cell.Effect2 == 'P'
		if cell.Instrument > 0 && cell.Instrument <= 32 {
			if v.Instrument != cell.Instrument {
				copy(v.Parameters[:], e.Project.Bank.Instruments[cell.Instrument-1][16:])
			}
			v.Instrument = cell.Instrument
			oldSlide, oldPorta := v.Slide, v.Porta
			v.SeqIndex = [7]int{}
			v.SeqDone = [7]bool{}
			v.Values = [7]uint16{}
			v.SeqClock = 0
			v.ColumnVolume, v.Transpose, v.NoiseTranspose = 0, 0, 0
			v.Extra = [2]byte{}
			v.ExtraStep = 0
			v.Slide, v.Porta, v.SlideRate, v.PortaRate = 0, 0, 0, 0
			if porta {
				v.Slide, v.Porta = oldSlide, oldPorta
			}
			v.Triggered = true
		}
		if cell.Note == model.NoteOff {
			*v = Voice{}
			v.Triggered = true
		} else if cell.Note > 1 {
			if porta && v.Note > 1 {
				v.Porta += int16(table(tonePeriods[:], int(v.Note))) - int16(table(tonePeriods[:], int(cell.Note))) + v.Slide
				v.Slide = 0
			}
			v.Note = cell.Note
		}
		if cell.Volume != 0 {
			v.ColumnVolume = int(cell.Volume & 15)
		}
	}
	e.effect(v, cell.Effect1, cell.Parameter1, muted)
	e.effect(v, cell.Effect2, cell.Parameter2, muted)
}
func (e *Engine) parseDMA(c model.Cell) {
	vals := [2][3]byte{{c.Note, c.Instrument, c.Volume}, {c.Effect1, c.Parameter1, c.Effect2}}
	for i, x := range vals {
		v := &e.DMA[i]
		if x[0] == 1 {
			v.Note = 0
			v.Triggered = true
		} else if x[0] > 1 {
			v.Note = x[0]
			v.Triggered = true
		}
		if x[1] > 0 && x[1] <= 8 {
			v.Sample = x[1]
			v.Triggered = true
		}
		if x[2] > 0 {
			v.Volume = x[2] & 15
		}
	}
}
func (e *Engine) effect(v *Voice, code, value byte, muted bool) {
	if muted && code != 'B' && code != 'S' && code != 'U' && code != 'Y' && code != 'Z' && code != '7' {
		return
	}
	if code >= '1' && code <= '6' {
		index := int(code - '1')
		v.Parameters[index] = value
		if index == 1 {
			v.SeqDone[1] = false
		}
		if index == 2 {
			v.SeqDone[2] = false
		}
		if index == 4 {
			v.SeqDone[5] = false
		}
		return
	}
	sequenceMap := map[byte]int{'L': 0, 'A': 1, 'V': 2, 'M': 3, 'N': 4, 'F': 5, '8': 6}
	if index, ok := sequenceMap[code]; ok {
		parameter := 32 + index
		if index == 6 {
			parameter = 39
		}
		v.Parameters[parameter] = value
		v.SeqIndex[index] = 0
		v.SeqDone[index] = false
		return
	}
	switch code {
	case '7':
		v.PWMLocked = false
		if value == 128 {
			v.PWMRate = 0
			v.PWMOffset = 0
		} else {
			v.PWMRate = int8(value)
		}
	case '9':
		v.Parameters[17] = value
		v.PWMLocked = true
	case 'B':
		e.Break = true
	case 'C':
		v.Parameters[19] = value
		v.Triggered = true
	case 'D':
		v.Parameters[23] = value
	case 'E':
		v.Parameters[24] = value
	case 'G':
		if value <= 8 {
			v.Parameters[20] = value
			v.Triggered = true
		}
	case 'H':
		v.SlideRate = -int16(int8(value))
		v.PortaRate = 0
	case 'I':
		v.Parameters[38] = value
	case 'O':
		v.NoiseTranspose = int(int8(value))
	case 'P':
		v.PortaRate = int16(value)
		if v.Porta < 0 {
			v.PortaRate = -v.PortaRate
		}
		v.SlideRate = 0
	case 'Q':
		v.Parameters[16] = value
	case 'R':
		if value >= 24 && value <= 205 {
			v.Parameters[21] = value
		}
	case 'S':
		if value >= 2 {
			e.Speed = int(value)
		}
	case 'T':
		v.Transpose = int(int8(value))
	case 'U':
		switch {
		case value <= 12:
			e.Bass = int(value)
		case value >= 16 && value <= 28:
			e.Treble = int(value - 16)
		case value >= 128 && value <= 160:
			e.MasterVolume = int(value-128) * 127 / 32
		case value >= 192 && value <= 224:
			e.Pan = int(value) - 208
		}
	case 'W':
		v.Parameters[18] = value & 15
	case 'X':
		v.Extra = [2]byte{value >> 4, value & 15}
		v.ExtraStep = 0
		v.SeqDone[1] = false
	case 'Y':
		e.TimerMask = value & 7
	case 'Z':
		e.Zync = value
	}
}
func (e *Engine) sequence(v *Voice) {
	if v.Note == 0 {
		return
	}
	if v.SeqClock > 0 {
		v.SeqClock++
		if v.SeqClock >= int(v.Parameters[16]) {
			v.SeqClock = 0
		}
		return
	}
	v.SeqClock++
	if v.SeqClock >= int(v.Parameters[16]) {
		v.SeqClock = 0
	}
	for kind := range 7 {
		if v.SeqDone[kind] {
			continue
		}
		parameter := 32 + kind
		if kind == 6 {
			parameter = 39
		}
		number := v.Parameters[parameter]
		disabled := number == 0 || (kind == 1 && v.Parameters[1] == 0) || (kind == 2 && v.Parameters[2] == 0) || (kind == 5 && v.Parameters[4] == 0)
		if disabled {
			v.Values[kind] = 0
			v.SeqDone[kind] = true
			continue
		}
		seq := e.Project.Bank.Sequences[number]
		length := min(int(seq.Length), len(seq.Values))
		if length <= 0 {
			v.Values[kind] = 0
			v.SeqDone[kind] = true
			continue
		}
		index := min(v.SeqIndex[kind], length-1)
		v.Values[kind] = seq.Values[index]
		index++
		if index >= length {
			index = min(int(seq.Repeat), length-1)
			if index == length-1 {
				v.SeqDone[kind] = true
			}
		}
		v.SeqIndex[kind] = index
	}
	if v.Extra[0] != 0 || v.Extra[1] != 0 {
		switch v.ExtraStep {
		case 0:
			v.Values[1] = 0
		case 1:
			v.Values[1] = uint16(v.Extra[0])
		case 2:
			v.Values[1] = uint16(v.Extra[1])
		}
		v.ExtraStep++
		if v.Extra[0] == 15 || v.Extra[1] == 15 {
			if v.Extra[0] == 15 {
				v.Values[1] = uint16(v.Extra[1])
			}
			v.ExtraStep %= 2
		} else {
			v.ExtraStep %= 3
		}
	}
}
func table(values []uint16, index int) uint16 {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}
func (e *Engine) Period(v *Voice, component int) uint16 {
	if v.Note == 0 {
		return 0
	}
	bit := byte(1 << component)
	p := v.Parameters
	if p[4]&bit != 0 {
		if p[5]&bit != 0 && p[37] == 0 {
			return uint16(p[23])<<8 | uint16(p[24])
		}
		if v.Values[5] != 65535 {
			return v.Values[5]
		}
	}
	adjust := 0
	if p[5]&bit != 0 {
		adjust = int(int8(p[23]))
	}
	if p[1]&bit != 0 {
		adjust += int(int16(v.Values[1]))
	}
	if p[3]&bit != 0 {
		adjust += v.Transpose
	}
	index := int(v.Note) + adjust
	period := int(table(tonePeriods[:], index))
	if component == 1 {
		period = int(table(envelopePeriods[:], index))
	}
	if component == 0 {
		period = int(table(timerPeriods[:], index))
	}
	if component == 2 && p[25] != 0 {
		period = int(table(envelopePeriods[:], index)) * 16
	}
	if p[0]&bit != 0 {
		delta := int(v.Porta + v.Slide)
		if adjust >= 0 {
			scale := int(table(tuningScale[:], adjust))
			if scale != 0 {
				delta = delta * 32 / scale
			}
		} else {
			delta = delta * int(table(tuningScale[:], -adjust)) / 32
		}
		if component == 1 {
			delta = (delta >> 3) + 1
			delta >>= 1
		}
		period += delta
	}
	if p[5]&bit != 0 {
		fine := int(int8(p[24]))
		if component == 1 {
			fine = ((fine >> 3) + 1) >> 1
		}
		period -= fine
	}
	if p[2]&bit != 0 {
		vibrato := int(int16(v.Values[2]))
		if component == 1 {
			vibrato = ((vibrato >> 3) + 1) >> 1
		}
		period -= vibrato
	}
	if component == 2 && period > 4095 {
		return 0
	}
	return uint16(max(0, min(65535, period)))
}
func (e *Engine) configure() {
	e.Registers[7] = 255
	winner := -1
	for ch := range 3 {
		v := &e.Voices[ch]
		mixer := v.Values[3]
		volume := int(v.Values[0]&255) - v.ColumnVolume - int(v.Parameters[22]) - v.TrackVolume
		if e.Mutes&(1<<ch) != 0 || v.Note == 0 || mixer == 0 || volume <= 0 {
			volume = 0
			mixer &= 0xff0f
			v.Values[3] = mixer
		}
		if mixer&0xf0 != 0 && volume > 0 {
			volume = 16
			winner = ch
		}
		e.Registers[8+ch] = byte(min(16, max(0, volume)))
		if volume > 0 {
			if mixer&0x0100 != 0 {
				period := e.Period(v, 2)
				e.Registers[ch*2], e.Registers[ch*2+1] = byte(period), byte(period>>8)&15
				if period != 0 {
					e.Registers[7] &^= 1 << ch
				}
			}
			if mixer&0x1000 != 0 {
				e.Registers[7] &^= 1 << (ch + 3)
				noise := int(v.Values[4]) + v.NoiseTranspose
				e.Registers[6] = byte(max(0, min(31, noise)))
			}
		}
	}
	if winner >= 0 {
		v := &e.Voices[winner]
		period := e.Period(v, 1)
		e.Registers[11], e.Registers[12] = byte(period), byte(period>>8)
		shape := byte(v.Values[3]>>4) & 15
		if shape == 1 {
			shape = v.Parameters[18] & 15
		}
		if e.Registers[13] != shape || v.Triggered && v.Parameters[19] != 0 {
			e.EnvelopeWrite = true
		}
		e.Registers[13] = shape
	}
}
