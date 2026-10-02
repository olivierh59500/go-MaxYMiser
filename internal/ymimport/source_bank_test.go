package ymimport

import (
	"encoding/binary"
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

func TestSourceNoiseAttackUsesNativeMixerOrderAndRetainsTheArpeggio(t *testing.T) {
	s := SourceScore{Instruments: []SourceInstrument{{ID: 3, Settings: []byte{4, 72, 1, 0, 1, 1}, VolumeSequence: []byte{14, 14, 13}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0, 3, 7, 12}, Repeat: 0}, NoiseProgram: []SourceNoiseStep{{1, 47}, {1, 47}, {3, 47}}}}}
	bank, report, err := SourceVoiceBank(s)
	if err != nil || len(report.Converted) != 1 || len(report.Unsupported) != 0 {
		t.Fatalf("noise attack did not convert: %+v %v", report, err)
	}
	p := model.New()
	p.Bank = bank
	e := replay.New(p)
	e.Trigger(0, 60, 4)
	for frame := 0; frame < 8; frame++ {
		e.Tick()
		wantMixer := byte(62)
		if frame == 0 {
			wantMixer = 55
		}
		if e.Registers[7]&63 != wantMixer {
			t.Fatalf("native attack call %d: mixer=%02x, want %02x", frame, e.Registers[7], wantMixer)
		}
		if frame == 0 && e.Registers[6] != 15 {
			t.Fatalf("native noise 47 did not become its five-bit period: %d", e.Registers[6])
		}
		if frame > 0 {
			want := []uint16{478, 402, 319, 239}[frame%4]
			got := uint16(e.Registers[0]) | uint16(e.Registers[1])<<8
			if got != want {
				t.Fatalf("noise attack lost chord step %d: %d, want %d", frame, got, want)
			}
		}
	}
}

func TestSourceNoiseProgramDecodingUsesValidatedPointersAndRejectsInvalidModes(t *testing.T) {
	b := sourceFixture()
	b[0x2030] = 4
	binary.BigEndian.PutUint32(b[0x16d4:], 0x10000+0x2620)
	copy(b[0x2620:], []byte{1, 47, 1, 47, 3, 47, 255})
	s, err := DecodeSource(b, 0, 24)
	if err != nil || len(s.Instruments[3].NoiseProgram) != 3 {
		t.Fatalf("noise program was not retained: %v", err)
	}
	if s.Instruments[3].NoiseProgram[1] != (SourceNoiseStep{1, 47}) {
		t.Fatal("native noise operands changed")
	}
	b[0x2622] = 9
	if _, err := DecodeSource(b, 0, 24); err == nil {
		t.Fatal("unknown native noise operation was silently translated")
	}
	b[0x2622] = 1
	binary.BigEndian.PutUint32(b[0x16d4:], 0xffffffff)
	if _, err := DecodeSource(b, 0, 24); err == nil {
		t.Fatal("invalid native noise pointer was accepted")
	}
}

func TestSourceBankRejectsUnsupportedHardwareWithoutInventingSounds(t *testing.T) {
	s := SourceScore{Instruments: []SourceInstrument{{ID: 2, Settings: []byte{0x25, 0, 1, 1, 0, 0}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}}}}
	bank, report, err := SourceVoiceBank(s)
	if err != nil || len(report.Converted) != 0 || report.Unsupported[2] == "" || bank.Instruments[2] != (model.Instrument{}) {
		t.Fatalf("hardware-specific source instrument became a generic square: %+v %v", report, err)
	}
}

func TestAutomaticPitchDrumRetainsItsAudibleSlideAndSemitoneDescent(t *testing.T) {
	sound := SourceInstrument{ID: 0, Settings: []byte{0x31, 0, 0, 0, 0, 1}, VolumeSequence: []byte{15, 15, 0}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}, NoiseProgram: []SourceNoiseStep{{1, 47}, {2, 47}, {2, 47}, {2, 47}, {3, 47}}}
	bank, report, err := SourceVoiceBank(SourceScore{Instruments: []SourceInstrument{sound}})
	if err != nil || len(report.Converted) != 1 || len(report.Unsupported) != 0 {
		t.Fatalf("verified automatic drum remained unsupported: %+v %v", report, err)
	}
	p := model.New()
	p.Bank = bank
	e := replay.New(p)
	e.Trigger(0, 60, 1)
	for frame := 0; frame < 4; frame++ {
		e.Tick()
		if frame < 3 {
			// Original native calls produce 578, 680, 784 from periods
			// 506, 536, 568 plus accumulator values 72, 144, 216.
			want := int(replay.TonePeriod(59-frame)) + 72*(frame+1)
			period := int(e.Registers[0]) | int(e.Registers[1])<<8
			if period != want || e.Registers[8] != 15 || e.Registers[7]&63 != 54 {
				t.Fatalf("automatic drum call %d: period=%d volume=%d mixer=%02x", frame, period, e.Registers[8], e.Registers[7])
			}
		} else if e.Registers[8] != 0 {
			t.Fatal("finite automatic program did not end in silence")
		}
	}
	raw, err := native.EncodeVoiceBank(bank)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := native.DecodeVoiceBank(raw); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticPitchProgramsRejectAnUnboundedAudibleTail(t *testing.T) {
	sound := SourceInstrument{ID: 0, Settings: []byte{0x21, 0, 0, 0, 0, 1}, VolumeSequence: []byte{15}, Arpeggio: SourceSequence{StepFrames: 1, Values: []int{0}, Repeat: 0}}
	bank, report, err := SourceVoiceBank(SourceScore{Instruments: []SourceInstrument{sound}})
	if err != nil || report.Unsupported[0] == "" || bank.Instruments[0] != (model.Instrument{}) {
		t.Fatal("an unbounded automatic program acquired a guessed finite loop")
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
