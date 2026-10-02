package native

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestMYI3RoundTripIncludesPWMAndSignedSample(t *testing.T) {
	p := model.Demo()
	p.Bank.Instruments[1][36] = 1
	p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255}
	p.Bank.Instruments[1][55] = 7
	p.Bank.Sequences[7] = model.Sequence{Length: 3, Repeat: 1, Values: [63]uint16{0, 255, 128}}
	file, err := ExportInstrument(&p.Bank, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeInstrument(file)
	if err != nil || len(raw) != 1084 || string(raw[:8]) != "MYM3.MYI" {
		t.Fatalf("wrong native instrument layout: %d %v", len(raw), err)
	}
	got, err := DecodeInstrument(raw)
	if err != nil || got.Instrument.Name() != "Chord pulse" || got.Sequences[7] != p.Bank.Sequences[7] || !bytes.Equal(got.Sample, p.Bank.Samples[0].PCM) {
		t.Fatalf("instrument lost definitions: %+v %v", got, err)
	}
	again, err := EncodeInstrument(got)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("MYI did not round-trip byte for byte")
	}
	for at := 0; at < 1080; at++ {
		if _, err := DecodeInstrument(raw[:at]); err == nil {
			t.Fatalf("truncated instrument accepted at %d", at)
		}
	}
}

func TestMYIVersionsRetainTheirSequenceWordCounts(t *testing.T) {
	for version := byte(0); version <= 3; version++ {
		file := InstrumentFile{Version: version}
		file.Instrument.SetName("Old voice")
		file.Sequences[0] = model.Sequence{Length: 2, Repeat: 1, Values: [63]uint16{15, 8}}
		raw, err := EncodeInstrument(file)
		if err != nil {
			t.Fatal(err)
		}
		want := []int{504, 504, 952, 1080}[version]
		got, err := DecodeInstrument(raw)
		if len(raw) != want || err != nil || got.Sequences[0] != file.Sequences[0] {
			t.Fatalf("version %d length=%d want=%d err=%v", version, len(raw), want, err)
		}
	}
}

func TestIndependentMYIVersionFixturesKeepSignedWordsAndSampleBoundaries(t *testing.T) {
	for version := byte(0); version <= 3; version++ {
		stride, count := 64, 7
		if version >= 2 {
			stride = 128
		}
		if version == 3 {
			count = 8
		}
		raw := make([]byte, 56+count*stride)
		copy(raw, "MYM0.MYI")
		raw[3] = '0' + version
		raw[8+36] = 1
		for sequence := 0; sequence < count; sequence++ {
			at := 56 + sequence*stride
			binary.BigEndian.PutUint16(raw[at:], 0xffff)
			binary.BigEndian.PutUint16(raw[at+stride-4:], uint16(0x8000+sequence))
			raw[at+stride-2], raw[at+stride-1] = byte(stride/2-1), byte(stride/2-2)
		}
		wantSample := []byte{0, 127, 128, 255}
		if version == 0 {
			raw = append(raw, 0, 8, 13, 15, 255)
			wantSample = []byte{128, 148, 7, 127}
		} else {
			raw = append(raw, wantSample...)
		}
		file, err := DecodeInstrument(raw)
		if err != nil || !bytes.Equal(file.Sample, wantSample) {
			t.Fatalf("MYI%d sample started at a later version's offset: %v", version, err)
		}
		for sequence := 0; sequence < count; sequence++ {
			seq := file.Sequences[sequence]
			if seq.Length != byte(stride/2-1) || seq.Repeat != byte(stride/2-2) || seq.Values[0] != 0xffff || seq.Values[stride/2-2] != uint16(0x8000+sequence) {
				t.Fatalf("MYI%d sequence %d lost its final signed word or native loop", version, sequence)
			}
		}
	}
}

func TestLegacyMYI0DigiSamplesUseTheOriginalInverseDACMapping(t *testing.T) {
	file := InstrumentFile{Version: 0}
	file.Instrument[36] = 1
	raw, err := EncodeInstrument(file)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, 0, 8, 13, 15, 255)
	got, err := DecodeInstrument(raw)
	if err != nil || !bytes.Equal(got.Sample, []byte{128, 148, 7, 127}) {
		t.Fatalf("legacy DAC conversion differs: %v %v", got.Sample, err)
	}
}

func TestLegacyMYI0MaximumSampleKeepsItsSeparateEndMarker(t *testing.T) {
	file := InstrumentFile{Version: 0}
	file.Instrument[36] = 1
	raw, err := EncodeInstrument(file)
	if err != nil {
		t.Fatal(err)
	}
	payload := append(bytes.Repeat([]byte{8}, 32768), 255)
	got, err := DecodeInstrument(append(append([]byte(nil), raw...), payload...))
	if err != nil || len(got.Sample) != 32768 || got.Sample[32767] != legacyDACPCM[8] {
		t.Fatalf("a legacy end marker reduced the maximum sample length: %v", err)
	}
	tooLong := append(bytes.Repeat([]byte{8}, 32769), 255)
	if _, err := DecodeInstrument(append(append([]byte(nil), raw...), tooLong...)); err == nil {
		t.Fatal("an oversized legacy sample became a valid signed payload")
	}
}

func TestInstrumentImportPreservesOtherSoundsAndFailsAtomicallyWhenFull(t *testing.T) {
	p := model.Demo()
	file, _ := ExportInstrument(&p.Bank, 1)
	before := p.Clone()
	if err := ImportInstrument(&p.Bank, 7, file); err != nil {
		t.Fatal(err)
	}
	if p.Bank.Instruments[1] != before.Bank.Instruments[1] || p.Bank.Sequences[3] != before.Bank.Sequences[3] {
		t.Fatal("import overwrote an existing sound")
	}
	arp := p.Bank.Instruments[7][49]
	if arp == 3 || p.Bank.Sequences[arp] != file.Sequences[1] {
		t.Fatal("import did not remap its independent arpeggio definition")
	}
	for i := range p.Bank.Sequences {
		p.Bank.Sequences[i].Values[0] = 1
	}
	bank := p.Bank
	if err := ImportInstrument(&p.Bank, 8, file); err == nil || p.Bank.Instruments != bank.Instruments || p.Bank.Sequences != bank.Sequences {
		t.Fatal("exhausted import partially altered the bank")
	}
}
