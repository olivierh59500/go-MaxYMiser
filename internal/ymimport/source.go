package ymimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

// SourceScore retains the original player's identifiers. Its variable-length
// patterns are not MaxYMiser patterns and are never silently converted to them.
type SourceScore struct {
	Player      string             `json:"player"`
	SHA256      string             `json:"sha256"`
	Rate        int                `json:"rate"`
	Speed       int                `json:"initial_ticks_per_step"`
	Frames      int                `json:"decoded_source_frames"`
	Instruments []SourceInstrument `json:"instruments"`
	Patterns    []SourcePattern    `json:"patterns"`
	Orders      [3][]SourceOrder   `json:"orders"`
	Events      []SourceEvent      `json:"events"`
}

type SourceInstrument struct {
	ID             int            `json:"id"`
	Offset         int            `json:"offset"`
	Settings       []byte         `json:"settings"`
	VolumeSequence []byte         `json:"volume_sequence"`
	Arpeggio       SourceSequence `json:"arpeggio"`
}

// SourceSequence retains byte-level timing separately from the tracker row
// speed. Values are signed semitone offsets, with explicit hold/loop behavior.
type SourceSequence struct {
	Offset     int   `json:"offset"`
	StepFrames int   `json:"step_frames"`
	Values     []int `json:"values"`
	Repeat     int   `json:"repeat"`
}

type SourcePattern struct {
	ID       int             `json:"id"`
	Offset   int             `json:"offset"`
	Commands []SourceCommand `json:"commands"`
}

type SourceCommand struct {
	Offset  int    `json:"offset"`
	Opcode  byte   `json:"opcode"`
	Operand []byte `json:"operand,omitempty"`
}

type SourceOrder struct {
	Pattern   int `json:"pattern"`
	Transpose int `json:"transpose"`
}

type SourceEvent struct {
	Channel    int  `json:"channel"`
	Frame      int  `json:"frame"`
	Pattern    int  `json:"pattern"`
	Order      int  `json:"order"`
	Offset     int  `json:"source_offset"`
	NativeNote int  `json:"native_note"`
	Note       int  `json:"midi_note"`
	Instrument int  `json:"instrument"`
	Retrigger  bool `json:"retrigger"`
	Rest       bool `json:"rest,omitempty"`
}

const madMaxLastNinja = "mad-max-last-ninja-v1"

// DecodeSource recognizes a bounded, reverse-engineered player layout. Author
// names and matching filenames are insufficient to select a music decoder.
func DecodeSource(data []byte, subtune, frames int) (SourceScore, error) {
	plain, err := native.UnpackICE(data)
	if err != nil {
		return SourceScore{}, err
	}
	return decodeMadMaxLastNinja(plain, subtune, frames)
}

type sourceReader struct {
	data   []byte
	origin uint32
}

func (r sourceReader) pointer(at int) (int, error) {
	if at < 0 || at+4 > len(r.data) {
		return 0, fmt.Errorf("source: truncated pointer at %#x", at)
	}
	p := binary.BigEndian.Uint32(r.data[at:])
	if p < r.origin || uint64(p-r.origin) >= uint64(len(r.data)) {
		return 0, fmt.Errorf("source: pointer at %#x lies outside this SNDH", at)
	}
	return int(p - r.origin), nil
}

