package ymimport

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestSourceBankTranslatesEnvelopeCadenceAndSignedArpeggios(t *testing.T) {
	s := SourceScore{Instruments: []SourceInstrument{{ID: 3, Settings: []byte{0, 0, 1, 2, 1, 3}, VolumeSequence: []byte{15, 14, 12}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0, 7, -5}, Repeat: 0}}}}
	bank, report, err := SourceVoiceBank(s)
	if err != nil || len(report.Converted) != 1 || len(report.Unsupported) != 0 {
		t.Fatalf("native bank was not translated: %+v %v", report, err)
	}
	p := model.New()
	p.Bank = bank
	e := replay.New(p)
	e.Trigger(0, 60, 4)
	var volumes []byte
	var pitches []uint16
	for i := 0; i < 13; i++ {
		e.Tick()
		volumes = append(volumes, e.Registers[8])
		pitches = append(pitches, uint16(e.Registers[0])|uint16(e.Registers[1])<<8)
	}
	if !reflect.DeepEqual(volumes, []byte{15, 15, 15, 14, 14, 14, 14, 12, 12, 12, 12, 12, 12}) {
		t.Fatalf("native first-step envelope timing changed: %v", volumes)
	}
	if !reflect.DeepEqual(pitches[:6], []uint16{478, 319, 638, 478, 319, 638}) {
		t.Fatalf("signed source arpeggio or loop changed: %v", pitches)
	}
	raw, err := native.EncodeVoiceBank(bank)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := native.DecodeVoiceBank(raw)
	if err != nil || loaded.Instruments[3] != bank.Instruments[3] || loaded.Sequences != bank.Sequences {
		t.Fatalf("translated definitions cannot be edited as native MYV: %v", err)
	}
}

func TestSourceBankRejectsUnsupportedHardwareWithoutInventingSounds(t *testing.T) {
	s := SourceScore{Instruments: []SourceInstrument{{ID: 2, Settings: []byte{0x25, 0, 1, 1, 0, 0}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}}}}
	bank, report, err := SourceVoiceBank(s)
	if err != nil || len(report.Converted) != 0 || report.Unsupported[2] == "" || bank.Instruments[2] != (model.Instrument{}) {
		t.Fatalf("hardware-specific source instrument became a generic square: %+v %v", report, err)
	}
}

func TestSourceArpeggioReadsHoldAndLoopAndRejectsBadPointers(t *testing.T) {
	for _, end := range []byte{0x8e, 0x8f} {
		b := sourceFixture()
		b[0x2604] = end
		s, err := DecodeSource(b, 0, 12)
		if err != nil {
			t.Fatal(err)
		}
		arp := s.Instruments[0].Arpeggio
		wanted := 0
		if end == 0x8e {
			wanted = 2
		}
		if !reflect.DeepEqual(arp.Values, []int{0, 7, 12}) || arp.Repeat != wanted || arp.StepFrames != 1 {
			t.Fatalf("wrong arpeggio termination: %+v", arp)
		}
	}
	b := sourceFixture()
	copy(b[0x824:], []byte{0xff, 0xff, 0xff, 0xff})
	if _, err := DecodeSource(b, 0, 12); err == nil {
		t.Fatal("invalid source arpeggio pointer accepted")
	}
}
