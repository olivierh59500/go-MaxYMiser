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
		switch status {
		case 0xf8:
			e.ClockPulse()
		case 0xfa:
			e.Reset()
			e.Play(false)
		case 0xfb:
			e.Playing = true
		case 0xfc:
			e.Stop()
		}
		return
	}
	if status == 0xf2 {
		if len(message) == 3 {
			e.SetSongPointer(int(message[1]&127) | int(message[2]&127)<<7)
		}
		return
	}
	if status == 0xf0 {
		if len(message) >= 6 && message[1] == 0x7f && message[3] == 6 {
			switch message[4] {
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
		if len(message) >= 3 {
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
		if status == 0xb0 && len(message) >= 3 && message[1] >= 16 && message[1] <= 51 {
			controller(e, 0, message[1], message[2])
		}
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
			if status == 0x90 && len(mapped) >= 3 && mapped[2] > 0 {
				note, instrument := mapped[1], e.Project.Song.State[32+track]
				if instrument == 255 {
					instrument, note = note%32+1, 48
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
	v := &e.Voices[ch]
	switch {
	case code >= 16 && code <= 20:
		bit := byte(1 << (code - 16))
		if value == 0 {
			e.Mutes &^= bit
		} else {
			e.Mutes |= bit
		}
	case code == 21:
		e.Speed = max(2, int(value))
	case code == 22:
		if value < e.Project.Song.Length {
			e.Position = int(value)
			e.Patterns = e.Project.Song.Orders[value]
			e.Row = 0
			e.TickInRow = 0
		}
	case code == 23:
		e.Jam = value != 0
	case code == 24:
		e.PatternMode = value != 0
	case code == 25:
		e.Row = int(value) % 64
		e.TickInRow = 0
	case code == 28:
		for i := range e.Voices {
			e.Voices[i].NoiseTranspose = int(value) - 64
		}
	case code >= 29 && code <= 31:
		e.Voices[code-29].NoiseTranspose = int(value) - 64
	case code == 32:
		for i := range e.Voices {
			e.Voices[i].Transpose = int(value) - 64
		}
	case code >= 33 && code <= 35:
		e.Voices[code-33].Transpose = int(value) - 64
	case code == 38:
		e.MasterVolume = int(value)
	case code >= 39 && code <= 41:
		e.Voices[code-39].TrackVolume = 15 - int(value>>3)
	case code >= 44 && code <= 47:
		e.QueuePattern(int(code-44), value)
	case code >= 60 && code <= 67:
		offset := int(code-60) + 32
		if code == 67 {
			offset = 39
		}
		v.Parameters[offset] = value
		v.SeqIndex = [7]int{}
		v.SeqDone = [7]bool{}
	case code == 109:
		v.Parameters[22] = value >> 3
	case code == 110:
		v.Parameters[16] = value
	case code == 111:
		v.Parameters[20] = value % 9
	case code == 112:
		v.Parameters[21] = value
	case code == 113:
		v.Parameters[18] = value & 15
	case code == 114:
		v.Parameters[19] = value
	case code == 115:
		v.Parameters[23] = value - 64
	case code == 116:
		v.Parameters[24] = value - 64
	case code == 117:
		v.Parameters[17] = value * 2
	case code == 48:
		e.MasterVolume = int(value)
	case code == 49:
		e.Pan = (int(value) - 64) / 4
	case code == 50:
		e.Bass = int(value) * 12 / 127
	case code == 51:
		e.Treble = int(value) * 12 / 127
	}
}
