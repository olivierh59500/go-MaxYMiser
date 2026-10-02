package model

// DACPCM is the original editor's inverse four-bit YM DAC-to-signed-PCM table.
var DACPCM = [16]byte{128, 129, 130, 132, 133, 135, 138, 142, 148, 157, 170, 189, 216, 7, 82, 127}

var DACLevels = func() (levels [256]byte) {
	for i := range levels {
		levels[i] = DACLevel(byte(i))
	}
	return
}()

// DACLevel uses the native signed sample byte as the lookup index. The groups
// below are the exact original PCM-to-YM thresholds, not a linear DAC scale.
func DACLevel(sample byte) byte {
	if sample < 8 {
		return 13
	}
	if sample < 83 {
		return 14
	}
	if sample < 128 {
		return 15
	}
	thresholds := [...]byte{128, 129, 130, 131, 133, 134, 136, 139, 143, 149, 158, 171, 190, 217}
	level := byte(0)
	for i, threshold := range thresholds {
		if sample < threshold {
			break
		}
		level = byte(i)
	}
	return level
}
