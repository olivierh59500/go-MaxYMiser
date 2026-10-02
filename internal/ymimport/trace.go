// Package ymimport inspects register recordings and proposes editable arrangements.
package ymimport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/olivierh59500/ym-player/pkg/lzh"
)

type Trace struct {
	Name, Author string
	Rate         int
	Clock        uint32
	Frames       [][14]byte
	Effects      bool
}

func Decode(data []byte) (Trace, error) {
	var trace Trace
	if lzh.IsLZHCompressed(data) {
		var err error
		data, err = lzh.Decompress(data)
		if err != nil {
			return trace, err
		}
	}
	if len(data) < 4 {
		return trace, fmt.Errorf("ymimport: incomplete header")
	}
	signature := string(data[:4])
	start, count, registers := 4, 0, 14
	interleaved := true
	trace.Rate, trace.Clock = 50, 2000000
	switch signature {
	case "YM2!", "YM3!":
		count = (len(data) - 4) / 14
		trace.Effects = signature == "YM2!"
	case "YM3b":
		if len(data) < 8 {
			return trace, fmt.Errorf("ymimport: incomplete YM3b")
		}
		count = (len(data) - 8) / 14
	case "YM5!", "YM6!":
		if len(data) < 34 || !bytes.Equal(data[4:12], []byte("LeOnArD!")) {
			return trace, fmt.Errorf("ymimport: incomplete YM5/YM6 header")
		}
		count = int(binary.BigEndian.Uint32(data[12:]))
		attributes := binary.BigEndian.Uint32(data[16:])
		drums := int(binary.BigEndian.Uint16(data[20:]))
		trace.Clock = binary.BigEndian.Uint32(data[22:])
		trace.Rate = int(binary.BigEndian.Uint16(data[26:]))
		start = 34 + int(binary.BigEndian.Uint16(data[32:]))
		registers = 16
		interleaved = attributes&1 != 0
		trace.Effects = drums > 0
		for i := 0; i < drums; i++ {
			if start+4 > len(data) {
				return trace, fmt.Errorf("ymimport: truncated drum header")
			}
			length := int(binary.BigEndian.Uint32(data[start:]))
			start += 4
			if length > len(data)-start {
				return trace, fmt.Errorf("ymimport: truncated drum")
			}
			start += length
		}
		names := make([]string, 3)
		for i := range names {
			if start >= len(data) {
				return trace, fmt.Errorf("ymimport: truncated metadata")
			}
			end := bytes.IndexByte(data[start:], 0)
			if end < 0 {
				return trace, fmt.Errorf("ymimport: unterminated metadata")
			}
			names[i] = string(data[start : start+end])
			start += end + 1
		}
		trace.Name, trace.Author = names[0], names[1]
	default:
		return trace, fmt.Errorf("ymimport: register reconstruction requires YM2, YM3, YM3b, YM5 or YM6")
	}
	if count <= 0 || count > 1000000 || trace.Rate < 1 || trace.Rate > 2000 || trace.Clock == 0 || count > (len(data)-start)/registers {
		return trace, fmt.Errorf("ymimport: invalid register stream dimensions")
	}
	trace.Frames = make([][14]byte, count)
	shape := byte(0)
	for frame := 0; frame < count; frame++ {
		for reg := 0; reg < 14; reg++ {
			at := start + frame*registers + reg
			if interleaved {
				at = start + reg*count + frame
			}
			trace.Frames[frame][reg] = data[at]
		}
		if trace.Frames[frame][13] == 255 {
			trace.Frames[frame][13] = shape
		} else {
			shape = trace.Frames[frame][13] & 15
		}
		if registers == 16 {
			for _, reg := range []int{14, 15} {
				at := start + frame*registers + reg
				if interleaved {
					at = start + reg*count + frame
				}
				trace.Effects = trace.Effects || data[at] != 0
			}
		}
	}
	return trace, nil
}
