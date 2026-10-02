package native

import (
	"encoding/binary"
	"fmt"
)

type iceOperation struct {
	value byte
	raw   bool
}

type iceWriter struct {
	operations []iceOperation
	bits       []byte
}

func (w *iceWriter) bit(value int) {
	bit := byte(value & 1)
	w.operations = append(w.operations, iceOperation{value: bit})
	w.bits = append(w.bits, bit)
}

func (w *iceWriter) value(value, width int) {
	for shift := width - 1; shift >= 0; shift-- {
		w.bit(value >> shift)
	}
}

func (w *iceWriter) literals(data []byte) error {
	length := len(data)
	if length == 0 {
		w.bit(0)
		return nil
	}
	if length > 33037 {
		return fmt.Errorf("native: ICE literal group exceeds the format limit")
	}
	w.bit(1)
	if length == 1 {
		w.bit(0)
	} else {
		w.bit(1)
		widths, maxima, bases := []int{2, 2, 3, 8, 15}, []int{3, 3, 7, 255, 32767}, []int{2, 5, 8, 15, 270}
		for i, width := range widths {
			value := length - bases[i]
			if value < maxima[i] || i == len(widths)-1 {
				w.value(value, width)
				break
			}
			w.value(maxima[i], width)
		}
	}
	for i := len(data) - 1; i >= 0; i-- {
		w.operations = append(w.operations, iceOperation{value: data[i], raw: true})
	}
	return nil
}

func (w *iceWriter) match(length, distance int) {
	encoded := length - 2
	switch {
	case encoded == 0:
		w.bit(0)
	case encoded == 1:
		w.value(2, 2)
	case encoded <= 3:
		w.value(6, 3)
		w.value(encoded-2, 1)
	case encoded <= 7:
		w.value(14, 4)
		w.value(encoded-4, 2)
	default:
		w.value(15, 4)
		w.value(encoded-8, 10)
	}
	offset := distance - encoded - 2
	if encoded == 0 {
		if offset <= 62 {
			w.bit(0)
			w.value(offset+1, 6)
		} else {
			w.bit(1)
			w.value(offset-63, 9)
		}
		return
	}
	if distance == 1 {
		w.value(2, 2)
		w.value(0, 5)
		return
	}
	switch {
	case offset <= 30:
		w.value(2, 2)
		w.value(offset+1, 5)
	case offset <= 286:
		w.bit(0)
		w.value(offset-31, 8)
	default:
		w.value(3, 2)
		w.value(offset-287, 12)
	}
}

// finish materializes bit-byte fetches and literals in the exact order in
// which the native backwards decoder consumes them. The first byte carries a
// partial group and sentinel; subsequent groups contain eight data bits.
func (w *iceWriter) finish(size int) []byte {
	initial := len(w.bits) % 8
	first := byte(1 << (7 - initial))
	for i := 0; i < initial; i++ {
		first |= w.bits[i] << (7 - i)
	}
	consumed := []byte{first}
	bitIndex := 0
	for _, op := range w.operations {
		if op.raw {
			consumed = append(consumed, op.value)
			continue
		}
		if bitIndex >= initial && (bitIndex-initial)%8 == 0 {
			value := byte(0)
			for i := 0; i < 8; i++ {
				value |= w.bits[bitIndex+i] << (7 - i)
			}
			consumed = append(consumed, value)
		}
		bitIndex++
	}
	out := make([]byte, 12+len(consumed))
	copy(out, "ICE!")
	binary.BigEndian.PutUint32(out[4:], uint32(len(out)))
	binary.BigEndian.PutUint32(out[8:], uint32(size))
	for i, value := range consumed {
		out[len(out)-1-i] = value
	}
	return out
}

// PackICE compresses a native payload using bounded LZ matches. It emits the
// standard backwards stream without the optional graphics bitplane transform.
// Match candidates are bounded so large banks cannot trigger quadratic scans.
func PackICE(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > maxICEOutput {
		return nil, fmt.Errorf("native: ICE input must be 1–%d bytes", maxICEOutput)
	}
	writer := iceWriter{}
	type candidates struct {
		positions [64]int
		count     int
	}
	index := map[uint16]*candidates{}
	add := func(end int) {
		if end < 2 {
			return
		}
		key := uint16(data[end-1])<<8 | uint16(data[end-2])
		values := index[key]
		if values == nil {
			values = &candidates{}
			index[key] = values
		}
		values.positions[values.count%64] = end
		values.count++
	}
	find := func(end int) (int, int) {
		if end < 2 {
			return 0, 0
		}
		key := uint16(data[end-1])<<8 | uint16(data[end-2])
		values := index[key]
		if values == nil {
			return 0, 0
		}
		best, distance := 0, 0
		for candidate := 0; candidate < min(values.count, 64); candidate++ {
			source := values.positions[(values.count-1-candidate)%64]
			delta := source - end
			if delta < 1 || delta > 5414 {
				continue
			}
			length := 0
			for length < min(end, 1033) && data[end-1-length] == data[source-1-length] {
				length++
			}
			if length < 2 {
				continue
			}
			// All positive offsets need distance >= match length. Distance
			// one has a dedicated repeated-byte encoding for longer runs.
			if delta != 1 {
				length = min(length, delta)
			}
			if length == 2 && delta > 576 {
				continue
			}
			if delta > length+4382 {
				continue
			}
			if length > best {
				best, distance = length, delta
			}
		}
		return best, distance
	}
	end, literalEnd := len(data), len(data)
	for end > 0 {
		length, distance := find(end)
		if length >= 3 || length == 2 && (distance <= 64 || literalEnd-end > 32000) {
			if err := writer.literals(data[end:literalEnd]); err != nil {
				return nil, err
			}
			writer.match(length, distance)
			for next := end; next > end-length; next-- {
				add(next)
			}
			end -= length
			literalEnd = end
		} else {
			add(end)
			end--
			if literalEnd-end >= 33037 {
				return nil, fmt.Errorf("native: ICE incompressible literal group exceeds format limit")
			}
		}
	}
	if err := writer.literals(data[:literalEnd]); err != nil {
		return nil, err
	}
	writer.bit(0)
	return writer.finish(len(data)), nil
}
