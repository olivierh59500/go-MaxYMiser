package native

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// syntheticSNDH contains only an inert original test prefix, not an Atari
// replayer. Execution interoperability is checked separately with local sources.
func syntheticSNDH(t *testing.T, p *model.Project) []byte {
	t.Helper()
	prefix := make([]byte, 224)
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint16(prefix[i*4:], 0x6000)
		binary.BigEndian.PutUint16(prefix[i*4+2:], uint16(200-i*4-2))
	}
	copy(prefix[12:], "SNDHCOMM")
	copy(prefix[20:], "Test author")
	copy(prefix[48:], "TITL")
	copy(prefix[52:], "Test title")
	copy(prefix[80:], "TC050")
	copy(prefix[90:], "TIME")
	copy(prefix[96:], "HDNS")
	binary.BigEndian.PutUint32(prefix[216:], 8)
	voice, err := EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	song, err := EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	digi := int(binary.BigEndian.Uint32(voice)) - 8
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(voice[i*4:], binary.BigEndian.Uint32(voice[i*4:])+uint32(len(song)))
	}
	binary.BigEndian.PutUint32(prefix[220:], uint32(len(prefix)+digi-220))
	result := append(prefix, voice[:digi]...)
	result = append(result, song...)
	return append(result, voice[digi:]...)
}

func TestSNDHExportKeepsPrefixAndRebuildsEditablePointers(t *testing.T) {
	original := model.New()
	raw := syntheticSNDH(t, original)
	template, err := ParseSNDHTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := model.Demo()
	p.Song.SetTickRate(100)
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255, 13}
	out, err := EncodeSNDH(template, p, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeContainer(out)
	if err != nil || !reflect.DeepEqual(got.Song, p.Song) || got.Bank.Instruments != p.Bank.Instruments || !bytes.Equal(got.Bank.Samples[0].PCM, p.Bank.Samples[0].PCM) {
		t.Fatalf("SNDH lost editable data: %v", err)
	}
	for i := 0; i < p.Bank.SequenceCount; i++ {
		if got.Bank.Sequences[i] != p.Bank.Sequences[i] {
			t.Fatal("SNDH changed a serialized sequence")
		}
	}
	if !bytes.Equal(out[:12], raw[:12]) || out[200] != raw[200] || string(out[80:85]) != "TC100" || binary.BigEndian.Uint16(out[94:96]) != 180 {
		t.Fatal("SNDH entry points or metadata were corrupted")
	}
	if got.Title != p.Title || got.Author != p.Author {
		t.Fatal("SNDH lost song metadata")
	}
}

func TestSNDHRejectsInvalidPrefixWithoutMutatingTemplate(t *testing.T) {
	raw := syntheticSNDH(t, model.New())
	raw[218] = 1
	if _, err := ParseSNDHTemplate(raw); err == nil {
		t.Fatal("unsupported pointer layout accepted")
	}
	template, err := ParseSNDHTemplate(syntheticSNDH(t, model.New()))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), template.Prefix...)
	p := model.Demo()
	p.Title = "This title is longer than the metadata field in this replay prefix"
	if _, err := EncodeSNDH(template, p, 0); err == nil || !bytes.Equal(before, template.Prefix) {
		t.Fatal("invalid metadata modified the source template")
	}
}