func decodeMadMaxLastNinja(data []byte, subtune, frames int) (SourceScore, error) {
	var score SourceScore
	// These instructions identify note delays and the original song initializer.
	// Absolute addresses are intentionally excluded from both signatures.
	initSignature := []byte{0x2c, 0, 0xd0, 0x40, 0xd0, 0x40, 0x26, 0x7a, 1, 0xc6, 0xd7, 0xc0}
	init := bytes.Index(data, initSignature)
	parse := bytes.Index(data, []byte{0x53, 0x28, 0, 0x1b, 0x66, 0, 1, 0x80, 0x13, 0xfc, 0, 0x2f})
	if init < 0 || parse < 0 || init-parse != 0x764 || !bytes.Contains(data[:min(len(data), 128)], []byte("SNDH")) {
		return score, fmt.Errorf("source: SNDH replay layout is not supported by a source-score decoder")
	}
	shift := init - 0x1d1a
	initAt := 0x356 + shift
	if initAt < 0 || initAt+42 > len(data) || !bytes.Equal(data[initAt:initAt+6], []byte{0x48, 0xe7, 0xff, 0xfe, 0x41, 0xfa}) {
		return score, fmt.Errorf("source: incompatible Mad Max relocation wrapper")
	}
	// The code relocation stub names the old base, then obtains the file-relative
	// base with lea (displacement,PC). No emulator addresses are required here.
	oldBase := -1
	for at := max(0, initAt-512); at+6 <= initAt; at += 2 {
		if data[at] == 0x22 && data[at+1] == 0x3c {
			oldBase = at + 2
			break
		}
	}
	if oldBase < 0 || oldBase+14 > len(data) || !bytes.Equal(data[oldBase+10:oldBase+12], []byte{0x41, 0xfa}) {
		return score, fmt.Errorf("source: missing Mad Max relocation origin")
	}
	leaTarget := oldBase + 12 + int(int16(binary.BigEndian.Uint16(data[oldBase+12:])))
	original := binary.BigEndian.Uint32(data[oldBase:])
	if leaTarget < 0 || original < uint32(leaTarget) {
		return score, fmt.Errorf("source: invalid Mad Max relocation origin")
	}
	r := sourceReader{data, original - uint32(leaTarget)}
	patternTable, err := r.pointer(0x1ee4 + shift)
	if err != nil {
		return score, err
	}
	instrumentTable, err := r.pointer(0x1ee0 + shift)
	if err != nil {
		return score, err
	}
	songTable, err := r.pointer(0x1ee8 + shift)
	if err != nil {
		return score, err
	}
	if subtune != 0 || frames < 1 || frames > 1000000 {
		return score, fmt.Errorf("source: invalid subtune or frame limit")
	}
	song, err := r.pointer(songTable + subtune*4)
	if err != nil {
		return score, err
	}
	speedAt := init + (0x1dbc - 0x1d1a) + 2 + 0x502 + subtune
	if speedAt >= len(data) || data[speedAt] == 0 {
		return score, fmt.Errorf("source: missing initial replay speed")
	}
	hash := sha256.Sum256(data)
	score = SourceScore{Player: madMaxLastNinja, SHA256: hex.EncodeToString(hash[:]), Rate: 50, Speed: int(data[speedAt]), Frames: frames}
	for id := 0; id < 32; id++ {
		at, err := r.pointer(instrumentTable + id*4)
		if err != nil || at < 6 {
			return score, fmt.Errorf("source: invalid instrument %d", id)
		}
		end := at
		for end < len(data) && end-at < 64 && data[end] != 255 {
			end++
		}
		if end == len(data) || end-at == 64 {
			return score, fmt.Errorf("source: unterminated instrument %d volume sequence", id)
		}
		instrument := SourceInstrument{ID: id, Offset: at - 6, Settings: append([]byte(nil), data[at-6:at]...), VolumeSequence: append([]byte(nil), data[at:end]...)}
		if instrument.Settings[1] < 128 {
			instrument.Arpeggio, err = decodeSourceArpeggio(r, 0x824+shift, int(instrument.Settings[1]))
			if err != nil {
				return score, fmt.Errorf("source: instrument %d arpeggio: %w", id, err)
			}
		}
		score.Instruments = append(score.Instruments, instrument)
	}
	patternIDs := map[int]bool{}
	for ch := range score.Orders {
		at, err := r.pointer(song + ch*4)
		if err != nil {
			return score, err
		}
		transpose := 0
		for steps := 0; ; steps++ {
			if steps >= 2048 || at >= len(data) {
				return score, fmt.Errorf("source: unterminated channel %d order list", ch)
			}
			v := data[at]
			at++
			if v == 255 || v == 254 {
				if v == 254 {
					return score, fmt.Errorf("source: stop command in channel %d is not yet supported", ch)
				}
				break
			}
			if v == 128 || v == 253 {
				if at >= len(data) {
					return score, fmt.Errorf("source: truncated order control")
				}
				if v == 128 {
					transpose = int(int8(data[at]))
				}
				at++
				continue
			}
			if v > 127 {
				return score, fmt.Errorf("source: unknown order command %#x", v)
			}
			score.Orders[ch] = append(score.Orders[ch], SourceOrder{int(v), transpose})
			patternIDs[int(v)] = true
		}
	}
	for id := 0; id < 128; id++ {
		if !patternIDs[id] {
			continue
		}
		at, err := r.pointer(patternTable + id*4)
		if err != nil {
			return score, err
		}
		pattern := SourcePattern{ID: id, Offset: at}
		for count := 0; ; count++ {
			if count >= 4096 || at >= len(data) {
				return score, fmt.Errorf("source: unterminated pattern %d", id)
			}
			op := data[at]
			if op == 0x83 {
				return score, fmt.Errorf("source: alternate instrument-bank command at %#x is not yet supported", at)
			}
			if op == 0x85 || op == 0x86 {
				return score, fmt.Errorf("source: per-step pitch command at %#x is not yet supported", at)
			}
			length, ok := sourceOperandLength(op)
			if !ok || at+1+length > len(data) {
				return score, fmt.Errorf("source: invalid pattern %d command %#x at %#x", id, op, at)
			}
			pattern.Commands = append(pattern.Commands, SourceCommand{at, op, append([]byte(nil), data[at+1:at+1+length]...)})
			at += 1 + length
			if op == 0x87 {
				break
			}
		}
		score.Patterns = append(score.Patterns, pattern)
	}
	score.Events, err = sourceTimeline(score, frames)
	return score, err
}

