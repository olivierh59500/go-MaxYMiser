package replay

import (
	"math"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func rmsEqualized(frequency float64, bass, treble int) float64 {
	var eq equalizer
	energy := 0.0
	for i := 0; i < 48000; i++ {
		input := int(1000 * math.Sin(2*math.Pi*frequency*float64(i)/48000))
		output := eq.process(input, 48000, bass, treble)
		if i >= 24000 {
			energy += float64(output) * float64(output)
		}
	}
	return math.Sqrt(energy / 24000)
}

func TestBassAndTrebleCommandsHaveAnAudibleFrequencyDependentEffect(t *testing.T) {
	flatLow := rmsEqualized(30, 6, 6)
	boostLow := rmsEqualized(30, 12, 6)
	cutLow := rmsEqualized(30, 0, 6)
	if boostLow/flatLow < 3 || cutLow/flatLow > 0.4 {
		t.Fatalf("bass control did not affect audio: flat=%f boost=%f cut=%f", flatLow, boostLow, cutLow)
	}
	flatHigh := rmsEqualized(16000, 6, 6)
	boostHigh := rmsEqualized(16000, 6, 12)
	if boostHigh/flatHigh < 3 {
		t.Fatal("treble control did not affect audio")
	}
}

func TestMicrowireCommandsUseDecibelStepsAndDoNotReplaceEditorVolume(t *testing.T) {
	p := model.New()
	e := New(p)
	e.effect(&e.Voices[0], 'U', 0x9f, false)
	if math.Abs(e.MicrowireGain-math.Pow(10, -2.0/20)) > 1e-9 || e.MasterVolume != 127 {
		t.Fatal("U master volume used linear scaling or changed editor volume")
	}
	e.effect(&e.Voices[0], 'U', 0xc0, false)
	if e.MicrowireLeft != 1 || math.Abs(e.MicrowireRight-math.Pow(10, -32.0/20)) > 1e-9 {
		t.Fatal("Microwire pan did not use 2 dB attenuation steps")
	}
	p.Song.State[49] = 0
	e.Bass = 6
	e.effect(&e.Voices[0], 'U', 12, false)
	if e.Bass != 6 {
		t.Fatal("Microwire acted with native DMA disabled")
	}
}
