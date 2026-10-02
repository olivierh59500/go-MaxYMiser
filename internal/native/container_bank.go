package native

import (
	"encoding/binary"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// Optimized SNDH wrappers can remove trailing empty sample guards without
// rewriting their pointers. Only an entirely empty sample bank can normalize
// those out-of-file offsets: no nonempty waveform is recovered or fabricated.
func decodeContainerBank(data []byte) (model.VoiceBank, error) {
	bank, err := DecodeVoiceBank(data)
	if err == nil || len(data) < bankHeader+8 {
		return bank, err
	}
	for i := 0; i < 8; i++ {
		if binary.BigEndian.Uint16(data[2088+i*4:]) != 0 {
			return bank, err
		}
	}
	first := int(binary.BigEndian.Uint32(data[:4]))
	if first < bankHeader+8 || first > len(data) {
		return bank, err
	}
	normalized := append([]byte(nil), data...)
	changed := false
	for i := 1; i < 8; i++ {
		at := i * 4
		offset := uint64(at) + uint64(binary.BigEndian.Uint32(data[at:]))
		if offset > uint64(len(data)) {
			binary.BigEndian.PutUint32(normalized[at:], uint32(len(data)-at))
			changed = true
		}
	}
	if !changed {
		return bank, err
	}
	return DecodeVoiceBank(normalized)
}
