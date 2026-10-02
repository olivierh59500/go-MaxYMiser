package edit

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func CopyPositions(song model.Song, first, last int) ([][4]byte, error) {
	if first < 0 || last < first || last >= int(song.Length) {
		return nil, fmt.Errorf("edit: invalid song position range")
	}
	return append([][4]byte(nil), song.Orders[first:last+1]...), nil
}

// InsertPositions shifts complete order rows and retains the repeat anchor's
// musical position. The input clipboard is copied before overlapping writes.
func InsertPositions(song *model.Song, at int, positions [][4]byte) error {
	length := int(song.Length)
	if length < 1 || song.Repeat >= song.Length || at < 0 || at > length || len(positions) == 0 || length+len(positions) > 255 {
		return fmt.Errorf("edit: invalid song insertion or native position capacity exceeded")
	}
	rows := append([][4]byte(nil), positions...)
	for _, row := range rows {
		for _, id := range row {
			if id >= 240 && id != model.EmptyPattern && id != model.NoteOffPattern && id != model.LoopPattern {
				return fmt.Errorf("edit: unsupported preset pattern %02X", id)
			}
		}
	}
	copy(song.Orders[at+len(rows):length+len(rows)], song.Orders[at:length])
	copy(song.Orders[at:at+len(rows)], rows)
	if int(song.Repeat) >= at {
		song.Repeat += byte(len(rows))
	}
	song.Length = byte(length + len(rows))
	return nil
}

// DeletePositions preserves the repeat anchor where possible, or moves it to
// the successor of a removed section. At least one song position remains.
func DeletePositions(song *model.Song, first, last int) error {
	length := int(song.Length)
	if first < 0 || last < first || last >= length || song.Repeat >= song.Length || last-first+1 >= length {
		return fmt.Errorf("edit: invalid song deletion; one position must remain")
	}
	count := last - first + 1
	copy(song.Orders[first:length-count], song.Orders[last+1:length])
	for at := length - count; at < length; at++ {
		song.Orders[at] = [4]byte{255, 255, 255, 255}
	}
	if int(song.Repeat) > last {
		song.Repeat -= byte(count)
	} else if int(song.Repeat) >= first {
		song.Repeat = byte(min(first, length-count-1))
	}
	song.Length = byte(length - count)
	return nil
}

// ClonePositionPattern makes one occurrence independent without replacing
// other references to the shared original pattern.
func ClonePositionPattern(project *model.Project, position, channel int) (byte, error) {
	if position < 0 || position >= int(project.Song.Length) || channel < 0 || channel > 3 {
		return 0, fmt.Errorf("edit: invalid song position or track")
	}
	old := project.Song.Orders[position][channel]
	if old >= model.MaxPatterns || int(old) >= len(project.Song.Patterns) {
		return 0, fmt.Errorf("edit: select an ordinary stored pattern to clone")
	}
	if len(project.Song.Patterns) >= model.MaxPatterns {
		return 0, fmt.Errorf("edit: native pattern capacity reached")
	}
	id := byte(len(project.Song.Patterns))
	project.Song.Patterns = append(project.Song.Patterns, project.Song.Patterns[old])
	project.Song.Orders[position][channel] = id
	if int(project.Song.State[0]) == position {
		project.Song.State[4+channel] = id
	}
	return id, nil
}
