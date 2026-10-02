package edit

import (
	"fmt"
	"math"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func replaceSample(sample *model.Sample, pcm []byte) {
	sample.PCM = pcm
	sample.Trailer = []byte{0}
}

// AmplifySample clips to the native signed eight-bit range after applying gain.
func AmplifySample(sample *model.Sample, decibels float64) error {
	if math.IsNaN(decibels) || math.IsInf(decibels, 0) || math.Abs(decibels) > 48 {
		return fmt.Errorf("edit: sample gain must be between -48 and 48 dB")
	}
	gain := math.Pow(10, decibels/20)
	pcm := make([]byte, len(sample.PCM))
	for i, value := range sample.PCM {
		pcm[i] = byte(int8(max(-128, min(127, int(math.Round(float64(int8(value))*gain))))))
	}
	replaceSample(sample, pcm)
	return nil
}

// TuneSample changes playback pitch by resampling the data. Linear
// interpolation prevents steps when stretching a sample down in pitch.
func TuneSample(sample *model.Sample, semitones float64) error {
	if math.IsNaN(semitones) || math.IsInf(semitones, 0) || math.Abs(semitones) > 24 {
		return fmt.Errorf("edit: sample tuning must be between -24 and 24 semitones")
	}
	if len(sample.PCM) == 0 {
		return fmt.Errorf("edit: import a sample before tuning it")
	}
	ratio := math.Pow(2, semitones/12)
	length := max(1, int(math.Round(float64(len(sample.PCM))/ratio)))
	if length > 32768 {
		return fmt.Errorf("edit: tuned sample exceeds 32 KiB")
	}
	pcm := make([]byte, length)
	for i := range pcm {
		position := math.Min(float64(len(sample.PCM)-1), float64(i)*ratio)
		at := int(position)
		a, b := float64(int8(sample.PCM[at])), float64(int8(sample.PCM[min(at+1, len(sample.PCM)-1)]))
		pcm[i] = byte(int8(math.Round(a + (b-a)*(position-float64(at)))))
	}
	replaceSample(sample, pcm)
	return nil
}

func TrimSample(sample *model.Sample, start, length int) error {
	if start < 0 || length < 0 || start > len(sample.PCM) || length > len(sample.PCM)-start {
		return fmt.Errorf("edit: sample trim range is outside the sample")
	}
	replaceSample(sample, append([]byte(nil), sample.PCM[start:start+length]...))
	return nil
}

func ToggleSampleSign(sample *model.Sample) {
	pcm := append([]byte(nil), sample.PCM...)
	for i := range pcm {
		pcm[i] ^= 128
	}
	replaceSample(sample, pcm)
}

// YMiseSample round-trips through the original YM four-bit DAC mapping so PCM
// playback reproduces the quantized DigiDrum character of the native editor.
func YMiseSample(sample *model.Sample) {
	pcm := make([]byte, len(sample.PCM))
	for i, value := range sample.PCM {
		pcm[i] = model.DACPCM[model.DACLevels[value]]
	}
	replaceSample(sample, pcm)
}
