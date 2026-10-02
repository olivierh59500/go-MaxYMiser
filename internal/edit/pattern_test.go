package edit

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMaskedPasteModesPreserveUnselectedFields(t *testing.T) {
	var pattern model.Pattern
	pattern[10] = model.Cell{Note: 60, Instrument: 2, Volume: 4, Effect1: 'S', Parameter1: 6}
	source := []model.Cell{{Note: 64, Instrument: 0, Volume: 8, Effect1: 'T', Parameter1: 12}}
	mask := ColumnMask{true, true, false, true, false, false, false}
	if err := PasteBlock(&pattern, 10, source, mask, Overlay); err != nil {
		t.Fatal(err)
	}
	if pattern[10] != (model.Cell{Note: 64, Instrument: 2, Volume: 4, Effect1: 'T', Parameter1: 6}) {
		t.Fatal("overlay erased an empty or unselected source column")
	}
	PasteBlock(&pattern, 10, []model.Cell{{Note: 67, Instrument: 3, Effect2: 'Z'}}, AllColumns, Underlay)
	if pattern[10].Note != 64 || pattern[10].Effect2 != 'Z' {
		t.Fatal("underlay replaced occupied destination columns")
	}
	PasteBlock(&pattern, 10, source, mask, Overwrite)
	if pattern[10].Instrument != 0 || pattern[10].Volume != 4 {
		t.Fatal("overwrite ignored its mask")
	}
}

func TestRowsAndExpansionRetainRecoverableMusicalData(t *testing.T) {
	var pattern model.Pattern
	pattern[10] = model.Cell{Note: 60, Instrument: 2}
	pattern[63] = model.Cell{Note: 72}
	InsertRow(&pattern, 10)
	if pattern[10].Note != 0 || pattern[11].Note != 60 {
		t.Fatal("row insertion did not shift the track")
	}
	DeleteRow(&pattern, 10)
	if pattern[10].Note != 60 || pattern[63] != (model.Cell{}) {
		t.Fatal("row deletion did not clear the final row")
	}
	pattern[40].Note = 67
	tail := ExpandPattern(&pattern)
	if pattern[20].Note != 60 || tail[8].Note != 67 {
		t.Fatal("expansion lost the displaced tail")
	}
	ShrinkPattern(&pattern)
	if pattern[10].Note != 60 {
		t.Fatal("shrink did not recover the expanded spacing")
	}
}

func TestPCMTransposeAndRemapTreatSecondLaneAsNotes(t *testing.T) {
	var pattern model.Pattern
	pattern[0] = model.Cell{Note: 60, Instrument: 1, Volume: 16, Effect1: 64, Parameter1: 1, Effect2: 3}
	pattern[1].Note = 1
	if err := TransposeNotes(&pattern, 0, 1, 12, true); err != nil {
		t.Fatal(err)
	}
	if pattern[0].Note != 72 || pattern[0].Effect1 != 76 || pattern[1].Note != 1 {
		t.Fatal("PCM transposition altered note-off or missed the second voice")
	}
	RemapInstrument(&pattern, 0, 1, 1, 2, true)
	if pattern[0].Instrument != 2 || pattern[0].Parameter1 != 2 {
		t.Fatal("PCM sample remap missed a voice")
	}
	AdjustVolume(&pattern, 0, 1, 2, true)
	if pattern[0].Volume != 2 || pattern[0].Effect2 != 5 || pattern[1].Volume != 0 {
		t.Fatal("attenuation mishandled maximum or unchanged volume")
	}
	before := pattern
	if err := TransposeNotes(&pattern, 0, 1, 100, true); err == nil || pattern != before {
		t.Fatal("out-of-range transposition partially modified the track")
	}
}
