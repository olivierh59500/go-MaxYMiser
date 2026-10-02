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
	if len(voice) < bankHeader || !bytes.Equal(voice[32:35], []byte("MYM")) || !bytes.Equal(voice[36:40], []byte("INST")) || len(music) < songHeader+18 {
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
	if (len(voice)-bankHeader)%stride != 0 || len(voice) <= bankHeader || (len(voice)-bankHeader)/stride > model.MaxSequences {
		return bank, model.Song{}, bankError
	}
	for slot := 0; slot < model.MaxSamples; slot++ {
		if binary.BigEndian.Uint16(voice[2088+slot*4:]) != 0 {
			return bank, model.Song{}, bankError
		}
	}
	// The native song-size operand includes the 8-byte tag, eight empty-slot
	// guards and a 2-byte suffix. Preserve the original version and suffix.
	tail := music[len(music)-18:]
	if !bytes.Equal(tail[:3], []byte("MYM")) || tail[3] != '0' && tail[3] != '1' || !bytes.Equal(tail[4:8], []byte("DIGI")) || !bytes.Equal(tail[8:16], make([]byte, 8)) {
		return bank, model.Song{}, bankError
	}
	song, err := DecodeSong(music[:len(music)-18])
	if err != nil {
		return bank, song, err
	}
	normalized := append([]byte(nil), voice...)
	normalized = append(normalized, tail...)
	for slot := 0; slot < model.MaxSamples; slot++ {
		binary.BigEndian.PutUint32(normalized[slot*4:], uint32(len(voice)+8+slot-slot*4))
	}
	bank, err = DecodeVoiceBank(normalized)
	return bank, song, err
}
