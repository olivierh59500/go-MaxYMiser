package replay

import (
	"encoding/binary"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/ym-player/pkg/stsound"
	"io"
	"sync"
)

// Synth owns the hardware state. Its lock separates the audio callback from
// edits and transport commands without creating a new synthesizer every frame.
type Synth struct {
	reference        *stsound.CYmMusic
	referenceBuffer  []int16
	referencePlaying bool
	referenceActive  bool
	mu               sync.Mutex
	Engine           *Engine
	Chip             *stsound.CYm2149Ex
	Rate             int
	untilTick        float64
	timers           [3]timer
	pcm              [2]sampleVoice
	scratch          [1]int16
	waveform         [512]float32
	waveAt           int
	midi             midiOutputState
}
type timer struct {
	kind             byte
	phase, frequency float64
	step             int
	digi             bool
	digiSample       int
	digiPosition     float64
	digiRate         float64
	digiVolume       int
	pwmDurations     [2]float64
	pwmRemaining     float64
}
type sampleVoice struct {
	sample         int
	position, step float64
	volume         int
	active         bool
}

func NewSynth(e *Engine, rate int) *Synth {
	s := &Synth{Engine: e, Rate: rate, Chip: stsound.NewYm2149Ex(2000000, 1, stsound.YmU32(rate))}
	s.Chip.SetFilter(true)
	return s
}
func (s *Synth) Edit(fn func(*Engine)) { s.mu.Lock(); defer s.mu.Unlock(); fn(s.Engine) }
func (s *Synth) Snapshot() (Engine, [512]float32) {
	return s.SnapshotInto(nil)
}

// SnapshotInto reuses view storage while preserving a consistent, independent
// copy. It avoids allocating the whole sample bank for every rendered frame.
func (s *Synth) SnapshotInto(view *model.Project) (Engine, [512]float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := *s.Engine
	if view == nil {
		view = &model.Project{}
	}
	patterns := view.Song.Patterns
	samples := view.Bank.Samples
	*view = *s.Engine.Project
	view.Song.Patterns = append(patterns[:0], s.Engine.Project.Song.Patterns...)
	for i, sample := range s.Engine.Project.Bank.Samples {
		view.Bank.Samples[i].PCM = append(samples[i].PCM[:0], sample.PCM...)
		view.Bank.Samples[i].Trailer = append(samples[i].Trailer[:0], sample.Trailer...)
	}
	e.Project = view
	var wave [512]float32
	for i := range wave {
		wave[i] = s.waveform[(s.waveAt+i)%len(wave)]
	}
	return e, wave
}
func (s *Synth) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Engine.Reset()
	s.Chip.Reset()
	s.timers = [3]timer{}
	s.pcm = [2]sampleVoice{}
	s.untilTick = 0
}

func (s *Synth) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil && s.referenceActive {
		return s.readReference(p)
	}
	frames := len(p) / 4
	for i := 0; i < frames; i++ {
		if s.untilTick <= 0 {
			s.Engine.Tick()
			s.configure()
			s.midiTick()
			s.untilTick += float64(s.Rate) / float64(s.Engine.Project.Song.TickRate())
		}
		s.untilTick--
		s.midiSample()
		for ch := range 3 {
			s.runTimer(ch)
		}
		s.Chip.Update(s.scratch[:], 1)
		sample := int(s.scratch[0])
		for ch := range 2 {
			v := &s.pcm[ch]
			if !v.active {
				continue
			}
			pcm := s.Engine.Project.Bank.Samples[v.sample].PCM
			at := int(v.position)
			if at >= len(pcm) {
				v.active = false
				continue
			}
			sample += (int(int8(pcm[at])) >> v.volume) * 128
			v.position += v.step
		}
		sample = sample * s.Engine.MasterVolume / 127
		sample = max(-32768, min(32767, sample))
		left, right := sample, sample
		if s.Engine.Pan < 0 {
			right = right * (16 + s.Engine.Pan) / 16
		} else if s.Engine.Pan > 0 {
			left = left * (16 - s.Engine.Pan) / 16
		}
		binary.LittleEndian.PutUint16(p[i*4:], uint16(int16(left)))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(int16(right)))
		s.waveform[s.waveAt] = float32(sample) / 32768
		s.waveAt = (s.waveAt + 1) % len(s.waveform)
	}
	return frames * 4, nil
}

var _ io.Reader = (*Synth)(nil)

