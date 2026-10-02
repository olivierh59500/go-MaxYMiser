package edit

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestClearSongRetainsSoundBankAndNativeSettings(t *testing.T) {
	p := model.Demo()
	p.Author, p.Year = "Composer", "2026"
	p.Song.State[40], p.Song.State[57] = 7, 5
	p.Song.State[52] = 7
	p.Song.SetTickRate(100)
	p.Song.SetSpeed(8)
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
	p.Bank.Samples[0].Trailer = []byte{0}
	before := p.Clone()
	ClearSong(p)
	if !reflect.DeepEqual(p.Bank, before.Bank) || p.Title != before.Title || p.Author != before.Author || p.Year != before.Year {
		t.Fatal("clearing song content changed reusable sounds or metadata")
	}
	if p.Song.Length != 1 || p.Song.Repeat != 0 || len(p.Song.Patterns) != 3 || p.Song.TickRate() != 100 || p.Song.Speed() != 8 || p.Song.State[40] != 7 || p.Song.State[57] != 5 {
		t.Fatal("clearing song content lost personal playback settings or retained arrangement data")
	}
	for _, pattern := range p.Song.Patterns {
		if pattern != (model.Pattern{}) {
			t.Fatal("cleared song retained notes or effects")
		}
	}
	for _, start := range []int{4, 52} {
		for channel, id := range p.Song.Orders[0] {
			if p.Song.State[start+channel] != id {
				t.Fatal("cleared song retained stale native pattern references")
			}
		}
	}
	raw, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := native.DecodeSong(raw); err != nil {
		t.Fatalf("cleared song cannot be reopened by the native codec: %v", err)
	}
}

func TestClearBankRetainsNotesAndLegacyNativeVersions(t *testing.T) {
	p := model.Demo()
	p.Bank.Version, p.Bank.SampleVersion = 0, 1
	p.Bank.Samples[0].PCM = []byte{1, 2, 3}
	p.Bank.Samples[0].Trailer = []byte{0}
	before := p.Clone()
	ClearBank(p)
	if !reflect.DeepEqual(p.Song, before.Song) || p.Title != before.Title || p.Bank.Version != 0 || p.Bank.SampleVersion != 1 || p.Bank.SequenceCount != 1 {
		t.Fatal("clearing sounds changed the composition or native versions")
	}
	for _, instrument := range p.Bank.Instruments {
		if instrument != (model.Instrument{}) {
			t.Fatal("cleared bank retained an instrument definition")
		}
	}
	for _, sample := range p.Bank.Samples {
		if len(sample.PCM) != 0 || len(sample.Trailer) != 0 {
			t.Fatal("cleared bank retained sample storage")
		}
	}
	raw, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := native.DecodeVoiceBank(raw)
	if err != nil || bank.Version != 0 || bank.SequenceCount != 1 {
		t.Fatalf("cleared legacy bank cannot be reopened: %v", err)
	}
}
