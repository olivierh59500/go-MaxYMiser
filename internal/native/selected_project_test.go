package native

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// These constructed layouts relocate the empty sample trailer after the song.
// Pointer cells and tag placement are independent of the decoder under test.
func displacedEmptySelector(t *testing.T, projects []*model.Project) []byte {
	t.Helper()
	out := append([]byte(nil), syntheticMultiSelector(t, projects)[:512]...)
	for slot, p := range projects {
		voice, err := EncodeVoiceBank(p.Bank)
		if err != nil {
			t.Fatal(err)
		}
		voice = voice[:len(voice)-16] // Remove tag and eight guards from this span.
		music, err := EncodeSong(p.Song)
		if err != nil {
			t.Fatal(err)
		}
		bankAt := len(out)
		out = append(out, voice...)
		songAt := len(out)
		out = append(out, music...)
		tag := []byte("MYM1DIGI")
		tag[3] = '0' + p.Bank.SampleVersion
		out = append(out, tag...)
		out = append(out, make([]byte, 8)...)
		out = append(out, 0x45, 0x79)
		code := 192 + slot*40
		binary.BigEndian.PutUint32(out[code+2:], uint32(bankAt-500))
		binary.BigEndian.PutUint32(out[code+8:], uint32(songAt-504))
		binary.BigEndian.PutUint32(out[code+14:], uint32(len(music)+18))
		binary.BigEndian.PutUint32(out[code+28:], uint32(songAt+22-code))
	}
	return out
}

func TestSelectorRecoversDisplacedEmptySampleTagAndAllNativeSongs(t *testing.T) {
	p, q := model.New(), model.New()
	p.Song.Patterns[0][0].Note, q.Song.Patterns[0][0].Note = 60, 72
	q.Bank.Version, q.Bank.SampleVersion = 0, 0
	q.Bank.Instruments[0].SetName("Legacy sound")
	raw := displacedEmptySelector(t, []*model.Project{p, q})
	before := append([]byte(nil), raw...)
	got, err := DecodeContainers(raw)
	if err != nil || len(got) != 2 {
		t.Fatalf("optimized native songs were not recovered: %v", err)
	}
	for i, want := range []*model.Project{p, q} {
		if !reflect.DeepEqual(got[i].Song, want.Song) || got[i].Bank.Instruments != want.Bank.Instruments || got[i].Bank.Version != want.Bank.Version || got[i].Bank.SampleVersion != want.Bank.SampleVersion || got[i].Subtune != i+1 || got[i].Subtunes != 2 {
			t.Fatal("recovery changed the song, instrument definitions or numbering")
		}
		for n := 0; n < want.Bank.SequenceCount; n++ {
			if got[i].Bank.Sequences[n] != want.Bank.Sequences[n] {
				t.Fatal("recovery changed a native sequence")
			}
		}
		for _, sample := range got[i].Bank.Samples {
			if len(sample.PCM) != 0 {
				t.Fatal("recovery invented waveform bytes")
			}
		}
		if !bytes.Equal(got[i].Bank.Samples[7].Trailer, []byte{0, 0x45, 0x79}) {
			t.Fatal("opaque trailing bytes were lost")
		}
		encoded, err := EncodeVoiceBank(got[i].Bank)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeVoiceBank(encoded); err != nil {
			t.Fatalf("recovered bank cannot be saved as MYV: %v", err)
		}
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("recovery modified its input")
	}
	template, err := ParseMultiSNDHTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var projects []*model.Project
	for _, item := range got {
		projects = append(projects, &model.Project{Title: item.Title, Author: item.Author, Song: item.Song, Bank: item.Bank})
	}
	projects[1].Song.Patterns[0][0].Note = 77
	exported, err := EncodeMultiSNDH(template, projects, []time.Duration{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeContainers(exported)
	if err != nil || len(again) != 2 || again[1].Song.Patterns[0][0].Note != 77 {
		t.Fatalf("recovered collection cannot be edited and exported: %v", err)
	}
}

func TestDisplacedBankRecoveryRejectsMissingAudioAndInvalidBoundaries(t *testing.T) {
	p, q := model.New(), model.New()
	raw := displacedEmptySelector(t, []*model.Project{p, q})
	bankAt := 512
	songAt := 504 + int(binary.BigEndian.Uint32(raw[200:]))
	tailAt := songAt + int(binary.BigEndian.Uint32(raw[206:])) - 18
	for name, mutate := range map[string]func([]byte){
		"nonempty missing sample":     func(b []byte) { binary.BigEndian.PutUint16(b[bankAt+2088:], 1) },
		"invalid sequence length":     func(b []byte) { b[bankAt+bankHeader+126] = 64 },
		"unknown sample version":      func(b []byte) { b[tailAt+3] = '2' },
		"nonempty sample guard":       func(b []byte) { b[tailAt+8] = 1 },
		"invalid song length":         func(b []byte) { binary.BigEndian.PutUint32(b[206:], uint32(len(raw))) },
		"invalid replay rate pointer": func(b []byte) { b[220] ^= 0x7f },
	} {
		t.Run(name, func(t *testing.T) {
			bad := append([]byte(nil), raw...)
			mutate(bad)
			if _, err := ParseMultiSNDHTemplate(bad); err == nil {
				t.Fatal("invalid native source was normalized into a complete collection")
			}
		})
	}
	if _, err := DecodeVoiceBank(raw[bankAt:songAt]); err == nil {
		t.Fatal("standalone MYV decoding silently repaired stale pointers")
	}
}