func (s *Synth) configure() {
	e := s.Engine
	for reg := 0; reg < 13; reg++ {
		if reg < 6 {
			v := &e.Voices[reg/2]
			kind := byte(v.Values[3] & 15)
			if (kind == 1 || kind == 15) && !(v.Triggered && v.Parameters[19] != 0) {
				continue
			}
		}
		if reg == 11 || reg == 12 {
			kind := byte(e.Voices[envelopeOwner(e)].Values[3] & 15)
			if kind == 12 || kind == 14 {
				continue
			}
		}
		if reg >= 8 && reg <= 10 && e.Registers[reg] != 0 {
			kind := byte(e.Voices[reg-8].Values[3] & 15)
			if kind == 5 || kind == 9 || kind == 13 {
				continue
			}
		}
		value := stsound.YmInt(e.Registers[reg])
		if s.Chip.ReadRegister(stsound.YmInt(reg)) != value {
			s.Chip.WriteRegister(stsound.YmInt(reg), value)
		}
	}
	owner := envelopeOwner(e)
	ownerKind := byte(e.Voices[owner].Values[3] & 15)
	if e.EnvelopeWrite && ownerKind != 11 && ownerKind != 12 {
		s.Chip.WriteRegister(13, stsound.YmInt(e.Registers[13]))
	}
	for ch := range 3 {
		v := &e.Voices[ch]
		kind := byte(v.Values[3] & 15)
		if e.TimerMask&(1<<(2-ch)) == 0 || e.Registers[8+ch] == 0 {
			kind = 0
		}
		t := &s.timers[ch]
		restarted := kind != t.kind || v.Triggered && v.Parameters[19] != 0
		if restarted {
			t.phase = 0
			t.step = 0
			t.digi = false
		}
		t.kind = kind
		wave := e.Project.Bank.Sequences[v.Parameters[38]]
		length := int(wave.Length)
		limit := 8
		switch kind {
		case 5:
			limit = 16
		case 1:
			limit = 4
		case 9:
			limit = 2
		}
		length = max(1, min(limit, length))
		period := e.Period(v, 0)
		if kind == 14 {
			period = (period + 8) &^ 15
		}
		t.frequency = timerFrequency(period, kind, length)
		if kind == 9 {
			t.pwmDurations = pwmDurations(period, v, s.Rate)
			if restarted {
				t.pwmRemaining = t.pwmDurations[0]
			}
		}
		if kind == 13 {
			sample := int(v.Parameters[20]) - 1
			if sample >= 0 && sample < 8 && v.Triggered {
				t.digiSample, t.digiPosition = sample, 0
				t.digiRate = 2457600 / float64(4*max(1, int(v.Parameters[21]))) / float64(s.Rate)
				t.digiVolume = int(e.Registers[8+ch])
				t.digi = len(e.Project.Bank.Samples[sample].PCM) > 0
			}
		} else {
			t.digi = false
		}
		if kind == 11 || kind == 12 || kind == 14 {
			if ch != owner {
				t.frequency = 0
			}
		}
		if restarted && kind == 15 && length > 0 {
			s.applyTimerValue(ch, wave.Values[0])
		}
		if restarted && kind == 12 && length > 0 {
			s.applyTimerValue(ch, wave.Values[length-1])
		}
	}
	for ch := range 2 {
		v := e.DMA[ch]
		mode := e.Project.Song.State[49]
		if mode == 0 || mode == 4 || ch == 1 && mode != 2 {
			s.pcm[ch].active = false
			continue
		}
		if v.Triggered {
			if v.Note <= 1 || v.Sample == 0 {
				s.pcm[ch].active = false
				continue
			}
			rate := pcmRate(v.Note, mode)
			volume := int(v.Volume)
			if volume > int(e.Project.Song.State[56]) || rate == 0 {
				s.pcm[ch].active = false
				continue
			}
			if mode == 2 {
				volume++
			}
			s.pcm[ch] = sampleVoice{sample: int(v.Sample) - 1, step: float64(rate) / float64(s.Rate), volume: volume, active: volume < 8}
		}
		if !v.Triggered && s.pcm[ch].active {
			volume := int(v.Volume)
			if mode == 2 {
				volume++
			}
			s.pcm[ch].volume = volume
			if int(v.Volume) > int(e.Project.Song.State[56]) || volume >= 8 {
				s.pcm[ch].active = false
			}
		}
		if e.Mutes&(1<<(ch+3)) != 0 {
			s.pcm[ch].active = false
		}
	}
}

func envelopeOwner(engine *Engine) int {
	for channel := 2; channel >= 0; channel-- {
		if engine.Voices[channel].Values[3]&0xf0 != 0 {
			return channel
		}
	}
	return 0
}

func timerFrequency(period uint16, kind byte, length int) float64 {
	if period == 0 || kind == 0 || kind == 13 {
		return 0
	}
	row := 0
	steps := 1
	if kind == 5 {
		row = max(1, min(16, length))
		steps = max(1, length)
	}
	if kind == 9 {
		row = 2
		steps = 2
	}
	divisors := [8]int{0, 4, 10, 16, 50, 64, 100, 200}
	for div := 1; div < 8; div++ {
		threshold, scale := timerDividers[row][div*2], timerDividers[row][div*2+1]
		if threshold == 0 || period > threshold {
			continue
		}
		data := (393*int(period)/(int(scale)*steps) + 1) >> 1
		if data <= 0 || data > 256 || div == 1 && data < 24 {
			return 0
		}
		return 2457600 / float64(divisors[div]*data)
	}
	return 0
}

