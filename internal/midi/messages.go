// Package midi translates MIDI 1.0 notes, controllers and transport messages.
package midi

import "github.com/olivierh59500/go-MaxYMiser/internal/replay"

type Decoder struct {
	Status byte
	Data   [2]byte
	Count  int
	Sysex  []byte
}

func (d *Decoder) Feed(data []byte, emit func([]byte)) {
	for _, b := range data {
		if b >= 0xf8 {
			emit([]byte{b})
			continue
		}
		if len(d.Sysex) > 0 {
			d.Sysex = append(d.Sysex, b)
			if b == 0xf7 {
				emit(d.Sysex)
				d.Sysex = nil
			}
			if len(d.Sysex) > 4096 {
				d.Sysex = nil
			}
			continue
		}
		if b == 0xf0 {
			d.Sysex = []byte{b}
			d.Status = 0
			continue
		}
		if b&128 != 0 {
			d.Status = b
			d.Count = 0
			if b == 0xf6 || b == 0xf7 {
				emit([]byte{b})
				d.Status = 0
			}
			continue
		}
		if d.Status == 0 {
			continue
		}
		d.Data[d.Count] = b
		d.Count++
		length := 2
		if d.Status&0xf0 == 0xc0 || d.Status&0xf0 == 0xd0 {
			length = 1
		}
		if d.Status == 0xf1 || d.Status == 0xf3 {
			length = 1
		}
		if d.Count == length {
			message := append([]byte{d.Status}, d.Data[:length]...)
			emit(message)
			d.Count = 0
			if d.Status >= 0xf0 {
				d.Status = 0
			}
		}
	}
}
func Apply(e *replay.Engine, message []byte) {
	if len(message) == 0 {
		return
	}
	status := message[0]
	if status >= 0xf8 {
		// The native input ignores clock/transport in internal and Sync24
		// modes. Notes, controllers and MMC remain independent of the clock.
		if !e.ExternalClock || e.Project.Song.State[31]&2 != 0 {
			return
		}
		switch status {
		case 0xf8:
			e.ClockPulse()
		case 0xfa:
			e.Reset()
			e.Play(false)
			e.CompensateClockLatency()
		case 0xfb:
			e.Continue()
			e.CompensateClockLatency()
		case 0xfc:
			e.Stop()
		}
		return
	}
	if status == 0xf2 {
		if len(message) == 3 && e.ExternalClock && e.Project.Song.State[31]&2 == 0 {
			e.SetSongPointer(int(message[1]&127) | int(message[2]&127)<<7)
		}
		return
	}
	if status == 0xf0 {
		if command, ok := MachineControl(message); ok {
			switch command {
			case 1:
				e.Stop()
			case 2:
				e.Play(false)
			}
		}
		return
	}
	channel := int(status & 15)
	if channel >= 5 {
		return
	}
	if channel >= 3 {
		ApplyPCM(e, channel-3, message)
		return
	}
	switch status & 0xf0 {
	case 0x90:
		if len(message) >= 3 {
			if message[2] == 0 {
				e.Trigger(channel, 1, 0)
			} else {
				instrument := e.Voices[channel].Instrument
				if instrument == 0 {
					instrument = 1
				}
				e.Trigger(channel, message[1], instrument)
				e.Voices[channel].ColumnVolume = 15 - int(message[2]>>3)
			}
		}
	case 0x80:
		if len(message) >= 3 {
			if e.Voices[channel].Note == message[1] {
				e.Trigger(channel, 1, 0)
			}
		}
	case 0xc0:
		if len(message) >= 2 {
			e.Voices[channel].Instrument = 0
			copy(e.Voices[channel].Parameters[:], e.Project.Bank.Instruments[int(message[1])%32][16:])
			e.Voices[channel].Instrument = message[1]%32 + 1
		}
	case 0xb0:
		if len(message) >= 3 && e.Project.Song.State[31]&4 != 0 {
			controller(e, channel, message[1], message[2])
		}
	}
}

func ApplyPCM(e *replay.Engine, channel int, message []byte) {
	if channel < 0 || channel >= 2 || len(message) < 2 {
		return
	}
	switch message[0] & 0xf0 {
	case 0x90:
		if len(message) < 3 {
			return
		}
		if message[2] == 0 {
			e.TriggerSample(channel, 1, 0)
		} else {
			slot := 35
			if channel == 1 {
				slot = 50
			}
			sample := e.Project.Song.State[slot]
			e.TriggerSample(channel, message[1], sample)
			e.DMA[channel].Volume = byte(7 - int(message[2]>>4))
		}
	case 0x80:
		note := message[1]
		for note >= 68 && e.Project.Song.State[49] != 4 {
			note -= 12
		}
		if e.DMA[channel].Note == note {
			e.TriggerSample(channel, 1, 0)
		}
	case 0xc0:
		slot := 35
		if channel == 1 {
			slot = 50
		}
		e.Project.Song.State[slot] = message[1]%8 + 1
	}
}

