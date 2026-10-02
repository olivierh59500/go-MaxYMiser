// Package edit implements reversible tracker editing operations.
package edit

import (
	"fmt"
	"math"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type Shape string

const (
	Ramp     Shape = "ramp"
	Triangle Shape = "triangle"
	Sine     Shape = "sine"
	Square   Shape = "square"
)

// GenerateSequence creates native words. Signed mode represents negative
// detune/arpeggio values using the native two's-complement word encoding.
func GenerateSequence(shape Shape, length, low, high int, cycles float64, signed bool) (model.Sequence, error) {
	var out model.Sequence
	if length < 1 || length > 63 || cycles <= 0 || cycles > 32 || math.IsNaN(cycles) || math.IsInf(cycles, 0) {
		return out, fmt.Errorf("edit: length must be 1–63 and cycles must be greater than zero and at most 32")
	}
	minValue, maxValue := 0, 65535
	if signed {
		minValue, maxValue = -32768, 32767
	}
	if low < minValue || low > maxValue || high < minValue || high > maxValue {
		return out, fmt.Errorf("edit: sequence endpoints exceed the selected word range")
	}
	if shape != Ramp && shape != Triangle && shape != Sine && shape != Square {
		return out, fmt.Errorf("edit: unknown sequence shape %q", shape)
	}
	out.Length = byte(length)
	for i := 0; i < length; i++ {
		phase := float64(i) * cycles / float64(length)
		phase -= math.Floor(phase)
		fraction := 0.0
		switch shape {
		case Ramp:
			fraction = float64(i) / float64(max(1, length-1))
		case Triangle:
			fraction = 1 - math.Abs(2*phase-1)
		case Sine:
			fraction = (1 - math.Cos(2*math.Pi*phase)) / 2
		case Square:
			if phase >= 0.5 {
				fraction = 1
			}
		}
		value := int(math.Round(float64(low) + float64(high-low)*fraction))
		out.Values[i] = uint16(value)
	}
	if shape == Ramp {
		out.Repeat = byte(length - 1)
	}
	return out, nil
}

// MorphSequences preserves the endpoints and interpolates definitions between
// them. The sequences must have equal lengths and repeat points so timing and
// looping remain meaningful throughout the morph.
func MorphSequences(bank *model.VoiceBank, first, last int, signed bool) error {
	if first < 0 || last >= model.MaxSequences || last <= first+1 {
		return fmt.Errorf("edit: choose two sequence IDs with at least one slot between them")
	}
	a, b := bank.Sequences[first], bank.Sequences[last]
	if a.Length == 0 || a.Length > 63 || a.Length != b.Length || a.Repeat != b.Repeat {
		return fmt.Errorf("edit: morph endpoints require the same valid length and repeat")
	}
	for id := first + 1; id < last; id++ {
		sequence := model.Sequence{Length: a.Length, Repeat: a.Repeat}
		fraction := float64(id-first) / float64(last-first)
		for i := 0; i < int(a.Length); i++ {
			x, y := int(a.Values[i]), int(b.Values[i])
			if signed {
				x, y = int(int16(a.Values[i])), int(int16(b.Values[i]))
			}
			sequence.Values[i] = uint16(int(math.Round(float64(x) + float64(y-x)*fraction)))
		}
		bank.Sequences[id] = sequence
	}
	bank.SequenceCount = max(bank.SequenceCount, last+1)
	return nil
}

// ModifySequence adds or scales a selected word range without changing loop
// metadata. It saturates in the selected signed/unsigned native word domain.
func ModifySequence(sequence *model.Sequence, first, last, offset int, scale float64, signed bool) error {
	if first < 0 || last < first || last >= int(sequence.Length) || last >= 63 || math.IsNaN(scale) || math.IsInf(scale, 0) || scale < 0 || scale > 64 {
		return fmt.Errorf("edit: invalid sequence modification range or scale")
	}
	minimum, maximum := 0, 65535
	if signed {
		minimum, maximum = -32768, 32767
	}
	for i := first; i <= last; i++ {
		value := int(sequence.Values[i])
		if signed {
			value = int(int16(sequence.Values[i]))
		}
		value = int(math.Round(float64(value)*scale)) + offset
		sequence.Values[i] = uint16(max(minimum, min(maximum, value)))
	}
	return nil
}
