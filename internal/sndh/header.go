// Package sndh reads executable Atari ST music containers independently of the
// original player's tracker or instrument format.
package sndh

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

const (
	maxFileSize   = 16 << 20
	maxHeaderSize = 64 << 10
)

// Metadata describes the outer executable container. Frames count calls to the
// play entry, rather than the player's internal tracker steps. A zero duration
// is indefinite; a nil Frames slice means no duration was supplied.
type Metadata struct {
	Title, Author, Year, Ripper, Converter string
	Rate, Subtunes, DefaultSubtune         int
	Frames                                 []uint32
	SubtuneNames                           []string
	Flags                                  []string
	HeaderEnd                              int // Exclusive, immediately after HDNS.
}

// File owns its unpacked executable data and metadata. Initialization may modify
// this program's entry instructions, so their initial targets are not validated
// against the file boundaries here.
type File struct {
	Data     []byte
	Metadata Metadata
}

// Parse accepts plain and Pack-Ice containers, with bounded metadata tables.
// The original input can be discarded or changed after Parse returns.
func Parse(raw []byte) (*File, error) {
	if len(raw) > maxFileSize {
		return nil, fmt.Errorf("sndh: file exceeds %d bytes", maxFileSize)
	}
	owned := append([]byte(nil), raw...)
	data, err := native.UnpackICE(owned)
	if err != nil {
		return nil, fmt.Errorf("sndh: %w", err)
	}
	if len(data) < 20 || len(data) > maxFileSize || !bytes.Equal(data[12:16], []byte("SNDH")) {
		return nil, fmt.Errorf("sndh: invalid executable container")
	}
	// Restart when ## follows an array. This also keeps text containing tag
	// names opaque and permits the count and duration tags in either order.
	songs := 1
	seen := map[int]bool{}
	for !seen[songs] {
		seen[songs] = true
		metadata, declared, err := parseMetadata(data, songs)
		if err != nil {
			return nil, err
		}
		if declared != songs {
			songs = declared
			continue
		}
		return &File{Data: data, Metadata: metadata}, nil
	}
	return nil, fmt.Errorf("sndh: conflicting subtune counts")
}

