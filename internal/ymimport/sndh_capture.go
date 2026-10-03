package ymimport

import (
	"context"
	"fmt"
	"io"

	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

// CaptureSNDH retains observable register states without assigning native
// instrument or pattern identities. The audio reference executes separately.
// Snapshot rates stay within the editable tracker range; fast player calls and
// subtick hardware effects remain in the original reference, not this grid.
func CaptureSNDH(ctx context.Context, raw []byte, subtune, frames int) (Trace, error) {
	var trace Trace
	if frames < 1 || frames > 16320 {
		return trace, fmt.Errorf("sndh: capture must contain 1–16320 editable frames")
	}
	if err := ctx.Err(); err != nil {
		return trace, err
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		return trace, err
	}
	if subtune == 0 {
		subtune = file.Metadata.DefaultSubtune
	}
	r, err := sndh.NewRenderer(file, subtune, 48000)
	if err != nil {
		return trace, err
	}
	defer r.Close()
	rate := file.Metadata.Rate
	if rate < 25 || rate > 200 {
		rate = 50
	}
	trace = Trace{Name: file.Metadata.Title, Author: file.Metadata.Author, Rate: rate, Clock: 2000000, Frames: make([][14]byte, frames)}
	buffer := make([]byte, (48000/rate+1)*4)
	previous := 0
	for i := range trace.Frames {
		if err := ctx.Err(); err != nil {
			return Trace{}, err
		}
		until := (i + 1) * 48000 / rate
		if _, err := io.ReadFull(r, buffer[:(until-previous)*4]); err != nil {
			return Trace{}, fmt.Errorf("sndh: capture frame %d: %w", i, err)
		}
		previous = until
		trace.Frames[i] = r.Registers()
	}
	trace.Effects = r.HasEffects()
	return trace, nil
}
