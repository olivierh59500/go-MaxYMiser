package replay

import "math"

// SetMicrowire applies a native LMC1992 tone or volume command independently of
// editor volume and pan. The original editor enables these controls with DMA.
// The circuit uses 2 dB steps; values above its maximum level remain at unity.
func (e *Engine) SetMicrowire(command uint16) {
	if e.Project.Song.State[49] == 0 {
		return
	}
	value := int(command & 63)
	switch command & 0x7c0 {
	case 0x440:
		e.Bass = min(value&15, 12)
	case 0x480:
		e.Treble = min(value&15, 12)
	case 0x4c0:
		e.MicrowireGain = math.Pow(10, float64(min(value, 40)-40)*2/20)
	case 0x500:
		e.MicrowireRight = math.Pow(10, float64(min(value&31, 20)-20)*2/20)
	case 0x540:
		e.MicrowireLeft = math.Pow(10, float64(min(value&31, 20)-20)*2/20)
	}
}
