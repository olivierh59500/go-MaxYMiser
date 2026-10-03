package export

import (
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

// SNDH renders the original executable, including its timer and DMA audio.
// A YM snapshot trace is deliberately not substituted for the source sound.
func SNDH(raw []byte, subtune int, path string, duration time.Duration) error {
	file, err := sndh.Parse(raw)
	if err != nil {
		return err
	}
	if subtune == 0 {
		subtune = file.Metadata.DefaultSubtune
	}
	r, err := sndh.NewRenderer(file, subtune, 48000)
	if err != nil {
		return err
	}
	defer r.Close()
	return writeAudio(r, path, duration)
}
