package replay

// MIDIMessage has fixed storage so transport/clock/notes can be queued by the
// audio renderer without allocating or calling a system MIDI driver.
type MIDIMessage struct {
	Data [3]byte
	Size uint8
}

type midiOutputState struct {
	enabled, playing    bool
	transportGeneration uint64
	clockPhase          float64
	notes               [2]byte
	channels            [2]byte
	active              [2]bool
	queue               [2048]MIDIMessage
	read, write         uint64
	dropped             uint64
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
		s.midi.transportGeneration = s.Engine.transportGeneration
		s.midiTransport()
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
	for voice := range m.notes {
		m.release(voice)
	}
	if m.playing {
		m.push(0xfc)
	}
	m.playing = false
}

func (m *midiOutputState) release(voice int) {
	if m.active[voice] {
		m.push(0x90|m.channels[voice], m.notes[voice], 0)
	}
	m.active[voice] = false
}

func (s *Synth) midiTransport() {
	m, e := &s.midi, s.Engine
	if !m.enabled {
		return
	}
	changed := e.transportGeneration != m.transportGeneration
	if !changed && e.Playing == m.playing {
		return
	}
	m.transportGeneration = e.transportGeneration
	if !e.Playing {
		m.stop()
		return
	}
	if changed && e.transportCommand == 0xfb {
		m.push(0xfb)
	} else {
		for voice := range m.notes {
			m.release(voice)
		}
		pointer := e.Position*64 + e.Row
		if pointer == 0 {
			m.push(0xfa)
		} else {
			m.push(0xf2, byte(pointer&127), byte(pointer>>7)&127)
			m.push(0xfb)
		}
	}
	m.playing = true
	m.clockPhase = 0
}

func (s *Synth) midiNotes() {
	m, e := &s.midi, s.Engine
	if !m.enabled {
		return
	}
	if e.Project.Song.State[49] != 4 {
		for voice := range m.notes {
			m.release(voice)
		}
		return
	}
	for voice, v := range e.DMA {
		channel := e.Project.Song.State[[]int{43, 51}[voice]] & 15
		if !v.Triggered {
			continue
		}
		previous, previousChannel, previousActive := m.notes[voice], m.channels[voice], m.active[voice]
		if v.Sample > 0 || previousChannel != channel {
			m.release(voice)
			previousActive = false
		}
		active := false
		var note byte
		if v.Note >= 12 && v.Note < 128 {
			note = byte((int(v.Note) + v.Transpose) & 127)
			attenuation := max(0, int(v.Volume)+v.TrackVolume)
			velocity := byte(max(0, 127-attenuation*16))
			m.push(0x90|channel, note, velocity)
			active = velocity > 0
		}
		// With no sample number, the native routine sends the new note
		// before releasing the previous one for monophonic legato.
		if previousActive {
			m.push(0x90|previousChannel, previous, 0)
			if previous == note && previousChannel == channel {
				active = false
			}
		}
		m.notes[voice], m.channels[voice], m.active[voice] = note, channel, active
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
