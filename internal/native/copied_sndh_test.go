package native

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestSingleCopiedSongExportRetainsWrapperRateAndRelocatesPayload(t *testing.T) {
	for _, direct := range []bool{false, true} {
		p := model.New()
		p.Title, p.Author = "Collection", "Author"
		p.Song.SetTickRate(85)
		raw := syntheticMultiSelector(t, []*model.Project{p})
		if direct {
			// Older wrappers address the offset table with a direct PC LEA.
			binary.BigEndian.PutUint16(raw[162:], 500-162)
			clear(raw[164:170])
		}
		template, err := ParseSNDHTemplate(raw)
		if err != nil {
			t.Fatal(err)
		}
		before := append([]byte(nil), template.Prefix...)
		p.Song.Patterns[0][7] = model.Cell{Note: 72, Instrument: 1}
		p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255, 1}
		p.Bank.SequenceCount++
		p.Bank.Sequences[p.Bank.SequenceCount-1] = model.Sequence{Length: 1}
		out, err := EncodeSNDH(template, p, 123*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeContainer(out)
		if err != nil || !reflect.DeepEqual(got.Song, p.Song) || !bytes.Equal(got.Bank.Samples[0].PCM, p.Bank.Samples[0].PCM) || got.Bank.SequenceCount != p.Bank.SequenceCount {
			t.Fatalf("copied-song export lost edited data: direct=%v, %v", direct, err)
		}
		if !bytes.Equal(before, template.Prefix) || !bytes.Equal(out[:12], raw[:12]) || !bytes.Contains(out[:len(before)], []byte("TC200\x00")) || binary.BigEndian.Uint16(out[96:]) != 123 {
			t.Fatal("export changed source, entry points, outer rate or duration")
		}
		if _, err := ParseSNDHTemplate(out); err != nil {
			t.Fatal("rebuilt copied-song template is no longer reusable:", err)
		}
		if _, err := ParseMultiSNDHTemplate(out); err == nil {
			t.Fatal("single-song wrapper was exposed as a complete multi-song template")
		}
	}
}

func TestSingleCopiedSongTemplateRejectsUnverifiedCopyAndRateOperands(t *testing.T) {
	p := model.New()
	p.Title, p.Author = "Collection", "Author"
	raw := syntheticMultiSelector(t, []*model.Project{p})
	for _, at := range []int{194, 200, 206, 216, 220, 224} {
		bad := append([]byte(nil), raw...)
		bad[at] ^= 0x7f
		if _, err := ParseSNDHTemplate(bad); err == nil {
			t.Fatalf("corrupt copied-song operand at %d was accepted", at)
		}
	}
}
