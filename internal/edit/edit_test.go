package edit

import (
	"bytes"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestGeneratedEnvelopeAndSignedVibratoHaveUsefulEndpoints(t *testing.T) {
	envelope, err := GenerateSequence(Ramp, 6, 15, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Values[0] != 15 || envelope.Values[5] != 0 || envelope.Repeat != 5 {
		t.Fatalf("ramp cannot decay and hold: %+v", envelope)
	}
	vibrato, err := GenerateSequence(Sine, 8, -4, 4, 1, true)
	if err != nil || int16(vibrato.Values[0]) != -4 || int16(vibrato.Values[4]) != 4 || vibrato.Repeat != 0 {
		t.Fatalf("signed vibrato did not span and loop: %+v err=%v", vibrato, err)
	}
}

func TestSequenceModificationKeepsLoopAndUnselectedWords(t *testing.T) {
	sequence := model.Sequence{Length: 4, Repeat: 1, Values: [63]uint16{65532, 2, 8, 9}}
	if err := ModifySequence(&sequence, 0, 2, 1, 2, true); err != nil {
		t.Fatal(err)
	}
	if int16(sequence.Values[0]) != -7 || sequence.Values[1] != 5 || sequence.Values[2] != 17 || sequence.Values[3] != 9 || sequence.Repeat != 1 {
		t.Fatal("range modification lost signed values, boundaries or loop")
	}
	before := sequence
	if err := ModifySequence(&sequence, 0, 4, 0, 1, true); err == nil || sequence != before {
		t.Fatal("invalid range partly modified a sequence")
	}
}

func TestMorphPreservesEndpointsAndInterpolatesSignedWords(t *testing.T) {
	var bank model.VoiceBank
	bank.Sequences[1] = model.Sequence{Length: 2, Repeat: 1, Values: [63]uint16{65528, 4}}
	bank.Sequences[3] = model.Sequence{Length: 2, Repeat: 1, Values: [63]uint16{4, 8}}
	a, b := bank.Sequences[1], bank.Sequences[3]
	if err := MorphSequences(&bank, 1, 3, true); err != nil {
		t.Fatal(err)
	}
	if int16(bank.Sequences[2].Values[0]) != -2 || bank.Sequences[2].Values[1] != 6 || bank.Sequences[1] != a || bank.Sequences[3] != b || bank.SequenceCount != 4 {
		t.Fatal("morph changed endpoints or failed interpolation")
	}
	bank.Sequences[3].Length = 3
	before := bank
	if err := MorphSequences(&bank, 1, 3, true); err == nil || bank.Sequences != before.Sequences {
		t.Fatal("invalid morph modified the bank")
	}
}

func TestSampleGainTuningTrimAndSignConversion(t *testing.T) {
	sample := model.Sample{PCM: []byte{0, 64, 127, 128, 192, 0}}
	if err := AmplifySample(&sample, 6.020599913); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sample.PCM, []byte{0, 127, 127, 128, 128, 0}) {
		t.Fatalf("signed gain/clipping wrong: %v", sample.PCM)
	}
	sample.PCM = []byte{0, 64, 0, 192}
	if err := TuneSample(&sample, -12); err != nil {
		t.Fatal(err)
	}
	if len(sample.PCM) != 8 || sample.PCM[1] != 32 || sample.PCM[5] != 224 {
		t.Fatalf("octave-down interpolation wrong: %v", sample.PCM)
	}
	if err := TrimSample(&sample, 1, 3); err != nil || !bytes.Equal(sample.PCM, []byte{32, 64, 32}) {
		t.Fatalf("trim wrong: %v %v", sample.PCM, err)
	}
	copy := append([]byte(nil), sample.PCM...)
	ToggleSampleSign(&sample)
	ToggleSampleSign(&sample)
	if !bytes.Equal(sample.PCM, copy) {
		t.Fatal("sign conversion was not reversible")
	}
	sample.PCM = make([]byte, 32768)
	if err := TuneSample(&sample, -12); err == nil || len(sample.PCM) != 32768 {
		t.Fatal("oversized tuning was not rejected without modifying the sample")
	}
}

func TestSampleLengthRetainsPrefixAndPadsSignedSilence(t *testing.T) {
	original := []byte{0, 127, 128, 255}
	sample := model.Sample{PCM: original, Parameters: [4]byte{3, 4, 5, 6}, Trailer: []byte{128}}
	if err := ResizeSample(&sample, 7); err != nil || !bytes.Equal(sample.PCM, []byte{0, 127, 128, 255, 0, 0, 0}) || sample.Parameters != ([4]byte{3, 4, 5, 6}) {
		t.Fatalf("sample extension lost PCM or did not append signed silence: %v", err)
	}
	if !bytes.Equal(original, []byte{0, 127, 128, 255}) || !bytes.Equal(sample.Trailer, []byte{0}) {
		t.Fatal("resizing altered its original storage or kept a stale native tail")
	}
	if err := ResizeSample(&sample, 2); err != nil || !bytes.Equal(sample.PCM, []byte{0, 127}) {
		t.Fatalf("sample shortening changed its first bytes: %v", err)
	}
	if err := ResizeSample(&sample, 32768); err != nil || len(sample.PCM) != 32768 {
		t.Fatalf("maximum native sample length was rejected: %v", err)
	}
	before := append([]byte(nil), sample.PCM...)
	for _, length := range []int{-1, 32769} {
		if err := ResizeSample(&sample, length); err == nil || !bytes.Equal(sample.PCM, before) {
			t.Fatal("invalid sample length changed its previous PCM")
		}
	}
	if err := ResizeSample(&sample, 0); err != nil || len(sample.PCM) != 0 {
		t.Fatalf("zero-length sample could not be cleared: %v", err)
	}
}
