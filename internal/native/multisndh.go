package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type multiSlot struct {
	instruction, ratePointer int
	version                  byte
}

// MultiSNDHTemplate retains a verified native selector and its common replayer.
// Every original slot must have an editable bank/song; mixed-player wrappers
// and missing songs cannot be exported through this layout.
type MultiSNDHTemplate struct {
	Prefix   []byte
	pointers int
	slots    []multiSlot
}

func ParseMultiSNDHTemplate(data []byte) (MultiSNDHTemplate, error) {
	var template MultiSNDHTemplate
	plain, err := UnpackICE(data)
	if err != nil {
		return template, err
	}
	projects, err := DecodeContainers(plain)
	if err != nil {
		return template, err
	}
	count := DeclaredSubtunes(plain)
	if count < 2 || count != len(projects) {
		return template, fmt.Errorf("native: complete native multi-song source required")
	}
	for at := 16; at+10 <= min(len(plain), 2048); at += 2 {
		if !bytes.Equal(plain[at:at+2], []byte{0x41, 0xfa}) || !bytes.Equal(plain[at+4:at+6], []byte{0xd1, 0xfc}) {
			continue
		}
		base := int64(at+2) + int64(int16(binary.BigEndian.Uint16(plain[at+2:]))) + int64(int32(binary.BigEndian.Uint32(plain[at+6:])))
		if base < 16 || base+12 >= int64(len(plain)) {
			continue
		}
		var slots []multiSlot
		firstBank := len(plain)
		seen := map[int]bool{}
		for code := 16; code+34 <= min(int(base), 2048); code += 2 {
			if !bytes.Equal(plain[code:code+2], []byte{0x20, 0xfc}) || !bytes.Equal(plain[code+6:code+8], []byte{0x20, 0xfc}) || !bytes.Equal(plain[code+12:code+14], []byte{0x20, 0xfc}) || !bytes.Equal(plain[code+18:code+20], []byte{0x41, 0xfa}) || !bytes.Equal(plain[code+22:code+24], []byte{0x43, 0xfa}) || !bytes.Equal(plain[code+26:code+28], []byte{0xd3, 0xfc}) || !bytes.Equal(plain[code+32:code+34], []byte{0x10, 0xd1}) {
				continue
			}
			bankAt := base + int64(binary.BigEndian.Uint32(plain[code+2:]))
			songAt := base + 4 + int64(binary.BigEndian.Uint32(plain[code+8:]))
			length := int64(binary.BigEndian.Uint32(plain[code+14:]))
			rateAt := int64(code+24) + int64(int16(binary.BigEndian.Uint16(plain[code+24:]))) + int64(int32(binary.BigEndian.Uint32(plain[code+28:])))
			if bankAt < base+12 || bankAt+bankHeader > songAt || songAt+length > int64(len(plain)) || length < songHeader || rateAt != songAt+22 || seen[int(bankAt)] {
				slots = nil
				break
			}
			bank, e := DecodeVoiceBank(plain[int(bankAt):int(songAt)])
			if e != nil {
				slots = nil
				break
			}
			if _, e := DecodeSong(plain[int(songAt):int(songAt+length)]); e != nil {
				slots = nil
				break
			}
			seen[int(bankAt)] = true
			firstBank = min(firstBank, int(bankAt))
			slots = append(slots, multiSlot{code, code + 28, bank.Version})
		}
		if len(slots) != count || firstBank != int(base)+12 {
			continue
		}
		for entry := 0; entry < 3; entry++ {
			code := entry * 4
			if binary.BigEndian.Uint16(plain[code:]) != 0x6000 {
				return template, fmt.Errorf("native: unsupported multi-song entry instruction")
			}
			target := code + 2 + int(int16(binary.BigEndian.Uint16(plain[code+2:])))
			if target < 16 || target >= firstBank {
				return template, fmt.Errorf("native: multi-song entry leaves its executable prefix")
			}
		}
		return MultiSNDHTemplate{Prefix: append([]byte(nil), plain[:firstBank]...), pointers: int(base), slots: slots}, nil
	}
	return template, fmt.Errorf("native: multi-song selector layout is not a verified relative-offset wrapper")
}

// EncodeMultiSNDH rebuilds all native banks and songs and the selector's voice,
// song, song-length and rate-address operands. Executable code and slot count
// remain those of the supplied runtime template.
func EncodeMultiSNDH(template MultiSNDHTemplate, projects []*model.Project, durations []time.Duration) ([]byte, error) {
	if len(projects) != len(template.slots) || len(projects) < 2 || len(durations) != len(projects) || template.pointers+12 != len(template.Prefix) {
		return nil, fmt.Errorf("native: multi-song export must replace every template slot")
	}
	out := append([]byte(nil), template.Prefix...)
	for slot, p := range projects {
		if p == nil || p.Bank.Version != template.slots[slot].version {
			return nil, fmt.Errorf("native: incompatible bank in multi-song slot %d", slot+1)
		}
		voice, err := EncodeVoiceBank(p.Bank)
		if err != nil {
			return nil, err
		}
		song, err := EncodeSong(p.Song)
		if err != nil {
			return nil, err
		}
		if len(out)%2 != 0 {
			out = append(out, 0)
		}
		bankAt := len(out)
		out = append(out, voice...)
		if len(out)%2 != 0 {
			out = append(out, 0)
		}
		songAt := len(out)
		out = append(out, song...)
		code := template.slots[slot].instruction
		binary.BigEndian.PutUint32(out[code+2:], uint32(bankAt-template.pointers))
		binary.BigEndian.PutUint32(out[code+8:], uint32(songAt-template.pointers-4))
		binary.BigEndian.PutUint32(out[code+14:], uint32(len(song)))
		pc := code + 24 + int(int16(binary.BigEndian.Uint16(out[code+24:])))
		binary.BigEndian.PutUint32(out[template.slots[slot].ratePointer:], uint32(songAt+22-pc))
	}
	if err := setSNDHText(out[:len(template.Prefix)], "TITL", projects[0].Title); err != nil {
		return nil, err
	}
	if err := setSNDHText(out[:len(template.Prefix)], "COMM", projects[0].Author); err != nil {
		return nil, err
	}
	if year := projects[0].Year; year != "" {
		if len(year) != 4 {
			return nil, fmt.Errorf("native: collection year must contain four decimal digits")
		}
		for _, digit := range year {
			if digit < '0' || digit > '9' {
				return nil, fmt.Errorf("native: invalid collection year")
			}
		}
		if err := setSNDHText(out[:len(template.Prefix)], "YEAR", year); err != nil {
			return nil, err
		}
	}
	end := bytes.Index(out[:len(template.Prefix)], []byte("HDNS"))
	at := sndhHeaderTag(out, "TIME", end)
	if end < 16 || end > 512 || at < 0 || at+4+len(projects)*2 > end {
		return nil, fmt.Errorf("native: multi-song template lacks its complete duration array")
	}
	for slot, duration := range durations {
		if duration < 0 || duration/time.Second > 65535 {
			return nil, fmt.Errorf("native: invalid multi-song duration")
		}
		binary.BigEndian.PutUint16(out[at+4+slot*2:], uint16(duration/time.Second))
	}
	if _, err := ParseMultiSNDHTemplate(out); err != nil {
		return nil, fmt.Errorf("native: generated multi-song selector: %w", err)
	}
	return out, nil
}
