package native

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// Configuration is the native 29-byte MYM.CNF save layout from the 1.67 editor.
// Hardware/display fields are retained even when the Go UI has no equivalent.
type Configuration [29]byte

func DecodeConfiguration(data []byte) (Configuration, error) {
	var config Configuration
	if len(data) != len(config) {
		return config, fmt.Errorf("native: MYM.CNF must contain 29 bytes")
	}
	copy(config[:], data)
	return config, nil
}

func (c Configuration) Apply(song *model.Song) {
	song.State[11] = c[0]
	copy(song.State[18:25], c[1:8])
	copy(song.State[40:44], c[17:21])
	song.State[51] = c[21]
	copy(song.State[32:36], c[22:26])
	song.State[50] = c[26]
	song.State[31] = c[27]
	song.State[57] = c[28]
}

func CaptureConfiguration(song model.Song, previous Configuration) Configuration {
	config := previous
	config[0] = song.State[11]
	copy(config[1:8], song.State[18:25])
	copy(config[17:21], song.State[40:44])
	config[21] = song.State[51]
	copy(config[22:26], song.State[32:36])
	config[26], config[27], config[28] = song.State[50], song.State[31], song.State[57]
	return config
}
