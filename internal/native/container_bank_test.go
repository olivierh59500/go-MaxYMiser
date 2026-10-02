package native

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestOptimizedSNDHCanOmitTrailingGuardsForAnEmptySampleBank(t *testing.T) {
	p := model.New()
	p.Bank.Instruments[0].SetName("Empty-bank sound")
	raw, err := EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	optimized := raw[:len(raw)-4]
	if _, err := DecodeVoiceBank(optimized); err == nil {
		t.Fatal("standalone MYV unexpectedly accepted out-of-file pointers")
	}
	bank, err := decodeContainerBank(optimized)
	if err != nil {
		t.Fatal(err)
	}
	if bank.Instruments != p.Bank.Instruments || bank.SequenceCount != p.Bank.SequenceCount {
		t.Fatal("empty-bank normalization changed the instruments or sequences")
	}
	for i := 0; i < p.Bank.SequenceCount; i++ {
		if bank.Sequences[i] != p.Bank.Sequences[i] {
			t.Fatal("empty-bank normalization changed a serialized sequence")
		}
	}
	for _, sample := range bank.Samples {
		if len(sample.PCM) != 0 {
			t.Fatal("normalization invented missing sample data")
		}
	}
	encoded, err := EncodeVoiceBank(bank)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVoiceBank(encoded)
	if err != nil || decoded.Instruments != bank.Instruments {
		t.Fatalf("normalized empty bank cannot be saved as native MYV: %v", err)
	}
	if !bytes.Equal(optimized, raw[:len(raw)-4]) {
		t.Fatal("normalization modified its source bytes")
	}
}

func TestContainerBankRecoveryNeverFillsMissingNonemptySamples(t *testing.T) {
	p := model.New()
	raw, _ := EncodeVoiceBank(p.Bank)
	raw = raw[:len(raw)-4]
	binary.BigEndian.PutUint16(raw[2088+7*4:], 1)
	if _, err := decodeContainerBank(raw); err == nil {
		t.Fatal("nonempty missing sample was silently filled")
	}
	raw, _ = EncodeVoiceBank(p.Bank)
	binary.BigEndian.PutUint32(raw[4:], 1)
	if _, err := decodeContainerBank(raw); err == nil {
		t.Fatal("a pointer into the header was repaired as an empty sample")
	}
}
