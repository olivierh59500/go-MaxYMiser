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
		if d.Count == length {
			message := append([]byte{d.Status}, d.Data[:length]...)
			emit(message)
			d.Count = 0
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
	if channel >= 3 {
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
		e.Patterns[code-44] = value
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
