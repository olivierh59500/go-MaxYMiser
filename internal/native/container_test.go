package native

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestLegacySequenceBankRetainsIndependentSignedSampleVersion(t *testing.T) {
	p := model.New()
	p.Bank.Version = 0
	p.Bank.SampleVersion = 1
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
	raw, err := EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeVoiceBank(raw)
	if err != nil || got.Version != 0 || got.SampleVersion != 1 {
		t.Fatalf("independent native versions lost: %v", err)
	}
	again, err := EncodeVoiceBank(got)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("legacy INST / signed DIGI bank changed on round-trip")
	}
}

func TestNativeContainerCanPlaceBankSamplesBeforeItsSong(t *testing.T) {
	p := model.Demo()
	bank, err := EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	song, err := EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	prefix := make([]byte, 32)
	copy(prefix[12:], "SNDH")
	raw := append(prefix, bank...)
	raw = append(raw, song...)
	got, err := DecodeContainer(raw)
	if err != nil || len(got.Song.Patterns) != len(p.Song.Patterns) || got.Bank.Instruments != p.Bank.Instruments {
		t.Fatalf("bank-before-song variant: %v", err)
	}
}

func TestNativeContainerSeparatesTwoSubtunesWithoutScanningSampleTags(t *testing.T) {
	p := model.Demo()
	first := syntheticSNDH(t, p)
	q := model.New()
	q.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1}
	bank, err := EncodeVoiceBank(q.Bank)
	if err != nil {
		t.Fatal(err)
	}
	song, err := EncodeSong(q.Song)
	if err != nil {
		t.Fatal(err)
	}
	digi := int(binary.BigEndian.Uint32(bank[:4])) - 8
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(bank[i*4:], binary.BigEndian.Uint32(bank[i*4:])+uint32(len(song)))
	}
	second := append([]byte(nil), bank[:digi]...)
	second = append(second, song...)
	second = append(second, bank[digi:]...)
	raw := append(first, second...)
	projects, err := DecodeContainers(raw)
	if err != nil || len(projects) != 2 || projects[1].Song.Patterns[0][0].Note != 69 {
		t.Fatalf("subtune payloads were confused: count=%d err=%v", len(projects), err)
	}
}

func TestSNDHMetadataWordsInTitleAreNotTreatedAsHeaderTags(t *testing.T) {
	p := model.Demo()
	p.Title = "ANOTHER TIME"
	template, err := ParseSNDHTemplate(syntheticSNDH(t, p))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeSNDH(template, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeContainer(raw)
	if err != nil || got.Title != p.Title || got.Author != p.Author {
		t.Fatalf("title text overwrote another metadata field: %+v %v", got, err)
	}
}
