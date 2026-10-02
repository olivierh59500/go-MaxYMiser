package replay

import (
	"encoding/binary"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/ym-player/pkg/stsound"
	"io"
	"math"
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
}
type timer struct {
	kind             byte
	phase, frequency float64
	step             int
	digi             bool
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
			s.untilTick += float64(s.Rate) / float64(s.Engine.Project.Song.TickRate())
		}
		s.untilTick--
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
			sample += int(int8(pcm[at])) * (15 - v.volume) * 7
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
		value := stsound.YmInt(e.Registers[reg])
		if s.Chip.ReadRegister(stsound.YmInt(reg)) != value {
			s.Chip.WriteRegister(stsound.YmInt(reg), value)
		}
	}
	if e.EnvelopeWrite {
		s.Chip.WriteRegister(13, stsound.YmInt(e.Registers[13]))
	}
	for ch := range 3 {
		v := &e.Voices[ch]
		kind := byte(v.Values[3] & 15)
		if e.TimerMask&(1<<(2-ch)) == 0 || e.Registers[8+ch] == 0 {
			kind = 0
		}
		t := &s.timers[ch]
		if kind != t.kind || v.Triggered && v.Parameters[19] != 0 {
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
		t.frequency = timerFrequency(period, kind, length)
		if kind == 13 {
			sample := int(v.Parameters[20]) - 1
			if sample >= 0 && sample < 8 && v.Triggered {
				pcm := e.Project.Bank.Samples[sample].PCM
				drum := make([]stsound.YmU8, len(pcm))
				// Convert signed PCM to the chip's four-bit logarithmic DAC scale.
				for i, b := range pcm {
					amplitude := int(int8(b)) + 128
					drum[i] = stsound.YmU8(amplitude*15/255) * 17
				}
				rate := 2457600 / (4 * max(1, int(v.Parameters[21])))
				s.Chip.DrumStart(stsound.YmInt(ch), drum, stsound.YmU32(len(drum)), stsound.YmInt(rate))
				t.digi = true
			}
		} else {
			s.Chip.DrumStop(stsound.YmInt(ch))
		}
	}
	for ch := range 2 {
		v := e.DMA[ch]
		if v.Triggered {
			if v.Note <= 1 || v.Sample == 0 {
				s.pcm[ch].active = false
				continue
			}
			rate := float64(8287) * math.Pow(2, (float64(v.Note)-60)/12)
			s.pcm[ch] = sampleVoice{sample: int(v.Sample) - 1, step: rate / float64(s.Rate), volume: int(v.Volume), active: true}
		}
		if e.Mutes&(1<<(ch+3)) != 0 {
			s.pcm[ch].active = false
		}
	}
}

func timerFrequency(period uint16, kind byte, length int) float64 {
	if period == 0 || kind == 0 || kind == 13 {
		return 0
	}
	row := 0
	steps := 1
	if kind == 5 {
		row = max(2, min(16, length))
		steps = max(2, length)
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

func (s *Synth) runTimer(ch int) {
	t := &s.timers[ch]
	if t.frequency <= 0 {
		return
	}
	if t.kind == 9 {
		t.phase += t.frequency / 2 / float64(s.Rate)
		t.phase -= math.Floor(t.phase)
		v := s.Engine.Voices[ch]
		width := int(v.Parameters[17]) + int(v.Values[6]&255)
		if !v.PWMLocked {
			width += int(v.PWMOffset)
		}
		duty := float64(256-max(-255, min(255, width))) / 512
		volume := int(s.Engine.Registers[8+ch])
		if t.phase >= duty {
			volume = 0
		}
		if s.Chip.ReadRegister(stsound.YmInt(8+ch)) != stsound.YmInt(volume) {
			s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(volume))
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
		switch t.kind {
		case 5:
			volume := max(0, int(e.Registers[8+ch])-int(value))
			s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(volume))
		case 9:
			level := int(e.Registers[8+ch])
			if t.step%2 == 0 {
				level = 0
			}
			s.Chip.WriteRegister(stsound.YmInt(8+ch), stsound.YmInt(level))
		case 11:
			s.Chip.WriteRegister(13, stsound.YmInt(value&15))
		case 1:
			period := uint16(e.Registers[ch*2]) | uint16(e.Registers[ch*2+1])<<8
			s.Chip.WriteRegister(stsound.YmInt(ch*2), 0)
			s.Chip.WriteRegister(stsound.YmInt(ch*2+1), 0)
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
				s.Chip.WriteRegister(11, stsound.YmInt(period&255))
				s.Chip.WriteRegister(12, stsound.YmInt(period>>8))
			}
			if t.kind == 12 {
				s.Chip.WriteRegister(13, stsound.YmInt(value&15))
			}
		}
	}
}
