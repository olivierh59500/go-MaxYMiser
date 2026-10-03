package ymimport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

const (
	sndhInferredPlayer = "sndh-inferred-registers"
	sndhSampledPlayer  = "sndh-sampled-excerpt"
	sndhSampleRate     = 25033
	sndhSampleNote     = 48 // Native PCM mode 3 chooses 25033 Hz for this octave.
	sndhSampleLimit    = 32768
)

// ImportSNDH executes a bounded excerpt and creates an editable candidate. If
// register snapshots cannot supply playable notes within the native bank's
// capacity, it retains the rendered mix as explicitly labeled PCM samples.
func ImportSNDH(ctx context.Context, raw []byte, subtune, frames int) (Trace, *model.Project, Report, error) {
	return ImportSNDHSelection(ctx, raw, subtune, ReconstructionOptions{EndFrame: frames, FramesPerRow: 1})
}

// ImportSNDHSelection keeps the complete captured reference timeline while
// reconstructing the requested interval. EndFrame zero selects 400 frames.
// PCM excerpts retain one native row per source frame for their sample timing.
func ImportSNDHSelection(ctx context.Context, raw []byte, subtune int, options ReconstructionOptions) (Trace, *model.Project, Report, error) {
	if options.EndFrame == 0 {
		options.EndFrame = options.StartFrame + 400
	}
	if options.StartFrame < 0 || options.EndFrame <= options.StartFrame || options.EndFrame > 16320 || options.FramesPerRow < 0 || options.FramesPerRow > 16 {
		return Trace{}, nil, Report{}, fmt.Errorf("sndh: invalid editable selection")
	}
	trace, err := CaptureSNDH(ctx, raw, subtune, options.EndFrame)
	if err != nil {
		return trace, nil, Report{}, err
	}
	project, report, err := ReconstructSelection(trace, options)
	if cancelled := ctx.Err(); cancelled != nil {
		return trace, nil, report, cancelled
	}
	if err == nil && sndhPSGNotes(project) > 0 {
		report.SourcePlayer = sndhInferredPlayer
		report.SourceLabelRate = trace.Rate
		for i := range report.Warnings {
			report.Warnings[i] = strings.ReplaceAll(report.Warnings[i], "YM reference", "original SNDH reference")
		}
		report.Warnings = appendUnique(report.Warnings, "The generated PSG bank is inferred from register snapshots; original SNDH instrument names and programs are not recovered.")
		if trace.Effects {
			report.Warnings = appendUnique(report.Warnings, "Timer and DMA events remain in the original SNDH audio reference and may not be represented by the generated PSG score.")
		}
		if file, parseErr := sndh.Parse(raw); parseErr == nil {
			project.Year = file.Metadata.Year
		}
		return trace, project, report, nil
	}
	if err != nil && !sndhCapacityError(err) {
		return trace, nil, report, err
	}
	reason := "The selected register snapshots contain no playable PSG notes."
	if err != nil {
		reason = "The inferred score exceeds native tracker capacity: " + err.Error()
	}
	project, report, err = sndhSampledProject(ctx, raw, subtune, trace, options.StartFrame, options.EndFrame)
	if err != nil {
		return trace, nil, report, err
	}
	report.Warnings = appendUnique(report.Warnings, reason)
	return trace, project, report, nil
}

func sndhPSGNotes(project *model.Project) int {
	if project == nil {
		return 0
	}
	count := 0
	for position := 0; position < int(project.Song.Length); position++ {
		for channel := 0; channel < 3; channel++ {
			pattern := int(project.Song.Orders[position][channel])
			if pattern >= len(project.Song.Patterns) {
				continue
			}
			for _, cell := range project.Song.Patterns[pattern] {
				if cell.Note >= 2 && cell.Instrument > 0 {
					count++
				}
			}
		}
	}
	return count
}

