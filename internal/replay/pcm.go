package replay

// Native note rates start at note 24. Native-rate mode chooses an STe hardware
// frequency by octave; the resampling modes use the chromatic rate table.
func pcmRate(note byte, mode byte) int {
	if note < 24 {
		return 0
	}
	for note >= 68 {
		note -= 12
	}
	if mode == 3 {
		return int(samplePeriods[int(note)-24])
	}
	return int(dmaNoteRates[int(note)-24])
}
