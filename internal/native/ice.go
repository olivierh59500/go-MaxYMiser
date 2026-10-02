package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const maxICEOutput = 16 << 20

type iceDecoder struct {
	packed []byte
	at     int
	bits   byte
	output []byte
	write  int
}

func (d *iceDecoder) previous() (byte, error) {
	if d.at <= 12 {
		return 0, fmt.Errorf("native: truncated ICE bit/literal stream")
	}
	d.at--
	return d.packed[d.at], nil
}

// The low sentinel bit refills the backwards stream; its carry becomes the
// inserted sentinel in the next byte. This is the native ADD/ADDX convention.
func (d *iceDecoder) bit() (int, error) {
	carry := int(d.bits >> 7)
	d.bits <<= 1
	if d.bits == 0 {
		value, err := d.previous()
		if err != nil {
			return 0, err
		}
		d.bits = value<<1 | byte(carry)
		carry = int(value >> 7)
	}
	return carry, nil
}

func (d *iceDecoder) readBits(count int) (int, error) {
	value := 0
	for i := 0; i < count; i++ {
		bit, err := d.bit()
		if err != nil {
			return 0, err
		}
		value = value<<1 | bit
	}
	return value, nil
}

// UnpackICE handles the original backwards Pack-Ice stream and optional ST
// bitplane transformation. Ordinary native files are returned unchanged.
func UnpackICE(data []byte) ([]byte, error) {
	if len(data) < 4 || !bytes.EqualFold(data[:4], []byte("ICE!")) {
		return data, nil
	}
	if len(data) < 13 {
		return nil, fmt.Errorf("native: incomplete ICE header")
	}
	packed := int(binary.BigEndian.Uint32(data[4:8]))
	size := int(binary.BigEndian.Uint32(data[8:12]))
	if packed < 13 || packed > len(data) || size <= 0 || size > maxICEOutput {
		return nil, fmt.Errorf("native: invalid ICE dimensions")
	}
	d := iceDecoder{packed: data[:packed], at: packed - 1, bits: data[packed-1], output: make([]byte, size), write: size}
	for d.write > 0 {
		literal, err := d.bit()
		if err != nil {
			return nil, err
		}
		if literal != 0 {
			length := 1
			flag, err := d.bit()
			if err != nil {
				return nil, err
			}
			if flag != 0 {
				widths := []int{2, 2, 3, 8, 15}
				maxima := []int{3, 3, 7, 255, 32767}
				bases := []int{1, 4, 7, 14, 269}
				for i, width := range widths {
					value, err := d.readBits(width)
					if err != nil {
						return nil, err
					}
					if value != maxima[i] || i == len(widths)-1 {
						length = value + bases[i] + 1
						break
					}
				}
			}
			if length > d.write {
				return nil, fmt.Errorf("native: ICE literal run exceeds output")
			}
			for i := 0; i < length; i++ {
				value, err := d.previous()
				if err != nil {
					return nil, err
				}
				d.write--
				d.output[d.write] = value
			}
		}
		if d.write == 0 {
			break
		}
		category := 3
		for category >= 0 {
			bit, err := d.bit()
			if err != nil {
				return nil, err
			}
			if bit == 0 {
				break
			}
			category--
		}
		lengthBits := []int{10, 2, 1, 0, 0}
		lengthBase := []int{8, 4, 2, 1, 0}
		length, err := d.readBits(lengthBits[category+1])
		if err != nil {
			return nil, err
		}
		length += lengthBase[category+1]
		offset := 0
		if length == 0 {
			flag, err := d.bit()
			if err != nil {
				return nil, err
			}
			width, base := 6, -1
			if flag != 0 {
				width, base = 9, 63
			}
			offset, err = d.readBits(width)
			if err != nil {
				return nil, err
			}
			offset += base
		} else {
			category := 1
			for category >= 0 {
				bit, err := d.bit()
				if err != nil {
					return nil, err
				}
				if bit == 0 {
					break
				}
				category--
			}
			widths, bases := []int{12, 5, 8}, []int{287, -1, 31}
			offset, err = d.readBits(widths[category+1])
			if err != nil {
				return nil, err
			}
			offset += bases[category+1]
			if offset < 0 {
				offset -= length
			}
		}
		count := length + 2
		if count > d.write {
			return nil, fmt.Errorf("native: ICE match exceeds output")
		}
		source := d.write + length + offset + 2
		for i := 0; i < count; i++ {
			source--
			d.write--
			if source <= d.write || source >= len(d.output) {
				return nil, fmt.Errorf("native: invalid ICE match offset")
			}
			d.output[d.write] = d.output[source]
		}
	}
	transform, err := d.bit()
	if err != nil {
		return nil, err
	}
	if transform != 0 {
		blocks := 4000
		custom, err := d.bit()
		if err != nil {
			return nil, err
		}
		if custom != 0 {
			value, err := d.readBits(16)
			if err != nil {
				return nil, err
			}
			blocks = value + 1
		}
		if blocks > len(d.output)/8 {
			return nil, fmt.Errorf("native: ICE bitplane transform exceeds output")
		}
		at := len(d.output)
		for block := 0; block < blocks; block++ {
			var planes [4]uint16
			for word := 0; word < 4; word++ {
				at -= 2
				packed := binary.BigEndian.Uint16(d.output[at:])
				for nibble := 0; nibble < 4; nibble++ {
					for plane := 0; plane < 4; plane++ {
						planes[plane] = planes[plane]<<1 | (packed >> 15)
						packed <<= 1
					}
				}
			}
			for plane, word := range planes {
				binary.BigEndian.PutUint16(d.output[at+plane*2:], word)
			}
		}
	}
	return d.output, nil
}
