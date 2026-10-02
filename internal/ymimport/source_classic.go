package ymimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const madMaxClassic = "mad-max-classic-1988-v1"

// This normalized digest identifies the complete program carried by the
// verified classic files. Song/pattern/instrument tables are outside this region. Only in-range
// absolute program pointers are rebased before hashing, as in the native stub.
const madMaxClassicDigest = "49c7b72e1eade49ef6deff55d3cf73d4bbf9076a0c688a3103508d24732050f8"

func classicProgramDigest(data []byte, code, end int) (string, error) {
	if code < 0 || end < code || end > len(data) {
		return "", fmt.Errorf("source: classic program range is outside the SNDH")
	}
	normalized := append([]byte(nil), data[code:end]...)
	for at := 0; at+4 <= len(normalized); at += 2 {
		value := binary.BigEndian.Uint32(normalized[at:])
		if at != 0xba2 && value >= 0x11db4 && value < 0x13300 {
			binary.BigEndian.PutUint32(normalized[at:], value-0x11db4)
			at += 2
		}
	}
	hash := sha256.Sum256(normalized)
	return hex.EncodeToString(hash[:]), nil
}

func detectMadMaxClassic(data []byte) (sourceDataLayout, bool, error) {
	var layout sourceDataLayout
	parser := bytes.Index(data, []byte{0x53, 0x28, 0, 0x1b, 0x66, 0, 1, 0x6e, 0x11, 0x7c, 0, 0, 0, 0x2e, 0x10, 0xbc})
	if parser < 0 || !bytes.Contains(data[:min(len(data), 128)], []byte("SNDH")) {
		return layout, false, nil
	}
	code := parser - 0xcf6
	root := code + 0x154c
	digest, err := classicProgramDigest(data, code, root)
	if err != nil || digest != madMaxClassicDigest || code > 0x11db4 {
		return layout, false, nil
	}
	// The initializer's paired PC-relative LEAs must address these exact
	// program and music regions. A matching code fragment alone is insufficient.
	initializer := false
	for at := 0; at+8 <= code; at += 2 {
		if !bytes.Equal(data[at:at+2], []byte{0x4b, 0xfa}) || !bytes.Equal(data[at+4:at+6], []byte{0x49, 0xfa}) {
			continue
		}
		musicTarget := at + 2 + int(int16(binary.BigEndian.Uint16(data[at+2:])))
		codeTarget := at + 6 + int(int16(binary.BigEndian.Uint16(data[at+6:])))
		if musicTarget == root && codeTarget == code {
			initializer = true
			break
		}
	}
	if !initializer || !bytes.Contains(data[:code], []byte{0x22, 0x3c, 0, 1, 0x1d, 0xb4, 0x24, 0x3c, 0, 1, 0x33, 0}) {
		return layout, true, fmt.Errorf("source: incompatible classic program relocation wrapper")
	}
	if !bytes.Contains(data[:code], []byte{0x22, 0x3c, 0, 6, 0, 0, 0x24, 0x3c, 0, 6, 0x20, 0}) {
		return layout, true, fmt.Errorf("source: incompatible classic music relocation wrapper")
	}
	if !bytes.Contains(data[:code], []byte("TC50\x00")) {
		return layout, true, fmt.Errorf("source: classic wrapper replay rate is not verified")
	}
	layout = sourceDataLayout{
		player:      madMaxClassic,
		music:       sourceReader{data: data, origin: uint32(0x60000 - root)},
		program:     sourceReader{data: data, origin: uint32(0x11db4 - code)},
		instruments: root + 16, patterns: root + 20, songs: root + 24,
		speed: root + 0x3f0, arpeggios: code + 0x146,
		noise: [3]int{code + 0xdfc, code + 0xe1a, code + 0xe38},
	}
	return layout, true, nil
}

// Classic 8C switches the mixer without an operand; 90 is a timed wait.
// Its jump table stops at 90, unlike the later Last Ninja command set.
func sourcePlayerOperandLength(player string, op byte) (int, bool) {
	if player == madMaxClassic {
		if op == 0x8c {
			return 0, true
		}
		if op > 0x90 && op < 0xb8 {
			return 0, false
		}
	}
	return sourceOperandLength(op)
}