func parseMetadata(data []byte, songs int) (Metadata, int, error) {
	m := Metadata{Rate: 50, Subtunes: songs, DefaultSubtune: 1}
	limit := min(len(data), maxHeaderSize)
	var seconds []uint16
	var frames []uint32
	countSeen := false
	for at := 16; at+4 <= limit; {
		if data[at] == 0 {
			at++
			continue
		}
		tag := string(data[at : at+4])
		switch tag {
		case "HDNS":
			m.HeaderEnd = at + 4
			if m.DefaultSubtune < 1 || m.DefaultSubtune > songs {
				m.DefaultSubtune = 1
			}
			if frames != nil {
				m.Frames = frames
			} else if seconds != nil {
				m.Frames = make([]uint32, songs)
				for i, value := range seconds {
					calls := uint64(value) * uint64(m.Rate)
					if calls > math.MaxUint32 {
						return m, songs, fmt.Errorf("sndh: TIME duration exceeds 32-bit frame count")
					}
					m.Frames[i] = uint32(calls)
				}
			}
			return m, songs, nil
		case "TITL", "COMM", "YEAR", "RIPP", "CONV":
			value, next, err := headerString(data, at+4, limit)
			if err != nil {
				return m, songs, fmt.Errorf("sndh: %s: %w", tag, err)
			}
			switch tag {
			case "TITL":
				m.Title = value
			case "COMM":
				m.Author = value
			case "YEAR":
				m.Year = value
			case "RIPP":
				m.Ripper = value
			case "CONV":
				m.Converter = value
			}
			at = next
		case "FRMS", "TIME":
			width := 4
			if tag == "TIME" {
				width = 2
			}
			next := at + 4 + songs*width
			if next > limit {
				return m, songs, fmt.Errorf("sndh: truncated %s array", tag)
			}
			if tag == "FRMS" {
				if frames != nil {
					return m, songs, fmt.Errorf("sndh: duplicate FRMS array")
				}
				frames = make([]uint32, songs)
				for i := range frames {
					frames[i] = binary.BigEndian.Uint32(data[at+4+i*4:])
				}
			} else {
				if seconds != nil {
					return m, songs, fmt.Errorf("sndh: duplicate TIME array")
				}
				seconds = make([]uint16, songs)
				for i := range seconds {
					seconds[i] = binary.BigEndian.Uint16(data[at+4+i*2:])
				}
			}
			at = next
		case "FLAG", "!#SN", "#!SN":
			if at+4 >= limit {
				return m, songs, fmt.Errorf("sndh: truncated %s payload", tag)
			}
			if tag == "FLAG" && data[at+4] == '~' {
				value, next, err := headerString(data, at+5, limit)
				if err != nil {
					return m, songs, fmt.Errorf("sndh: FLAG: %w", err)
				}
				m.Flags = make([]string, songs)
				for i := range m.Flags {
					m.Flags[i] = value
				}
				at = next
				continue
			}
			values, next, err := headerStrings(data, at, songs, limit)
			if err != nil {
				return m, songs, fmt.Errorf("sndh: %s: %w", tag, err)
			}
			if tag == "FLAG" {
				m.Flags = values
			} else {
				m.SubtuneNames = values
			}
			at = next
		default:
			prefix := string(data[at : at+2])
			switch prefix {
			case "##":
				if data[at+2] < '0' || data[at+2] > '9' || data[at+3] < '0' || data[at+3] > '9' {
					return m, songs, fmt.Errorf("sndh: invalid subtune count")
				}
				declared := max(1, int(data[at+2]-'0')*10+int(data[at+3]-'0'))
				if countSeen && declared != songs {
					return m, songs, fmt.Errorf("sndh: conflicting subtune counts")
				}
				if declared != songs {
					return m, declared, nil
				}
				countSeen = true
				at += 4
			case "TA", "TB", "TC", "TD", "!V", "!#", "#!":
				value, next, err := headerString(data, at+2, limit)
				if err != nil {
					return m, songs, fmt.Errorf("sndh: %s: %w", prefix, err)
				}
				if prefix == "!#" || prefix == "#!" {
					m.DefaultSubtune, _ = strconv.Atoi(value)
				} else {
					rate, err := strconv.ParseUint(value, 10, 32)
					if err != nil || rate == 0 || rate > math.MaxInt32 {
						return m, songs, fmt.Errorf("sndh: invalid %s replay rate", prefix)
					}
					m.Rate = int(rate)
				}
				at = next
			default:
				// Unknown optional tags remain in File.Data; advance with a
				// fixed bound instead of reading untrusted strings as code.
				at++
			}
		}
	}
	return m, songs, fmt.Errorf("sndh: missing HDNS within bounded header")
}

func headerStrings(data []byte, tag, songs, limit int) ([]string, int, error) {
	tableEnd := tag + 4 + songs*2
	if tableEnd > limit {
		return nil, 0, fmt.Errorf("truncated string offset table")
	}
	values := make([]string, songs)
	next := tableEnd
	for i := range values {
		start := tag + int(binary.BigEndian.Uint16(data[tag+4+i*2:]))
		if start < tableEnd || start >= limit {
			return nil, 0, fmt.Errorf("string offset %d lies outside the header", i+1)
		}
		value, end, err := headerString(data, start, limit)
		if err != nil {
			return nil, 0, err
		}
		values[i] = value
		next = max(next, end)
	}
	return values, next, nil
}

func headerString(data []byte, start, limit int) (string, int, error) {
	if start < 0 || start >= limit {
		return "", 0, fmt.Errorf("string starts outside the header")
	}
	zero := bytes.IndexByte(data[start:limit], 0)
	if zero < 0 {
		return "", 0, fmt.Errorf("unterminated string")
	}
	value := data[start : start+zero]
	if utf8.Valid(value) {
		return string(value), start + zero + 1, nil
	}
	// Preserve legacy single-byte metadata in valid Unicode strings. Existing
	// UTF-8 text is retained above without interpreting individual bytes.
	runes := make([]rune, len(value))
	for i, b := range value {
		runes[i] = rune(b)
	}
	return string(runes), start + zero + 1, nil
}
