package native

import (
	"bytes"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"reflect"
	"testing"
)

func TestSongRoundTripRetainsOrdersReservedStateAndEffects(t *testing.T) {
	p := model.New()
	p.Song.Length = 2
	p.Song.Repeat = 1
	p.Song.State[63] = 0xa5
	p.Song.Orders[1] = [4]byte{2, 1, 0, model.NoteOffPattern}
	p.Song.Patterns[0][0] = model.Cell{Note: 45, Instrument: 2, Volume: 16, Effect1: 'X', Parameter1: 0x47, Effect2: 'S', Parameter2: 6}
	p.Song.Patterns[0][63] = model.Cell{Note: model.NoteOff}
	raw, err := EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSong(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p.Song) {
		t.Fatal("song lost musical data or reserved state")
	}
	again, err := EncodeSong(got)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("song bytes were not stable")
	}
	for n := 0; n < songHeader; n++ {
		if _, err := DecodeSong(raw[:n]); err == nil {
			t.Fatalf("truncation at %d accepted", n)
		}
	}
}

func TestVoiceBankRoundTripRetainsSequencesSignedSamplesAndTrailers(t *testing.T) {
	p := model.New()
	p.Bank.Instruments[0][63] = 0x5a
	p.Bank.Sequences[7] = model.Sequence{Length: 3, Repeat: 1}
	p.Bank.Sequences[7].Values[0] = 0xffff
	p.Bank.Sequences[7].Values[1] = 0x0804
	p.Bank.Sequences[7].Values[2] = 15
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255, 5}
	p.Bank.Samples[0].Trailer = []byte{0, 0, 0, 0}
	p.Bank.Samples[0].Parameters[3] = 0x7a
	raw, err := EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeVoiceBank(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Instruments[0] != p.Bank.Instruments[0] || got.Sequences[7] != p.Bank.Sequences[7] || !bytes.Equal(got.Samples[0].PCM, p.Bank.Samples[0].PCM) {
		t.Fatal("voice bank changed native instrument or sample data")
	}
	again, err := EncodeVoiceBank(got)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("voice bank bytes were not stable")
	}
	raw[0] = 0xff
	if _, err := DecodeVoiceBank(raw); err == nil {
		t.Fatal("out-of-range sample pointer accepted")
	}
}
