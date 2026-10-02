package ui

import (
	"fmt"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type instrumentSequence struct {
	name, values string
	id, offset   int
}

// instrumentSequences describes the definitions that give otherwise similar
// scalar settings different sounds. Values remain native hexadecimal words.
func instrumentSequences(bank *model.VoiceBank, index int) []instrumentSequence {
	names := []string{"Volume", "Arpeggio", "Vibrato", "Mixer", "Noise", "Fixed", "Timer", "PWM"}
	inst := bank.Instruments[index]
	result := make([]instrumentSequence, len(names))
	for i, name := range names {
		id := int(inst[48+i])
		sequence := bank.Sequences[id]
		length := max(1, min(63, int(sequence.Length)))
		var words []string
		for _, value := range sequence.Values[:min(4, length)] {
			words = append(words, fmt.Sprintf("%04X", value))
		}
		values := strings.Join(words, " ")
		if length > 4 {
			values += " …"
		}
		result[i] = instrumentSequence{name: name, values: values, id: id, offset: 48 + i}
	}
	return result
}