// pwmDurations reproduces the two separately quantized MFP intervals. The
// second interval is shortened by pulse width, with the first lengthened.
func pwmDurations(period uint16, voice *Voice, rate int) [2]float64 {
	base := max(0, int(period)-4)
	width := int(voice.Parameters[17]) + int(voice.Values[6]&255)
	if !voice.PWMLocked {
		width += int(voice.PWMOffset)
	}
	width = max(0, min(255, width))
	delta := base * width >> 8
	long, short := base+delta, base-delta
	if long > 5102 {
		short += long - 5102
		long = 5102
	}
	if short < 10 {
		long += short - 10
		short = 10
	}
	var durations [2]float64
	for i, interval := range []int{long, short} {
		if interval > 0 {
			frequency := timerFrequency(uint16(interval), 9, 2)
			if frequency > 0 {
				durations[i] = float64(rate) / frequency
			}
		}
	}
	return durations
}

func (s *Synth) runTimer(ch int) {
	t := &s.timers[ch]
	if t.kind == 13 {
		level := max(0, min(15, t.digiVolume)-2)
		if t.digiVolume == 15 {
			level = 13
		}
		if t.digi {
			pcm := s.Engine.Project.Bank.Samples[t.digiSample].PCM
			at := int(t.digiPosition)
			if at >= len(pcm) {
				t.digi = false
			} else {
				level = int(digiLevels[pcm[at]])
				level = max(0, level+t.digiVolume-15)
				t.digiPosition += t.digiRate
			}
		}
		s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(level))
		return
	}
	if t.frequency <= 0 {
		return
	}
	if t.kind == 9 {
		if t.pwmDurations[0] <= 0 || t.pwmDurations[1] <= 0 {
			return
		}
		t.pwmRemaining--
		for t.pwmRemaining <= 0 {
			v := &s.Engine.Voices[ch]
			sequence := s.Engine.Project.Bank.Sequences[v.Parameters[38]]
			level := max(0, int(s.Engine.Registers[8+ch])-int(sequence.Values[t.step]))
			s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(level))
			t.step = (t.step + 1) % 2
			t.pwmRemaining += t.pwmDurations[t.step]
		}
		return
	}
	t.phase += t.frequency / float64(s.Rate)
	for t.phase >= 1 {
		t.phase--
		e := s.Engine
		v := &e.Voices[ch]
		seq := e.Project.Bank.Sequences[v.Parameters[38]]
		limit := 8
		switch t.kind {
		case 5:
			limit = 16
		case 1:
			limit = 4
		case 9:
			limit = 2
		}
		length := max(1, min(limit, int(seq.Length)))
		value := seq.Values[t.step%length]
		t.step = (t.step + 1) % length
		s.applyTimerValue(ch, value)
	}
}

func (s *Synth) applyTimerValue(ch int, value uint16) {
	e := s.Engine
	t := &s.timers[ch]
	switch t.kind {
	case 5:
		volume := max(0, int(e.Registers[8+ch])-int(value))
		s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(volume))
	case 11:
		s.Chip.WriteRegister(13, stsound.YmInt(value&15))
	case 1:
		period := uint16(e.Registers[ch*2]) | uint16(e.Registers[ch*2+1])<<8
		transpose := int(byte(value) & 127)
		if transpose != 0 {
			period = uint16(uint32(period) * uint32(table(fmScale[:], transpose)) >> 16)
		}
		s.Chip.WriteRegister(stsound.YmInt(ch*2+1), 0)
		s.Chip.WriteRegister(stsound.YmInt(ch*2), 0)
		s.Chip.WriteRegister(stsound.YmInt(ch*2), stsound.YmInt(period&255))
		s.Chip.WriteRegister(stsound.YmInt(ch*2+1), stsound.YmInt(period>>8))
	case 15, 14, 12:
		base := uint16(e.Registers[ch*2]) | uint16(e.Registers[ch*2+1])<<8
		transpose := int(byte(value) & 127)
		if t.kind == 12 {
			transpose = int(value>>8) & 127
		}
		scale := table(fmScale[:], transpose)
		if t.kind == 14 || t.kind == 12 {
			base = uint16(e.Registers[11]) | uint16(e.Registers[12])<<8
		}
		period := uint16(uint32(base) * uint32(scale) >> 16)
		if transpose == 0 {
			period = base
		}
		if t.kind == 15 {
			s.Chip.WriteRegister(stsound.YmInt(ch*2), stsound.YmInt(period&255))
			s.Chip.WriteRegister(stsound.YmInt(ch*2+1), stsound.YmInt(period>>8)&15)
		} else {
			if t.kind == 12 {
				s.Chip.WriteRegister(13, stsound.YmInt(value&15))
			}
			s.Chip.WriteRegister(11, stsound.YmInt(period&255))
			s.Chip.WriteRegister(12, stsound.YmInt(period>>8))
		}
	}
}