// ApplyMapped uses the five native MIDI channel assignments. Duplicate
// assignments select a free voice first, then reuse one deterministically.
func ApplyMapped(e *replay.Engine, message []byte) {
	if len(message) == 0 || message[0] >= 0xf0 {
		Apply(e, message)
		return
	}
	channel := message[0] & 15
	status := message[0] & 0xf0
	if status == 0xb0 {
		if len(message) < 3 || e.Project.Song.State[31]&4 == 0 {
			return
		}
		if message[1] >= 16 && message[1] <= 51 {
			controller(e, 0, message[1], message[2])
			return
		}
	}
	var tracks []int
	for track, offset := range []int{40, 41, 42, 43, 51} {
		if e.Project.Song.State[offset]&15 != channel {
			continue
		}
		instOffset := []int{32, 33, 34, 35, 50}[track]
		if e.Project.Song.State[instOffset] != 0 {
			tracks = append(tracks, track)
		}
	}
	if len(tracks) == 0 {
		return
	}
	if status == 0x90 && len(message) >= 3 && message[2] > 0 {
		chosen := tracks[0]
		for _, track := range tracks {
			free := track < 3 && e.Voices[track].Note <= 1 || track >= 3 && e.DMA[track-3].Note <= 1
			if free {
				chosen = track
				break
			}
		}
		tracks = []int{chosen}
	}
	for _, track := range tracks {
		mapped := append([]byte(nil), message...)
		mapped[0] = status | byte(track)
		if track < 3 {
			if e.Project.Song.State[32+track] == 0xdd && (status == 0x80 || status == 0x90 && len(mapped) >= 3 && mapped[2] == 0) {
				if e.Voices[track].Instrument == mapped[1]%32+1 {
					e.Trigger(track, 1, 0)
				}
				continue
			}
			if status == 0x90 && len(mapped) >= 3 && mapped[2] > 0 {
				note, instrument := mapped[1], e.Project.Song.State[32+track]
				if instrument == 0xdd {
					instrument, note = note%32+1, 60
				}
				if instrument <= 32 {
					e.Trigger(track, note, instrument)
				}
				e.Voices[track].ColumnVolume = 15 - int(mapped[2]>>3)
			} else {
				Apply(e, mapped)
				if status == 0xc0 && len(mapped) >= 2 {
					e.Project.Song.State[32+track] = mapped[1]%32 + 1
				}
			}
		} else {
			Apply(e, mapped)
		}
	}
}
func controller(e *replay.Engine, ch int, code, value byte) {
	if code >= 60 {
		instrumentController(e, ch, code, value)
		return
	}
	switch {
	case code >= 16 && code <= 20:
		bit := byte(1 << (code - 16))
		if value >= 32 {
			e.Mutes &^= bit
		} else {
			e.Mutes |= bit
		}
	case code == 21:
		e.Speed = int(value>>3) + 2
	case code == 22:
		e.SelectPosition(int(value))
	case code == 23:
		e.Jam = value != 0
	case code == 24:
		e.PatternMode = value != 0
	case code == 25:
		e.Row = int(value >> 1)
		e.TickInRow = 0
	case code == 28:
		for i := range e.Voices {
			e.Voices[i].TrackNoiseTranspose = -((int(value) - 64) >> 1)
		}
	case code >= 29 && code <= 31:
		e.Voices[code-29].TrackNoiseTranspose = -((int(value) - 64) >> 1)
	case code == 32:
		for i := range e.Voices {
			e.Voices[i].TrackTranspose = (int(value) - 64) >> 1
		}
	case code >= 33 && code <= 35:
		e.Voices[code-33].TrackTranspose = (int(value) - 64) >> 1
	case code == 36 || code == 37:
		e.DMA[code-36].Transpose = (int(value) - 64) >> 1
	case code == 38:
		for i := range e.Voices {
			e.Voices[i].TrackVolume = 15 - int(value>>3)
		}
		for i := range e.DMA {
			e.DMA[i].TrackVolume = 8 - int(value/15)
		}
	case code >= 39 && code <= 41:
		e.Voices[code-39].TrackVolume = 15 - int(value>>3)
	case code == 42 || code == 43:
		e.DMA[code-42].TrackVolume = 8 - int(value/15)
	case code >= 44 && code <= 47:
		pattern := value
		if pattern >= 126 {
			pattern += 128
		}
		e.QueuePattern(int(code-44), pattern)
	case code == 48:
		e.SetMicrowire(0x4c0 | uint16(int(value)*41/128))
	case code == 49:
		if value < 64 {
			e.SetMicrowire(0x500 | uint16(int(value)*21/64))
			e.SetMicrowire(0x554)
		} else {
			e.SetMicrowire(0x540 | uint16((127-int(value))*21/64))
			e.SetMicrowire(0x514)
		}
	case code == 50:
		e.SetMicrowire(0x440 | uint16(value/10))
	case code == 51:
		e.SetMicrowire(0x480 | uint16(value/10))
	}
}