func sndhCapacityError(err error) bool {
	message := err.Error()
	for _, text := range []string{"more than 32 distinct timbres", "255-position reconstruction limit", "more than 240 distinct patterns"} {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

func sndhSampledProject(ctx context.Context, raw []byte, subtune int, trace Trace, start, end int) (*model.Project, Report, error) {
	report := Report{SourcePlayer: sndhSampledPlayer, StartFrame: start, FramesPerRow: 1, SourceLabelRate: trace.Rate}
	// The skipped initial byte is a native mode-3 guard, leaving 32767
	// meaningful samples. A sample may span several 64-row patterns; using
	// the complete capacity keeps faster capture rates equally useful.
	rowsPerSample := (sndhSampleLimit - 1) * trace.Rate / sndhSampleRate
	if rowsPerSample < 1 {
		return nil, report, fmt.Errorf("sndh: capture rate cannot fit a native PCM row")
	}
	frames := min(end-start, model.MaxSamples*rowsPerSample)
	report.EndFrame, report.Frames = start+frames, frames
	if report.EndFrame < end {
		report.Warnings = append(report.Warnings, fmt.Sprintf("The sampled excerpt stops at frame %d: the native bank holds at most eight samples of 32768 bytes each. Select a later range to sample the rest.", report.EndFrame))
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		return nil, report, err
	}
	if subtune == 0 {
		subtune = file.Metadata.DefaultSubtune
	}
	renderer, err := sndh.NewRenderer(file, subtune, sndhSampleRate)
	if err != nil {
		return nil, report, err
	}
	defer renderer.Close()
	first := start * sndhSampleRate / trace.Rate
	last := (start + frames) * sndhSampleRate / trace.Rate
	buffer := make([]byte, 1024*4)
	for skipped := 0; skipped < first; {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		count := min(first-skipped, len(buffer)/4)
		if _, err := io.ReadFull(renderer, buffer[:count*4]); err != nil {
			return nil, report, fmt.Errorf("sndh: sampled excerpt seek: %w", err)
		}
		skipped += count
	}
	levels := make([]int16, last-first)
	peak := 0
	for at := 0; at < len(levels); {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		count := min(len(levels)-at, len(buffer)/4)
		if _, err := io.ReadFull(renderer, buffer[:count*4]); err != nil {
			return nil, report, fmt.Errorf("sndh: sampled excerpt audio: %w", err)
		}
		for i := 0; i < count; i++ {
			// The current SNDH backend supplies the same rendered mix on
			// both channels. Store the mix, without assigning voice names.
			level := int16(binary.LittleEndian.Uint16(buffer[i*4:]))
			levels[at+i] = level
			peak = max(peak, int(level), -int(level))
		}
		at += count
	}
	if peak == 0 {
		return nil, report, fmt.Errorf("sndh: selected excerpt contains no audio to sample")
	}
	pcm := make([]byte, len(levels))
	for i, level := range levels {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, report, err
			}
		}
		value := int(math.Round(float64(level) * 127 / float64(peak)))
		pcm[i] = byte(int8(max(-127, min(127, value))))
	}
	project := model.New()
	project.Title, project.Author, project.Year = trace.Name, trace.Author, file.Metadata.Year
	project.Bank = model.VoiceBank{Version: 1, SampleVersion: 1, SequenceCount: 1}
	project.Song.SetTickRate(trace.Rate)
	project.Song.SetSpeed(1)
	project.Song.State[49] = 3
	project.Song.Patterns = nil
	positions := (frames + model.Rows - 1) / model.Rows
	project.Song.Length, project.Song.Repeat = byte(positions), 0
	for i := range project.Song.Orders {
		project.Song.Orders[i] = [4]byte{model.EmptyPattern, model.EmptyPattern, model.EmptyPattern, model.EmptyPattern}
	}
	for position := 0; position < positions; position++ {
		project.Song.Orders[position][3] = byte(len(project.Song.Patterns))
		project.Song.Patterns = append(project.Song.Patterns, model.Pattern{})
	}
	samples := (frames + rowsPerSample - 1) / rowsPerSample
	for slot := 0; slot < samples; slot++ {
		frame := slot * rowsPerSample
		finish := min(frames, frame+rowsPerSample)
		from := (start+frame)*sndhSampleRate/trace.Rate - first
		to := (start+finish)*sndhSampleRate/trace.Rate - first
		data := make([]byte, 1, 1+to-from)
		data = append(data, pcm[from:to]...)
		if len(data) > sndhSampleLimit {
			return nil, report, fmt.Errorf("sndh: sampled block exceeds native sample capacity")
		}
		project.Bank.Samples[slot] = model.Sample{PCM: data, Trailer: []byte{0}}
		position, row := frame/model.Rows, frame%model.Rows
		pattern := project.Song.Orders[position][3]
		project.Song.Patterns[pattern][row] = model.Cell{Note: sndhSampleNote, Instrument: byte(slot + 1)}
	}
	// A partial final pattern must end after the selected frame, including
	// excerpts whose PCM sample changes do not coincide with pattern edges.
	var stop model.Pattern
	stop[(frames-1)%model.Rows].Effect1 = 'B'
	project.Song.Orders[positions-1][0] = byte(len(project.Song.Patterns))
	project.Song.Patterns = append(project.Song.Patterns, stop)
	report.Instruments, report.Patterns, report.Positions = samples, len(project.Song.Patterns), positions
	report.Warnings = append(report.Warnings,
		"This is a sampled audio excerpt, not recovered original instruments, voices, notes or tracker patterns.",
		"The rendered mix is normalized and quantized to mono 8-bit PCM at 25033 Hz. The original executable remains the audio reference.",
		fmt.Sprintf("The fourth track plays %d generated sample blocks on a one-frame row grid; each block has an initial native PCM guard byte.", samples))
	return project, report, nil
}
