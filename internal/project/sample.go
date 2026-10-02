package project

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// SaveSample writes the signed PCM payload without a native bank trailer.
// Repeated saves replace a complete previous sample after staging and syncing.
func SaveSample(sample model.Sample, path string) error {
	if len(sample.PCM) > 32768 {
		return fmt.Errorf("project: raw sample exceeds 32 KiB")
	}
	return atomicWrite(path, sample.PCM)
}

// DecodeSample accepts signed headerless PCM or ordinary PCM WAV files.
func DecodeSample(data []byte) ([]byte, error) {
	if len(data) < 12 || !bytes.Equal(data[:4], []byte("RIFF")) {
		return append([]byte(nil), data...), nil
	}
	if !bytes.Equal(data[8:12], []byte("WAVE")) {
		return nil, fmt.Errorf("project: unsupported RIFF sample")
	}
	channels, bits, format := 0, 0, 0
	var pcm []byte
	for at := 12; at+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[at+4:]))
		if size < 0 || size > len(data)-at-8 {
			return nil, fmt.Errorf("project: truncated WAV chunk")
		}
		chunk := data[at+8 : at+8+size]
		switch string(data[at : at+4]) {
		case "fmt ":
			if len(chunk) < 16 {
				return nil, fmt.Errorf("project: invalid WAV format")
			}
			format = int(binary.LittleEndian.Uint16(chunk))
			channels = int(binary.LittleEndian.Uint16(chunk[2:]))
			bits = int(binary.LittleEndian.Uint16(chunk[14:]))
		case "data":
			pcm = chunk
		}
		at += 8 + size + (size & 1)
	}
	if format != 1 || channels < 1 || channels > 2 || (bits != 8 && bits != 16) {
		return nil, fmt.Errorf("project: sample WAV must be PCM mono/stereo, 8 or 16 bits")
	}
	stride := channels * bits / 8
	out := make([]byte, len(pcm)/stride)
	for frame := range out {
		sum := 0
		for channel := 0; channel < channels; channel++ {
			at := frame*stride + channel*bits/8
			if bits == 8 {
				sum += int(pcm[at]) - 128
			} else {
				sum += int(int16(binary.LittleEndian.Uint16(pcm[at:]))) >> 8
			}
		}
		out[frame] = byte(int8(sum / channels))
	}
	return out, nil
}
