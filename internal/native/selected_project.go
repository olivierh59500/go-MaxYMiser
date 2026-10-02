package native

import (
	"bytes"
	"encoding/binary"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// decodeSelectedProject uses exact spans supplied by a verified native selector.
// Some optimized empty banks retain sample pointers into the song, while their
// actual DIGI tag and guards follow the song's complete pattern data. Only that
// bounded layout is recovered; nonempty missing samples remain an error.
func decodeSelectedProject(voice, music []byte) (model.VoiceBank, model.Song, error) {
	bank, bankError := DecodeVoiceBank(voice)
	if bankError == nil {
		song, err := DecodeSong(music)
		return bank, song, err
	}
	if len(voice) < bankHeader || !bytes.Equal(voice[32:35], []byte("MYM")) || !bytes.Equal(voice[36:40], []byte("INST")) || len(music) < songHeader+16 {
		return bank, model.Song{}, bankError
	}
	stride := 128
	switch voice[35] {
	case '0':
		stride = 64
	case '1':
	default:
		return bank, model.Song{}, bankError
	}
	padding := (len(voice) - bankHeader) % stride
	if padding != 0 && padding != 2 || len(voice)-padding <= bankHeader || (len(voice)-padding-bankHeader)/stride > model.MaxSequences {
		return bank, model.Song{}, bankError
	}
	for slot := 0; slot < model.MaxSamples; slot++ {
		if binary.BigEndian.Uint16(voice[2088+slot*4:]) != 0 {
			return bank, model.Song{}, bankError
		}
	}
	// Traverse complete native pattern records. A matching tag inside a row
	// cannot delimit the music. Its remaining bytes belong to the exact span
	// copied by the selector, including any opaque optimizer suffix.
	tagAt := selectedSampleTag(music)
	if tagAt < 0 {
		return bank, model.Song{}, bankError
	}
	tail := music[tagAt:]
	song, err := DecodeSong(music[:tagAt])
	if err != nil {
		return bank, song, err
	}
	normalized := append([]byte(nil), voice[:len(voice)-padding]...)
	base := len(normalized)
	normalized = append(normalized, tail...)
	for slot := 0; slot < model.MaxSamples; slot++ {
		binary.BigEndian.PutUint32(normalized[slot*4:], uint32(base+8+slot-slot*4))
	}
	bank, err = DecodeVoiceBank(normalized)
	return bank, song, err
}

func selectedSampleTag(music []byte) int {
	row, patterns := 0, 0
	for at := songHeader; at+8 <= len(music); at += 8 {
		if row == 0 && at+16 <= len(music) && bytes.Equal(music[at:at+3], []byte("MYM")) && (music[at+3] == '0' || music[at+3] == '1') && bytes.Equal(music[at+4:at+8], []byte("DIGI")) && bytes.Equal(music[at+8:at+16], make([]byte, 8)) {
			return at
		}
		if row == 0 && patterns >= model.MaxPatterns {
			return -1
		}
		skip := int(music[at+7])
		if row+skip >= model.Rows {
			return -1
		}
		row += skip + 1
		if row == model.Rows {
			row = 0
			patterns++
		}
	}
	return -1
}