func sourceOperandLength(op byte) (int, bool) {
	if op < 128 || op >= 0xb8 {
		return 0, true
	}
	switch op {
	case 0x80, 0x81, 0x82, 0x83, 0x87, 0x8a, 0x8b, 0x8d, 0x8e, 0x8f:
		return 0, true
	case 0x85, 0x86, 0x89, 0x8c, 0x90, 0x91:
		return 1, true
	case 0x84:
		return 2, true
	case 0x88, 0x92:
		return 3, true
	}
	return 0, false
}

type sourceVoice struct {
	order, command, delay, duration, instrument int
	pattern                                     SourcePattern
	legato                                      bool
}

func sourceTimeline(score SourceScore, frames int) ([]SourceEvent, error) {
	patterns := map[int]SourcePattern{}
	for _, p := range score.Patterns {
		patterns[p.ID] = p
	}
	var voices [3]sourceVoice
	for ch := range voices {
		if len(score.Orders[ch]) == 0 {
			return nil, fmt.Errorf("source: channel %d has no patterns", ch)
		}
		voices[ch] = sourceVoice{delay: 1, duration: 1, instrument: -1, pattern: patterns[score.Orders[ch][0].Pattern]}
	}
	speed, countdown := score.Speed, 1
	var events []SourceEvent
	for frame := 0; frame < frames; frame++ {
		countdown--
		if countdown != 0 {
			continue
		}
		for ch := range voices {
			v := &voices[ch]
			v.delay--
			if v.delay != 0 {
				continue
			}
			v.legato = false
			for count := 0; ; count++ {
				if count >= 4096 || v.command >= len(v.pattern.Commands) {
					return nil, fmt.Errorf("source: invalid channel %d command flow", ch)
				}
				c := v.pattern.Commands[v.command]
				v.command++
				op := c.Opcode
				if op < 128 || op == 0x80 || op == 0x90 {
					if op < 128 {
						note := int(op) + score.Orders[ch][v.order].Transpose
						if v.instrument >= 0 && v.instrument < len(score.Instruments) && score.Instruments[v.instrument].Settings[0]&2 != 0 {
							return nil, fmt.Errorf("source: fixed-pitch sample instrument %d is not yet supported", v.instrument)
						}
						events = append(events, SourceEvent{Channel: ch, Frame: frame, Pattern: v.pattern.ID, Order: v.order, Offset: c.Offset, NativeNote: int(op), Note: note + 12, Instrument: v.instrument, Retrigger: !v.legato})
					} else {
						events = append(events, SourceEvent{Channel: ch, Frame: frame, Pattern: v.pattern.ID, Order: v.order, Offset: c.Offset, Instrument: v.instrument, Rest: true})
					}
					v.delay = v.duration
					break
				}
				switch {
				case op == 0x87:
					v.order = (v.order + 1) % len(score.Orders[ch])
					v.pattern, v.command = patterns[score.Orders[ch][v.order].Pattern], 0
				case op == 0x8e:
					v.legato = true
				case op == 0x91:
					v.duration = int(c.Operand[0]) + 1
				case op >= 0xe0:
					v.duration = int(op-0xe0) + 1
				case op >= 0xc0:
					v.instrument = int(op - 0xc0)
				case op >= 0xb8:
					speed = int(op-0xb8) + 1
				}
			}
		}
		countdown = speed
	}
	return events, nil
}
