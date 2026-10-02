package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// SNDHTemplate retains an existing native replay prefix independently of its
// song. Templates are loaded at runtime; original executable data is not bundled
// with this Go application. Data pointers must use the original relative layout.
type SNDHTemplate struct {
	Prefix   []byte
	Version  byte
	bankAt   int
	pointers int
}

func ParseSNDHTemplate(data []byte) (SNDHTemplate, error) {
	var template SNDHTemplate
	plain, err := UnpackICE(data)
	if err != nil {
		return template, err
	}
	projects, err := DecodeContainers(plain)
	if err != nil {
		return template, err
	}
	if len(projects) != 1 || DeclaredSubtunes(plain) > 1 {
		return template, fmt.Errorf("native: multi-tune replay wrapper cannot export a single song; select a single-song replay template")
	}
	inst := bytes.Index(plain, []byte("MYM1INST"))
	if inst < 40 {
		inst = bytes.Index(plain, []byte("MYM0INST"))
	}
	if inst < 40 {
		return template, fmt.Errorf("native: SNDH template has no supported replay prefix")
	}
	pointers, bankAt := inst-40, inst-32
	if binary.BigEndian.Uint32(plain[pointers:]) != 8 {
		return template, fmt.Errorf("native: SNDH template does not use relative replay data pointers")
	}
	songPointer := pointers + 4
	songAt := songPointer + int(binary.BigEndian.Uint32(plain[songPointer:]))
	if songAt < bankAt || songAt+8 > len(plain) || !bytes.Equal(plain[songAt:songAt+8], []byte("MYM0TRAK")) {
		return template, fmt.Errorf("native: SNDH template has an invalid song pointer")
	}
	for entry := 0; entry < 3; entry++ {
		at := entry * 4
		if binary.BigEndian.Uint16(plain[at:]) != 0x6000 {
			return template, fmt.Errorf("native: SNDH template has unsupported entry instructions")
		}
		target := at + 2 + int(int16(binary.BigEndian.Uint16(plain[at+2:])))
		if target < 16 || target >= pointers {
			return template, fmt.Errorf("native: SNDH entry points outside its replay prefix")
		}
	}
	template = SNDHTemplate{Prefix: append([]byte(nil), plain[:bankAt]...), Version: plain[inst+3] - '0', bankAt: bankAt, pointers: pointers}
	return template, nil
}

// EncodeSNDH replaces editable payloads and metadata while preserving the
// template's executable prefix and its original entry-point addresses.
func EncodeSNDH(template SNDHTemplate, project *model.Project, duration time.Duration) ([]byte, error) {
	if template.bankAt != len(template.Prefix) || template.pointers != template.bankAt-8 || template.Version > 1 || len(template.Prefix) < 16 {
		return nil, fmt.Errorf("native: invalid SNDH replay template")
	}
	if duration < 0 || duration/time.Second > 65535 {
		return nil, fmt.Errorf("native: SNDH duration must be 0–65535 seconds")
	}
	bank := project.Bank
	if bank.Version != template.Version {
		return nil, fmt.Errorf("native: bank version %d does not match replay template version %d", bank.Version, template.Version)
	}
	voice, err := EncodeVoiceBank(bank)
	if err != nil {
		return nil, err
	}
	song, err := EncodeSong(project.Song)
	if err != nil {
		return nil, err
	}
	// First sample pointer lands eight bytes after the sample tag. The song
	// is inserted before this tag, as in the native editor's own SNDH export.
	digi := int(binary.BigEndian.Uint32(voice[:4])) - 8
	if digi < bankHeader || digi+8 > len(voice) {
		return nil, fmt.Errorf("native: invalid SNDH voice payload")
	}
	for i := 0; i < 8; i++ {
		at := i * 4
		offset := binary.BigEndian.Uint32(voice[at:])
		binary.BigEndian.PutUint32(voice[at:], offset+uint32(len(song)))
	}
	out := append([]byte(nil), template.Prefix...)
	if err := setSNDHText(out, "TITL", project.Title); err != nil {
		return nil, err
	}
	if err := setSNDHText(out, "COMM", project.Author); err != nil {
		return nil, err
	}
	if err := setSNDHRate(out, project.Song.TickRate()); err != nil {
		return nil, err
	}
	headerEnd := bytes.Index(out, []byte("HDNS"))
	if headerEnd < 16 || headerEnd > 512 {
		return nil, fmt.Errorf("native: SNDH template lacks a bounded header")
	}
	if at := sndhHeaderTag(out, "TIME", headerEnd); at >= 0 && at+6 <= headerEnd {
		binary.BigEndian.PutUint16(out[at+4:], uint16(duration/time.Second))
	} else if duration != 0 {
		return nil, fmt.Errorf("native: SNDH template has no TIME field")
	}
	songAt := template.bankAt + digi
	binary.BigEndian.PutUint32(out[template.pointers+4:], uint32(songAt-(template.pointers+4)))
	out = append(out, voice[:digi]...)
	out = append(out, song...)
	out = append(out, voice[digi:]...)
	if _, err := DecodeContainer(out); err != nil {
		return nil, fmt.Errorf("native: generated SNDH payload: %w", err)
	}
	return out, nil
}

func sndhHeaderTag(data []byte, tag string, end int) int {
	for at := 16; at+len(tag) <= end; at++ {
		if bytes.Equal(data[at:at+len(tag)], []byte(tag)) && (at == 16 || data[at-1] == 0) {
			return at
		}
	}
	return -1
}

func setSNDHText(prefix []byte, tag, value string) error {
	end := bytes.Index(prefix, []byte("HDNS"))
	if end < 16 || end > 512 {
		return fmt.Errorf("native: unsupported SNDH metadata header")
	}
	at := sndhHeaderTag(prefix, tag, end)
	if at < 0 {
		return fmt.Errorf("native: SNDH template lacks %s metadata", tag)
	}
	start := at + 4
	terminator := bytes.IndexByte(prefix[start:end], 0)
	if terminator < 0 {
		return fmt.Errorf("native: unterminated SNDH metadata")
	}
	finish := start + terminator + 1
	for finish < end && prefix[finish] == 0 {
		finish++
	}
	capacity := finish - start
	if len(value) >= capacity || bytes.IndexByte([]byte(value), 0) >= 0 {
		return fmt.Errorf("native: %s metadata is limited to %d bytes by this replay template", tag, capacity-1)
	}
	clear(prefix[start:finish])
	copy(prefix[start:], value)
	return nil
}

func setSNDHRate(prefix []byte, rate int) error {
	end := bytes.Index(prefix, []byte("HDNS"))
	if end < 16 || end > 512 {
		return fmt.Errorf("native: unsupported SNDH metadata header")
	}
	at := sndhHeaderTag(prefix, "TC", end)
	if at < 0 {
		return fmt.Errorf("native: SNDH template lacks timer C rate metadata")
	}
	finish := at + 2
	for finish < end && (prefix[finish] >= '0' && prefix[finish] <= '9' || prefix[finish] == 0) {
		finish++
	}
	value := strconv.Itoa(rate)
	if len(value)+1 > finish-(at+2) {
		return fmt.Errorf("native: timer C rate does not fit SNDH metadata")
	}
	clear(prefix[at+2 : finish])
	copy(prefix[at+2:], value)
	return nil
}
