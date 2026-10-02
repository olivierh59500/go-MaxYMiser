package native

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// syntheticMultiSelector contains constructed selector instructions and native
// data, not a bundled Atari sound driver. Actual execution is verified locally.
func syntheticMultiSelector(t *testing.T, projects []*model.Project) []byte {
	t.Helper()
	prefix := make([]byte, 512)
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint16(prefix[i*4:], 0x6000)
		binary.BigEndian.PutUint16(prefix[i*4+2:], uint16(160-i*4-2))
	}
	copy(prefix[12:], "SNDHCOMM")
	copy(prefix[20:], "Author")
	copy(prefix[48:], "TITL")
	copy(prefix[52:], "Collection")
	copy(prefix[80:], "##02\x00TC200\x00")
	copy(prefix[92:], "TIME")
	copy(prefix[100:], "HDNS")
	copy(prefix[160:], []byte{0x41, 0xfa, 0xff, 0xfe, 0xd1, 0xfc})
	binary.BigEndian.PutUint32(prefix[166:], 500-160)
	template := MultiSNDHTemplate{Prefix: prefix, pointers: 500}
	for slot := 0; slot < 2; slot++ {
		at := 192 + slot*40
		for _, offset := range []int{0, 6, 12} {
			copy(prefix[at+offset:], []byte{0x20, 0xfc})
		}
		copy(prefix[at+18:], []byte{0x41, 0xfa, 0, 0, 0x43, 0xfa, 0xff, 0xe8, 0xd3, 0xfc})
		copy(prefix[at+32:], []byte{0x10, 0xd1})
		template.slots = append(template.slots, multiSlot{at, at + 28, 1})
	}
	// Build source pointers directly; EncodeMultiSNDH subsequently validates
	// them using the same strict parser used for runtime templates.
	out := append([]byte(nil), prefix...)
	for slot, p := range projects {
		voice, e := EncodeVoiceBank(p.Bank)
		if e != nil {
			t.Fatal(e)
		}
		song, e := EncodeSong(p.Song)
		if e != nil {
			t.Fatal(e)
		}
		bankAt := len(out)
		out = append(out, voice...)
		if len(out)%2 != 0 {
			out = append(out, 0)
		}
		songAt := len(out)
		out = append(out, song...)
		at := template.slots[slot].instruction
		binary.BigEndian.PutUint32(out[at+2:], uint32(bankAt-500))
		binary.BigEndian.PutUint32(out[at+8:], uint32(songAt-504))
		binary.BigEndian.PutUint32(out[at+14:], uint32(len(song)))
		binary.BigEndian.PutUint32(out[at+28:], uint32(songAt+22-at))
	}
	return out
}

func TestMultiSongExportRelocatesAllSelectorAndRateOperands(t *testing.T) {
	p, q := model.New(), model.New()
	p.Title, p.Author = "Collection", "Author"
	q.Title, q.Author = p.Title, p.Author
	raw := syntheticMultiSelector(t, []*model.Project{p, q})
	template, err := ParseMultiSNDHTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), template.Prefix...)
	p.Song.Patterns[0][1] = model.Cell{Note: 60, Instrument: 1}
	p.Bank.Samples[0].PCM = []byte{1, 2, 3, 4, 5}
	q.Song.SetTickRate(100)
	q.Song.Patterns[2][33].Note = 72
	out, err := EncodeMultiSNDH(template, []*model.Project{p, q}, []time.Duration{time.Minute, 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeContainers(out)
	if err != nil || len(got) != 2 || !reflect.DeepEqual(got[0].Song, p.Song) || !reflect.DeepEqual(got[1].Song, q.Song) || !bytes.Equal(got[0].Bank.Samples[0].PCM, p.Bank.Samples[0].PCM) {
		t.Fatalf("multi-song relocation lost an editable payload: %v", err)
	}
	if !bytes.Equal(template.Prefix, before) || !bytes.Equal(raw[:12], out[:12]) || binary.BigEndian.Uint16(out[96:]) != 60 || binary.BigEndian.Uint16(out[98:]) != 120 {
		t.Fatal("template mutation, entry-point change or duration-array loss")
	}
	if _, err := ParseSNDHTemplate(out); err == nil {
		t.Fatal("multi-song executable was accepted as a single-song template")
	}
}

func TestMultiSongExportRejectsIncompleteSelectorsAndSlotReplacement(t *testing.T) {
	p, q := model.New(), model.New()
	p.Title, p.Author = "Collection", "Author"
	raw := syntheticMultiSelector(t, []*model.Project{p, q})
	template, err := ParseMultiSNDHTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeMultiSNDH(template, []*model.Project{p}, []time.Duration{0}); err == nil {
		t.Fatal("export discarded a selector-referenced song")
	}
	for _, at := range []int{194, 200, 208, 220, 240} {
		bad := append([]byte(nil), raw...)
		bad[at] ^= 0x7f
		if _, err := ParseMultiSNDHTemplate(bad); err == nil {
			t.Fatalf("invalid selector operand at %d was accepted", at)
		}
	}
}

func FuzzNativeMultiSongSelector(f *testing.F) {
	f.Add([]byte("SNDH"))
	f.Add(make([]byte, 64))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		ParseMultiSNDHTemplate(data)
	})
}
