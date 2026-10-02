package edit

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestSongRemapSeparatesYMSoundsAndPCMSamples(t *testing.T) {
	p := model.New()
	p.Song.Patterns = append(p.Song.Patterns, model.Pattern{})
	p.Song.Orders[0] = [4]byte{0, 1, 255, 3}
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Song.Patterns[3][0] = model.Cell{Note: 60, Instrument: 1, Effect1: 64, Parameter1: 1}
	if err := RemapSong(p, 1, 2, false); err != nil {
		t.Fatal(err)
	}
	if p.Song.Patterns[0][0].Instrument != 2 || p.Song.Patterns[3][0].Instrument != 1 {
		t.Fatal("YM remap changed PCM data")
	}
	if err := RemapSong(p, 1, 3, true); err != nil {
		t.Fatal(err)
	}
	if p.Song.Patterns[3][0].Instrument != 3 || p.Song.Patterns[3][0].Parameter1 != 3 {
		t.Fatal("PCM remap missed a sample voice")
	}
	p.Song.Orders[0][2] = 3
	before := p.Clone()
	if err := RemapSong(p, 3, 4, true); err == nil || p.Song.Patterns[3] != before.Song.Patterns[3] {
		t.Fatal("ambiguous remap changed a shared pattern")
	}
}
