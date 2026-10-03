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
	Subtune, Subtunes    int
	Effects              bool
	Seeking              bool
	Error                string
}

type referenceSource interface {
	Read([]byte) (int, error)
	Info() Reference
	SetPlaying(bool)
	Seek(uint32) error
	Close()
}

type ymReference struct {
	music  *stsound.CYmMusic
	buffer []int16
}

func (r *ymReference) Read(p []byte) (int, error) {
	frames := len(p) / 4
	if cap(r.buffer) < frames {
		r.buffer = make([]int16, frames)
	}
	r.music.Update(r.buffer[:frames], frames)
	for i, sample := range r.buffer[:frames] {
		binary.LittleEndian.PutUint16(p[i*4:], uint16(sample))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(sample))
	}
	return frames * 4, nil
}
func (r *ymReference) Info() Reference {
	info := r.music.GetMusicInfo()
	out := Reference{Name: info.SongName, Author: info.SongAuthor, Format: info.SongType, Position: uint32(r.music.GetPos()), Duration: uint32(r.music.GetMusicTime()), Subtune: 1, Subtunes: 1}
	for reg := range out.Registers {
		out.Registers[reg] = byte(r.music.ReadYmRegister(reg))
	}
	return out
}
func (r *ymReference) SetPlaying(on bool) {
	if on {
		r.music.Play()
	} else {
		r.music.Pause()
	}
}
func (r *ymReference) Seek(ms uint32) error { r.music.SetMusicTime(stsound.YmU32(ms)); return nil }
func (r *ymReference) Close()               { r.music.UnLoad() }

func (s *Synth) LoadYM(data []byte) error {
	music := stsound.NewYmMusic(s.Rate)
	if err := music.LoadMemory(data); err != nil {
		return err
	}
	music.SetLoopMode(true)
	music.Play()
	s.replaceReference(&ymReference{music: music})
	return nil
}
func (s *Synth) replaceReference(ref referenceSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.Close()
	}
	s.Engine.Stop()
	s.reference = ref
	s.referencePlaying = true
	s.referenceActive = true
	s.referenceError = ""
}
func (s *Synth) CloseYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.Close()
		s.reference = nil
	}
	s.referencePlaying, s.referenceActive = false, false
	s.referenceError = ""
}
func (s *Synth) ToggleYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return
	}
	s.referencePlaying = !s.referencePlaying
	s.reference.SetPlaying(s.referencePlaying)
}
func (s *Synth) StopYM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.SetPlaying(false)
	}
	s.referencePlaying = false
}

// PauseReference returns to the current native transport without restarting it.
func (s *Synth) PauseReference() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.SetPlaying(false)
	}
	s.referencePlaying, s.referenceActive = false, false
}
func (s *Synth) SeekYM(ms uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.seekReferenceLocked(ms)
	}
}

// The audio mutex owns the audible renderer. An SNDH seek prepares an
// independent renderer outside this lock and replaces it only when complete.
func (s *Synth) seekReferenceLocked(ms uint32) bool {
	if ref, ok := s.reference.(*sndhReference); ok {
		s.startSNDHSeekLocked(ref, ms)
		return ref.seeking
	}
	if err := s.reference.Seek(ms); err != nil {
		s.referenceError = err.Error()
		return false
	}
	return true
}
func (s *Synth) Reference() (Reference, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return Reference{}, false
	}
	r := s.reference.Info()
	r.Active = s.referenceActive
	r.Playing = s.referencePlaying
	r.Error = s.referenceError
	return r, true
}
func (s *Synth) readReference(p []byte) (int, error) {
	if !s.referencePlaying {
		clear(p)
		return len(p), nil
	}
	n, err := s.reference.Read(p)
	if err != nil {
		s.referenceError = err.Error()
		s.referencePlaying = false
		clear(p[n:])
		n = len(p)
	}
	for i := 0; i+4 <= n; i += 4 {
		sample := int16(binary.LittleEndian.Uint16(p[i:]))
		s.waveform[s.waveAt] = float32(sample) / 32768
		s.waveAt = (s.waveAt + 1) % len(s.waveform)
	}
	return n, nil
}

// SelectReference switches the audible source while preserving the reference.
func (s *Synth) SelectReference(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference == nil {
		return
	}
	s.referenceActive = active
	if active {
		s.Engine.Stop()
		if !s.seekReferenceLocked(0) {
			return
		}
		s.reference.SetPlaying(true)
		s.referencePlaying = true
	} else {
		s.reference.SetPlaying(false)
		s.referencePlaying = false
		s.Engine.Reset()
		s.Engine.Play(false)
	}
}
func (s *Synth) PreviewInstrument(channel int, note, instrument byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reference != nil {
		s.reference.SetPlaying(false)
		s.referencePlaying = false
		s.referenceActive = false
	}
	s.Engine.Trigger(channel, note, instrument)
}
