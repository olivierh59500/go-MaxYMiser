package replay

import (
	"encoding/binary"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

type Reference struct {
	Active               bool
	Name, Author, Format string
	Position, Duration   uint32
	Registers            [14]byte
	Playing              bool
}

func (s *Synth) LoadYM(data []byte) error {
	music := stsound.NewYmMusic(s.Rate)
	if err := music.LoadMemory(data); err != nil {
		return err
	}
	music.SetLoopMode(true)
	music.Play()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.UnLoad()
	}
	s.Engine.Stop()
	s.reference = music
	s.referencePlaying = true
	s.referenceActive = true
	return nil
}
func (s *Synth) CloseYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.UnLoad()
		s.reference = nil
	}
	s.referencePlaying = false
	s.referenceActive = false
}
func (s *Synth) ToggleYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return
	}
	s.referencePlaying = !s.referencePlaying
	if s.referencePlaying {
		s.reference.Play()
	} else {
		s.reference.Pause()
	}
}
func (s *Synth) StopYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.Stop()
	}
	s.referencePlaying = false
}
func (s *Synth) SeekYM(milliseconds uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.SetMusicTime(stsound.YmU32(milliseconds))
	}
}
func (s *Synth) Reference() (Reference, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return Reference{}, false
	}
	info := s.reference.GetMusicInfo()
	r := Reference{Name: info.SongName, Author: info.SongAuthor, Format: info.SongType, Position: uint32(s.reference.GetPos()), Duration: uint32(s.reference.GetMusicTime()), Playing: s.referencePlaying, Active: s.referenceActive}
	for reg := range r.Registers {
		r.Registers[reg] = byte(s.reference.ReadYmRegister(reg))
	}
	return r, true
}
func (s *Synth) readReference(p []byte) (int, error) {
	frames := len(p) / 4
	if cap(s.referenceBuffer) < frames {
		s.referenceBuffer = make([]int16, frames)
	}
	buffer := s.referenceBuffer[:frames]
	s.reference.Update(buffer, frames)
	for i, sample := range buffer {
		binary.LittleEndian.PutUint16(p[i*4:], uint16(sample))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(sample))
		s.waveform[s.waveAt] = float32(sample) / 32768
		s.waveAt = (s.waveAt + 1) % len(s.waveform)
	}
	return frames * 4, nil
}

// SelectReference keeps the original YM loaded while switching the audible source.
func (s *Synth) SelectReference(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return
	}
	s.referenceActive = active
	if active {
		s.Engine.Stop()
		s.reference.SetMusicTime(0)
		s.reference.Play()
		s.referencePlaying = true
	} else {
		s.reference.Pause()
		s.referencePlaying = false
		s.Engine.Reset()
		s.Engine.Play(false)
	}
}

func (s *Synth) PreviewInstrument(channel int, note, instrument byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.Pause()
		s.referencePlaying = false
		s.referenceActive = false
	}
	s.Engine.Trigger(channel, note, instrument)
}
