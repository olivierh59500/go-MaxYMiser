// Package native reads and writes MaxYMiser's editable native data formats.
package native

import (
	"bytes"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

const songHeader = 8 + 64 + 256*4 + 2

func DecodeSong(data []byte) (model.Song, error) {
	var song model.Song
	if len(data) < songHeader || !bytes.Equal(data[:3], []byte("MYM")) || !bytes.Equal(data[4:8], []byte("TRAK")) {
		return song, fmt.Errorf("native: invalid MYS header")
	}
	song.Version = data[3] - '0'
	copy(song.State[:], data[8:72])
	for i := range song.Orders {
		copy(song.Orders[i][:], data[72+i*4:76+i*4])
	}
	song.Length, song.Repeat = data[1096], data[1097]
	if song.Length == 0 {
		return song, fmt.Errorf("native: empty order list")
	}
	if song.Repeat >= song.Length {
		return song, fmt.Errorf("native: repeat exceeds song length")
	}
	offset := songHeader
	for offset < len(data) {
		if len(song.Patterns) >= model.MaxPatterns {
			return song, fmt.Errorf("native: too many patterns")
		}
		var pattern model.Pattern
		for row := 0; row < model.Rows; {
			if len(data)-offset < 8 {
				return song, fmt.Errorf("native: truncated pattern %d row %d", len(song.Patterns), row)
			}
			record := data[offset : offset+8]
			offset += 8
			skip := int(record[7])
			if row+skip >= model.Rows {
				return song, fmt.Errorf("native: RLE run exceeds pattern length")
			}
			pattern[row] = model.Cell{Note: record[0], Instrument: record[1], Volume: record[2], Effect1: record[3], Parameter1: record[4], Effect2: record[5], Parameter2: record[6]}
			row += skip + 1
		}
		song.Patterns = append(song.Patterns, pattern)
	}
	for pos := 0; pos < int(song.Length); pos++ {
		for _, id := range song.Orders[pos] {
			if id < 240 && int(id) >= len(song.Patterns) {
				return song, fmt.Errorf("native: missing pattern %d at song position %d", id, pos)
			}
		}
	}
	return song, nil
}

func EncodeSong(song model.Song) ([]byte, error) {
	if song.Length == 0 || song.Repeat >= song.Length || len(song.Patterns) > model.MaxPatterns {
		return nil, fmt.Errorf("native: invalid song dimensions")
	}
	out := make([]byte, songHeader)
	copy(out, []byte("MYM0TRAK"))
	out[3] = '0' + song.Version
	copy(out[8:72], song.State[:])
	for i := range song.Orders {
		copy(out[72+i*4:76+i*4], song.Orders[i][:])
	}
	out[1096], out[1097] = song.Length, song.Repeat
	for _, pattern := range song.Patterns {
		for row := 0; row < model.Rows; {
			c := pattern[row]
			skip := 0
			for row+skip+1 < model.Rows && pattern[row+skip+1] == (model.Cell{}) {
				skip++
			}
			out = append(out, c.Note, c.Instrument, c.Volume, c.Effect1, c.Parameter1, c.Effect2, c.Parameter2, byte(skip))
			row += skip + 1
		}
	}
	return out, nil
}
