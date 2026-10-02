// Package replay translates MaxYMiser's pattern and instrument sequencing.
package replay

import (
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type Voice struct {
	Parameters                          [48]byte
	Note, Instrument                    byte
	SeqIndex                            [7]int
	SeqDone                             [7]bool
	SeqClock                            int
	Values                              [7]uint16
	ColumnVolume, TrackVolume           int
	Transpose, NoiseTranspose           int
	TrackTranspose, TrackNoiseTranspose int
	SlideRate, Slide, PortaRate, Porta  int16
	Extra                               [2]byte
	ExtraStep                           int
	SequenceArpeggio                    uint16
	PWMRate                             int8
	PWMOffset                           int16
	PWMLocked                           bool
	Triggered                           bool
	ParametersDirty                     bool
	PreviewTriggers                     uint64
}
type PCMVoice struct {
	Sample, Note, Volume   byte
	Triggered              bool
	PreviewTriggers        uint64
	TrackVolume, Transpose int
}

type Engine struct {
	Project                                      *model.Project
	Voices                                       [3]Voice
	DMA                                          [2]PCMVoice
	Registers                                    [14]byte
	EnvelopeWrite                                bool
	Position, Row, TickInRow, Speed              int
	Playing, PatternMode, Jam                    bool
	Patterns                                     [4]byte
	Mutes, TimerMask, Zync                       byte
	Loops                                        int
	Break                                        bool
	Ticks                                        uint64
	MasterVolume, Pan, Bass, Treble              int
	MicrowireGain, MicrowireLeft, MicrowireRight float64
	pending                                      [3]bool
	pendingDMA                                   [2]bool
	ExternalClock                                bool
	clockPulses                                  int
	rowParsed                                    bool
	transportGeneration                          uint64
	transportCommand                             byte
	NextPatterns                                 [4]byte
	nextPatternMask                              byte
	NextPosition                                 int
	PositionQueued                               bool
}

func New(p *model.Project) *Engine {
	e := &Engine{Project: p}
	e.Reset()
	return e
}
func (e *Engine) Reset() {
	e.transportGeneration++
	e.transportCommand = 0xfc
	e.Voices = [3]Voice{}
	e.DMA = [2]PCMVoice{}
	e.pending = [3]bool{}
	e.pendingDMA = [2]bool{}
	e.Registers = [14]byte{}
	e.Registers[7] = 255
	e.Position, e.Row, e.TickInRow = 0, 0, 0
	e.Speed = e.Project.Song.Speed()
	e.Playing, e.PatternMode, e.Break = false, false, false
	e.Jam = e.Project.Song.State[39] != 0
	e.ExternalClock = e.Project.Song.State[31]&1 != 0
	e.clockPulses, e.rowParsed = 0, false
	e.NextPatterns, e.nextPatternMask = [4]byte{}, 0
	e.NextPosition, e.PositionQueued = 0, false
	e.Mutes = e.Project.Song.State[37]
	e.TimerMask = e.Project.Song.State[36] & 7
	e.MasterVolume, e.Pan, e.Bass, e.Treble = 127, 0, 6, 6
	e.MicrowireGain, e.MicrowireLeft, e.MicrowireRight = 1, 1, 1
	e.Loops = 0
	e.Ticks = 0
	e.EnvelopeWrite = false
	e.Zync = 0
	e.Patterns = e.Project.Song.Orders[0]
}
func (e *Engine) Play(pattern bool) {
	e.transportGeneration++
	e.transportCommand = 0xfa
	if !pattern {
		e.loadPosition()
	}
	e.Playing = true
	e.PatternMode = pattern
	e.TickInRow = 0
	e.Row = 0
	e.rowParsed, e.clockPulses = false, 0
}

// PlayFrom starts a selected row without replacing the live pattern combination
// in pattern mode. The caller chooses whether to stop existing voices first.
func (e *Engine) PlayFrom(pattern bool, row int) bool {
	if row < 0 || row >= model.Rows {
		return false
	}
	e.Play(pattern)
	e.Row = row
	return true
}

// Continue preserves the current row and replay phase for MIDI transport.
func (e *Engine) Continue() {
	e.Playing = true
	e.transportGeneration++
	e.transportCommand = 0xfb
}

func (e *Engine) Stop() {
	e.transportGeneration++
	e.transportCommand = 0xfc
	e.Playing = false
	e.pending = [3]bool{}
	e.PositionQueued = false
	e.nextPatternMask = 0
	e.clockPulses = 0
	// The audio renderer consumes a stop trigger at the next sequencer tick.
	e.pendingDMA = [2]bool{true, true}
	for i := range e.Voices {
		v := &e.Voices[i]
		*v = Voice{TrackVolume: v.TrackVolume, TrackTranspose: v.TrackTranspose, TrackNoiseTranspose: v.TrackNoiseTranspose}
	}
	for i := range e.DMA {
		v := &e.DMA[i]
		*v = PCMVoice{TrackVolume: v.TrackVolume, Transpose: v.Transpose}
	}
	e.Registers[7] = 255
	e.Registers[8], e.Registers[9], e.Registers[10] = 0, 0, 0
}
func (e *Engine) Tick() {
	e.tickWithOutput(nil)
}

// tickWithOutput lets the audio renderer consume each external call separately,
// retaining note and timer events when several pulses arrive in one buffer.
func (e *Engine) tickWithOutput(output func(external bool)) {
	if e.Playing && e.ExternalClock {
		// In the native editor, external pulses call the whole replayer.
		// The internal timer only services sound while playback is stopped.
		e.EnvelopeWrite = false
		for i := range e.Voices {
			e.Voices[i].Triggered = false
		}
		for i := range e.DMA {
			e.DMA[i].Triggered = false
		}
		var ym [3]bool
		var pcm [2]bool
		envelope := false
		hadPulses := e.clockPulses > 0
		for e.clockPulses > 0 {
			e.clockPulses--
			e.tick(true)
			if output != nil {
				output(true)
			}
			for i := range ym {
				ym[i] = ym[i] || e.Voices[i].Triggered
			}
			for i := range pcm {
				pcm[i] = pcm[i] || e.DMA[i].Triggered
			}
			envelope = envelope || e.EnvelopeWrite
		}
		for i := range ym {
			e.Voices[i].Triggered = ym[i]
		}
		for i := range pcm {
			e.DMA[i].Triggered = pcm[i]
		}
		e.EnvelopeWrite = envelope
		if output != nil && !hadPulses {
			output(false)
		}
		return
	}
	e.tick(false)
	if output != nil {
		output(false)
	}
}

func (e *Engine) tick(external bool) {
	e.Ticks++
	e.EnvelopeWrite = false
	for i := range e.Voices {
		e.Voices[i].Triggered = e.pending[i]
		e.pending[i] = false
	}
	for i := range e.DMA {
		e.DMA[i].Triggered = e.pendingDMA[i]
		e.pendingDMA[i] = false
	}
	if e.Playing && e.TickInRow == 0 {
		if !external || !e.rowParsed {
			e.parseRow()
		}
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
		speed := max(1, e.Speed)
		if e.TickInRow >= speed {
			e.TickInRow = 0
			e.advanceRow()
		}
	}
}

func (e *Engine) parseRow() {
	for channel := 0; channel < 3; channel++ {
		e.parse(channel, e.cell(e.Patterns[channel], e.Row), e.Mutes&(1<<channel) != 0)
	}
	if e.Project.Song.State[49] != 0 {
		if e.Patterns[3] == model.NoteOffPattern && e.Row == 0 {
			// The native preset clears both DMA lanes before testing mutes or
			// the one/two-channel mode. Ordinary PCM cells retain those tests.
			for i := range e.DMA {
				e.DMA[i].Note, e.DMA[i].Sample = 0, 0
				e.DMA[i].Triggered = true
			}
		} else {
			e.parseDMA(e.cell(e.Patterns[3], e.Row))
		}
	}
	e.rowParsed = true
}

func (e *Engine) advanceRow() {
	e.rowParsed = false
	e.Row++
	if !e.Break && e.Row < 64 {
		return
	}
	e.Break, e.Row = false, 0
	if !e.PatternMode {
		if e.PositionQueued {
			e.Position = e.NextPosition
			e.PositionQueued = false
		} else {
			e.Position++
			if e.Position >= int(e.Project.Song.Length) {
				e.Position = int(e.Project.Song.Repeat)
				e.Loops++
			}
		}
		e.loadPosition()
	}
	for channel := 0; channel < 4; channel++ {
		if e.nextPatternMask&(1<<channel) != 0 {
			e.Patterns[channel] = e.NextPatterns[channel]
		}
	}
	e.nextPatternMask = 0
}

// SelectPosition queues song jumps at a pattern boundary while Jam is playing.
// Ordinary selection is immediate and resets only the tracker row timing.
func (e *Engine) SelectPosition(position int) bool {
	if position < 0 || position >= int(e.Project.Song.Length) {
		return false
	}
	if e.Playing && e.Jam && !e.PatternMode {
		e.NextPosition, e.PositionQueued = position, true
		return true
	}
	e.Position, e.PositionQueued = position, false
	e.loadPosition()
	e.Row, e.TickInRow, e.rowParsed = 0, 0, false
	e.clockPulses = 0
	return true
}

// ClockPulse queues one external replay call. Clock selection normally sets six
// pulses per row; the native speed controller can override that live value.
// Instrument sequences and effects advance once per pulse, as in the editor.
func (e *Engine) ClockPulse() {
	if !e.ExternalClock || !e.Playing {
		return
	}
	e.clockPulses++
}

// CompensateClockLatency queues the native number of clock pulses on Start
// and Continue. Tick consumes them with synthesis triggers in the audio pass.
func (e *Engine) CompensateClockLatency() {
	if e.ExternalClock && e.Playing {
		e.clockPulses += int(e.Project.Song.State[57])
	}
}

func (e *Engine) SetSongPointer(rows int) {
	rows = max(0, rows)
	if length := int(e.Project.Song.Length); length > 0 {
		e.Position = (rows / 64) % length
		e.loadPosition()
	}
	e.Row, e.TickInRow = rows%64, 0
	e.clockPulses, e.rowParsed = 0, false
}

func (e *Engine) QueuePattern(channel int, pattern byte) {
	if channel < 0 || channel >= 4 {
		return
	}
	e.NextPatterns[channel] = pattern
	e.nextPatternMask |= 1 << channel
}

func (e *Engine) QueuedPattern(channel int) (byte, bool) {
	if channel < 0 || channel >= 4 {
		return 0, false
	}
	return e.NextPatterns[channel], e.nextPatternMask&(1<<channel) != 0
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
	if instrument > 0 {
		e.Voices[channel].ParametersDirty = true
	}
	count := e.Voices[channel].PreviewTriggers + 1
	e.parse(channel, model.Cell{Note: note, Instrument: instrument}, false)
	e.Voices[channel].PreviewTriggers = count
	e.pending[channel] = true
}

// TriggerSample previews a signed PCM bank independently of song playback.
func (e *Engine) TriggerSample(channel int, note, sample byte) {
	if channel < 0 || channel >= 2 || sample > 8 {
		return
	}
	for note >= 68 && e.Project.Song.State[49] != 4 {
		note -= 12
	}
	e.DMA[channel] = PCMVoice{Sample: sample, Note: note, PreviewTriggers: e.DMA[channel].PreviewTriggers + 1, TrackVolume: e.DMA[channel].TrackVolume, Transpose: e.DMA[channel].Transpose}
	e.pendingDMA[channel] = true
}
func (e *Engine) parse(ch int, cell model.Cell, muted bool) {
	v := &e.Voices[ch]
	if !muted {
		porta := cell.Effect1 == 'P' || cell.Effect2 == 'P'
		if cell.Instrument > 0 && cell.Instrument <= 32 {
			if v.Instrument != cell.Instrument || v.ParametersDirty {
				copy(v.Parameters[:], e.Project.Bank.Instruments[cell.Instrument-1][16:])
				v.ParametersDirty = false
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
			v.SequenceArpeggio = 0
			v.Slide, v.Porta, v.SlideRate, v.PortaRate = 0, 0, 0, 0
			if porta {
				v.Slide, v.Porta = oldSlide, oldPorta
			}
			v.Triggered = true
		}
		if cell.Note == model.NoteOff {
			*v = Voice{TrackVolume: v.TrackVolume, TrackTranspose: v.TrackTranspose, TrackNoiseTranspose: v.TrackNoiseTranspose}
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
		if e.Mutes&(1<<(i+3)) != 0 || i == 1 && e.Project.Song.State[49] != 2 && e.Project.Song.State[49] != 4 {
			continue
		}
		v := &e.DMA[i]
		if x[1] > 0 && x[1] <= 8 {
			v.Sample = x[1]
			v.Volume = 0
		}
		if x[0] == 1 {
			v.Note = 0
			v.Sample = 0
			v.Triggered = true
		} else if x[0] > 1 {
			note := x[0]
			for note >= 68 && e.Project.Song.State[49] != 4 {
				note -= 12
			}
			v.Note = note
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
	if code >= '1' && code <= '6' || strings.ContainsRune("89ACDEFGILMNQRVW", rune(code)) {
		v.ParametersDirty = true
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
		if value >= 2 && !e.ExternalClock {
			e.Speed = int(value)
		}
	case 'T':
		v.Transpose = int(int8(value))
	case 'U':
		if e.Project.Song.State[49] == 0 {
			return
		}
		switch {
		case value <= 12:
			e.SetMicrowire(0x440 | uint16(value))
		case value >= 16 && value <= 28:
			e.SetMicrowire(0x480 | uint16(value-16))
		case value >= 128 && value <= 160:
			e.SetMicrowire(0x4c0 | uint16(value-120))
		case value >= 192 && value <= 224:
			pan := int(value) - 208
			e.SetMicrowire(0x554)
			e.SetMicrowire(0x514)
			if pan < 0 {
				e.SetMicrowire(0x500 | uint16(20+pan))
			} else if pan > 0 {
				e.SetMicrowire(0x540 | uint16(20-pan))
			}
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
			if kind == 1 {
				v.SequenceArpeggio = 0
			}
			v.SeqDone[kind] = true
			continue
		}
		seq := e.Project.Bank.Sequences[number]
		length := min(int(seq.Length), len(seq.Values))
		if length <= 0 {
			v.Values[kind] = 0
			if kind == 1 {
				v.SequenceArpeggio = 0
			}
			v.SeqDone[kind] = true
			continue
		}
		index := min(v.SeqIndex[kind], length-1)
		v.Values[kind] = seq.Values[index]
		if kind == 1 {
			v.SequenceArpeggio = v.Values[kind]
		}
		index++
		if index >= length {
			index = min(int(seq.Repeat), length-1)
			if index == length-1 {
				v.SeqDone[kind] = true
			}
		}
		v.SeqIndex[kind] = index
	}
	v.Values[1] = v.SequenceArpeggio
	if v.Extra[0] != 0 || v.Extra[1] != 0 {
		switch v.ExtraStep {
		case 0:
		case 1:
			if v.Extra[0] == 15 {
				v.Values[1] += uint16(v.Extra[1])
				v.ExtraStep = -1
			} else {
				v.Values[1] += uint16(v.Extra[0])
			}
		case 2:
			v.Values[1] += uint16(v.Extra[1])
		}
		v.ExtraStep++
		v.ExtraStep %= 3
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
	period, fixed := 0, false
	if p[4]&bit != 0 {
		if p[5]&bit != 0 && p[37] == 0 {
			period, fixed = int(p[23])<<8|int(p[24]), true
		} else if v.Values[5] != 65535 {
			period, fixed = int(v.Values[5]), true
		}
	}
	if !fixed {
		adjust := 0
		if p[5]&bit != 0 && (component == 1 || p[25] == 0) {
			adjust = int(int8(p[23]))
		}
		if p[1]&bit != 0 {
			adjust += int(int16(v.Values[1]))
		}
		if p[3]&bit != 0 {
			adjust += v.Transpose + v.TrackTranspose
		}
		index := int(v.Note) + adjust
		period = int(table(tonePeriods[:], index))
		if component == 1 {
			period = int(table(envelopePeriods[:], index))
		}
		if component != 1 && p[25] != 0 {
			period = int(table(envelopePeriods[:], index)) * 16
			if p[5]&bit != 0 {
				coarse := int(int8(p[23]))
				period = period * int(table(tuningScale[:], 36-coarse)) >> 8
			}
		}
		if p[0]&bit != 0 {
			delta := int(v.Porta + v.Slide)
			if adjust >= 0 {
				scale := int(table(tuningScale[:], adjust))
				if scale != 0 {
					delta = delta * 32 / scale
				}
			} else {
				// MULS stores a long, but the original ASR.W shifts only its
				// signed low word; preserve overflow before dividing by 32.
				delta = int(int16(delta*int(table(tuningScale[:], -adjust))) >> 5)
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
		if v.Note == 0 || mixer == 0 || volume <= 0 {
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
				e.Registers[7] &^= 1 << ch
			}
			if mixer&0x1000 != 0 {
				e.Registers[7] &^= 1 << (ch + 3)
				// Native ADD.B wraps each transpose into the low byte before
				// testing its sign and clamping the YM's five-bit period.
				noise := int(int8(byte(v.Values[4]) + byte(v.NoiseTranspose) + byte(v.TrackNoiseTranspose)))
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

func (e *Engine) loadPosition() {
	length := int(e.Project.Song.Length)
	if length == 0 {
		e.Position = 0
		e.Patterns = [4]byte{255, 255, 255, 255}
		return
	}
	if e.Position < 0 || e.Position >= length {
		e.Position = min(int(e.Project.Song.Repeat), length-1)
	}
	for checked := 0; checked < length; checked++ {
		order := e.Project.Song.Orders[e.Position]
		loop := false
		for _, id := range order {
			loop = loop || id == model.LoopPattern
		}
		if !loop {
			e.Patterns = order
			return
		}
		if e.Jam {
			next := 0
			for at := e.Position - 1; at >= 0; at-- {
				for _, id := range e.Project.Song.Orders[at] {
					if id == model.LoopPattern {
						next = at + 1
						break
					}
				}
				if next != 0 {
					break
				}
			}
			e.Position = next
		} else {
			e.Position++
			if e.Position >= length {
				e.Position = int(e.Project.Song.Repeat)
				e.Loops++
			}
		}
	}
	e.Patterns = [4]byte{255, 255, 255, 255}
}
