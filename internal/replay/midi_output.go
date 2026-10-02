package replay

// MIDIMessage has fixed storage so transport/clock/notes can be queued by the
// audio renderer without allocating or calling a system MIDI driver.
type MIDIMessage struct {
	Data [3]byte
	Size uint8
}

type midiOutputState struct {
	enabled, playing bool
	clockPhase       float64
	notes            [2]byte
	channels         [2]byte
	queue            [256]MIDIMessage
	read, write      uint64
	dropped          uint64
}

func (s *Synth) EnableMIDIOutput(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !enabled && s.midi.enabled {
		s.midi.stop()
	}
	s.midi.enabled = enabled
	if enabled {
		s.midi.playing = false
		s.midi.clockPhase = 0
	}
}

func (m *midiOutputState) push(data ...byte) {
	if m.write-m.read >= uint64(len(m.queue)) {
		m.dropped++
		return
	}
	message := MIDIMessage{Size: uint8(len(data))}
	copy(message.Data[:], data)
	m.queue[m.write%uint64(len(m.queue))] = message
	m.write++
}

func (m *midiOutputState) stop() {
	for voice, note := range m.notes {
		if note > 1 {
			m.push(0x80|m.channels[voice], note, 0)
		}
		m.notes[voice] = 0
	}
	if m.playing {
		m.push(0xfc)
	}
	m.playing = false
}

func (s *Synth) midiTick() {
	m, e := &s.midi, s.Engine
	if !m.enabled {
		return
	}
	if e.Playing != m.playing {
		if e.Playing {
			pointer := e.Position*64 + e.Row
			m.push(0xf2, byte(pointer&127), byte(pointer>>7)&127)
			m.push(0xfa)
			m.clockPhase = 0
		} else {
			m.stop()
		}
		m.playing = e.Playing
	}
	if e.Project.Song.State[49] != 4 {
		for voice, note := range m.notes {
			if note > 1 {
				m.push(0x80|m.channels[voice], note, 0)
				m.notes[voice] = 0
			}
		}
		return
	}
	for voice, v := range e.DMA {
		channel := e.Project.Song.State[[]int{43, 51}[voice]] & 15
		if !v.Triggered {
			continue
		}
		if m.notes[voice] > 1 {
			m.push(0x80|m.channels[voice], m.notes[voice], 0)
			m.notes[voice] = 0
		}
		if v.Note > 1 && v.Sample > 0 {
			m.push(0xc0|channel, (v.Sample-1)&127)
			velocity := byte(max(1, 127-int(v.Volume)*8))
			m.push(0x90|channel, v.Note&127, velocity)
			m.notes[voice], m.channels[voice] = v.Note&127, channel
		}
	}
}

func (s *Synth) midiSample() {
	if !s.midi.enabled || !s.Engine.Playing || s.Engine.ExternalClock {
		return
	}
	frequency := float64(s.Engine.Project.Song.TickRate()) * 6 / float64(max(1, s.Engine.Speed))
	s.midi.clockPhase += frequency / float64(s.Rate)
	for s.midi.clockPhase >= 1 {
		s.midi.clockPhase--
		s.midi.push(0xf8)
	}
}

// DrainMIDI is called outside the audio callback. Its count is bounded by the
// destination buffer; dropped events are reported rather than hidden.
func (s *Synth) DrainMIDI(dst []MIDIMessage) (int, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for count < len(dst) && s.midi.read < s.midi.write {
		dst[count] = s.midi.queue[s.midi.read%uint64(len(s.midi.queue))]
		s.midi.read++
		count++
	}
	return count, s.midi.dropped
}
