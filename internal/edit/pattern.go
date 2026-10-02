package edit

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type PasteMode string

const (
	Overwrite PasteMode = "overwrite"
	Overlay   PasteMode = "overlay"
	Underlay  PasteMode = "underlay"
)

type ColumnMask [7]bool

var AllColumns = ColumnMask{true, true, true, true, true, true, true}

func cellFields(cell *model.Cell) [7]*byte {
	return [7]*byte{&cell.Note, &cell.Instrument, &cell.Volume, &cell.Effect1, &cell.Parameter1, &cell.Effect2, &cell.Parameter2}
}

func validRows(first, last int) bool {
	return first >= 0 && last >= first && last < model.Rows
}

func CopyBlock(pattern model.Pattern, first, last int) ([]model.Cell, error) {
	if !validRows(first, last) {
		return nil, fmt.Errorf("edit: invalid pattern row range")
	}
	return append([]model.Cell(nil), pattern[first:last+1]...), nil
}

// PasteBlock masks individual native columns. Overlay skips empty source
// fields; underlay fills only empty destination fields. Coupled effect and
// parameter columns remain independently selectable, as in the native editor.
func PasteBlock(pattern *model.Pattern, row int, block []model.Cell, mask ColumnMask, mode PasteMode) error {
	if row < 0 || row >= model.Rows || len(block) == 0 || mode != Overwrite && mode != Overlay && mode != Underlay {
		return fmt.Errorf("edit: invalid block paste")
	}
	for i := 0; i < len(block) && row+i < model.Rows; i++ {
		source := block[i]
		src, dst := cellFields(&source), cellFields(&pattern[row+i])
		for column := range mask {
			if !mask[column] || mode == Overlay && *src[column] == 0 || mode == Underlay && *dst[column] != 0 {
				continue
			}
			*dst[column] = *src[column]
		}
	}
	return nil
}

func ClearBlock(pattern *model.Pattern, first, last int, mask ColumnMask) error {
	if !validRows(first, last) {
		return fmt.Errorf("edit: invalid pattern row range")
	}
	for row := first; row <= last; row++ {
		for column, field := range cellFields(&pattern[row]) {
			if mask[column] {
				*field = 0
			}
		}
	}
	return nil
}

func InsertRow(pattern *model.Pattern, row int) error {
	if row < 0 || row >= model.Rows {
		return fmt.Errorf("edit: invalid insertion row")
	}
	copy(pattern[row+1:], pattern[row:model.Rows-1])
	pattern[row] = model.Cell{}
	return nil
}

func DeleteRow(pattern *model.Pattern, row int) error {
	if row < 0 || row >= model.Rows {
		return fmt.Errorf("edit: invalid deletion row")
	}
	copy(pattern[row:], pattern[row+1:])
	pattern[model.Rows-1] = model.Cell{}
	return nil
}

// ExpandPattern spaces out the first 32 rows and returns the displaced tail.
// This keeps cut-off musical data available in the editor's block clipboard.
func ExpandPattern(pattern *model.Pattern) []model.Cell {
	tail := append([]model.Cell(nil), pattern[32:]...)
	before := *pattern
	*pattern = model.Pattern{}
	for row := 0; row < 32; row++ {
		pattern[row*2] = before[row]
	}
	return tail
}

func ShrinkPattern(pattern *model.Pattern) []model.Cell {
	before := *pattern
	displaced := make([]model.Cell, 32)
	*pattern = model.Pattern{}
	for row := 0; row < 32; row++ {
		pattern[row] = before[row*2]
		displaced[row] = before[row*2+1]
	}
	return displaced
}

// TransposeNotes refuses an out-of-range edit before changing any row.
// PCM patterns carry a second note in Effect1, not a YM tracker effect.
func TransposeNotes(pattern *model.Pattern, first, last, semitones int, pcm bool) error {
	if !validRows(first, last) {
		return fmt.Errorf("edit: invalid pattern row range")
	}
	result := *pattern
	for row := first; row <= last; row++ {
		fields := []*byte{&result[row].Note}
		if pcm {
			fields = append(fields, &result[row].Effect1)
		}
		for _, field := range fields {
			if *field <= model.NoteOff {
				continue
			}
			note := int(*field) + semitones
			if note < 2 || note > 127 {
				return fmt.Errorf("edit: transposition exceeds the note range at row %02X", row)
			}
			*field = byte(note)
		}
	}
	*pattern = result
	return nil
}

func AdjustVolume(pattern *model.Pattern, first, last, attenuation int, pcm bool) error {
	if !validRows(first, last) {
		return fmt.Errorf("edit: invalid pattern row range")
	}
	for row := first; row <= last; row++ {
		fields := []*byte{&pattern[row].Volume}
		if pcm {
			fields = append(fields, &pattern[row].Effect2)
		}
		for _, field := range fields {
			if *field == 0 {
				continue
			}
			value := max(0, min(15, int(*field&15)+attenuation))
			if value == 0 {
				value = 16
			}
			*field = byte(value)
		}
	}
	return nil
}

func RemapInstrument(pattern *model.Pattern, first, last int, from, to byte, pcm bool) error {
	limit := byte(model.MaxInstruments)
	if pcm {
		limit = model.MaxSamples
	}
	if !validRows(first, last) || from == 0 || to == 0 || from > limit || to > limit {
		return fmt.Errorf("edit: invalid instrument remap")
	}
	for row := first; row <= last; row++ {
		fields := []*byte{&pattern[row].Instrument}
		if pcm {
			fields = append(fields, &pattern[row].Parameter1)
		}
		for _, field := range fields {
			if *field == from {
				*field = to
			}
		}
	}
	return nil
}
