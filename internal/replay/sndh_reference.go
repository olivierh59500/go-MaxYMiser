package replay

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

const sndhSeekTimeout = 20 * time.Second

type sndhRenderer interface {
	io.Reader
	PositionSamples() uint64
	Registers() [14]byte
	HasEffects() bool
	Close() error
}

type sndhRendererFactory func(*sndh.File, int, int) (sndhRenderer, error)

type sndhReference struct {
	file                *sndh.File
	renderer            sndhRenderer
	subtune, sampleRate int
	playing             bool
	prefix              []byte
	newRenderer         sndhRendererFactory
	// These fields, like renderer and playing, are protected by Synth.mu.
	seekCancel     context.CancelFunc
	seekGeneration uint64
	seeking        bool
}

func makeSNDHRenderer(file *sndh.File, subtune, rate int) (sndhRenderer, error) {
	return sndh.NewRenderer(file, subtune, rate)
}

func newSNDHReference(raw []byte, subtune, rate int) (*sndhReference, error) {
	file, err := sndh.Parse(raw)
	if err != nil {
		return nil, err
	}
	if subtune == 0 {
		subtune = file.Metadata.DefaultSubtune
	}
	r, err := sndh.NewRenderer(file, subtune, rate)
	if err != nil {
		return nil, err
	}
	// Validate the first play call before replacing an existing composition.
	prime := make([]byte, 128*4)
	if _, err = io.ReadFull(r, prime); err != nil {
		r.Close()
		return nil, err
	}
	return &sndhReference{file: file, renderer: r, subtune: subtune, sampleRate: rate, playing: true, prefix: prime, newRenderer: makeSNDHRenderer}, nil
}

// LoadSNDH executes the selected original player while retaining the editable
// composition. Its synthesizer output includes timer and STE DMA activity.
func (s *Synth) LoadSNDH(raw []byte, subtune int) error {
	ref, err := newSNDHReference(raw, subtune, s.Rate)
	if err != nil {
		return err
	}
	s.replaceReference(ref)
	return nil
}

func (r *sndhReference) Read(p []byte) (int, error) {
	if !r.playing {
		clear(p)
		return len(p), nil
	}
	n := copy(p, r.prefix)
	r.prefix = r.prefix[n:]
	if n == len(p) {
		return n, nil
	}
	count, err := r.renderer.Read(p[n:])
	return n + count, err
}
func (r *sndhReference) Info() Reference {
	m := r.file.Metadata
	position := r.renderer.PositionSamples() - uint64(len(r.prefix)/4)
	out := Reference{Name: m.Title, Author: m.Author, Format: "SNDH", Position: uint32(position * 1000 / uint64(r.sampleRate)), Registers: r.renderer.Registers(), Subtune: r.subtune, Subtunes: m.Subtunes, Effects: r.renderer.HasEffects(), Seeking: r.seeking}
	if r.subtune <= len(m.Frames) && m.Rate > 0 {
		out.Duration = uint32(uint64(m.Frames[r.subtune-1]) * 1000 / uint64(m.Rate))
	}
	return out
}
func (r *sndhReference) SetPlaying(on bool) { r.playing = on }

// Seek is the bounded synchronous source operation. Synth routes SNDH seeks
// through startSNDHSeekLocked so this work never runs under the audio lock.
func (r *sndhReference) Seek(ms uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), sndhSeekTimeout)
	defer cancel()
	next, err := prepareSNDHSeek(ctx, r.file, r.subtune, r.sampleRate, ms, r.newRenderer)
	if err != nil {
		return err
	}
	r.renderer.Close()
	r.renderer = next
	r.prefix = nil
	return nil
}

func prepareSNDHSeek(ctx context.Context, file *sndh.File, subtune, rate int, ms uint32, create sndhRendererFactory) (sndhRenderer, error) {
	if ms > 3600000 {
		return nil, fmt.Errorf("sndh: seek exceeds one hour")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	next, err := create(file, subtune, rate)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			next.Close()
		}
	}()
	buffer := make([]byte, 1024*4)
	remaining := uint64(ms) * uint64(rate) / 1000
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := min(remaining, 1024)
		if _, err = io.ReadFull(next, buffer[:int(n)*4]); err != nil {
			return nil, err
		}
		remaining -= n
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	failed = false
	return next, nil
}

func (r *sndhReference) cancelSeek() {
	if r.seekCancel != nil {
		r.seekCancel()
		r.seekCancel = nil
	}
	r.seekGeneration++
	r.seeking = false
}

func (s *Synth) startSNDHSeekLocked(ref *sndhReference, ms uint32) {
	ref.cancelSeek()
	s.referenceError = ""
	if ms > 3600000 {
		s.referenceError = "sndh: seek exceeds one hour"
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sndhSeekTimeout)
	ref.seekCancel, ref.seeking = cancel, true
	generation := ref.seekGeneration
	file, subtune, rate, create := ref.file, ref.subtune, ref.sampleRate, ref.newRenderer
	go func() {
		defer cancel()
		next, err := prepareSNDHSeek(ctx, file, subtune, rate, ms, create)
		s.mu.Lock()
		if s.reference != ref || ref.seekGeneration != generation {
			s.mu.Unlock()
			if next != nil {
				next.Close()
			}
			return
		}
		if err == nil {
			err = ctx.Err()
		}
		ref.seekCancel, ref.seeking = nil, false
		if err != nil {
			s.referenceError = fmt.Sprintf("sndh: seek: %v", err)
			s.mu.Unlock()
			if next != nil {
				next.Close()
			}
			return
		}
		old := ref.renderer
		ref.renderer, ref.prefix, ref.playing = next, nil, s.referencePlaying
		s.mu.Unlock()
		// No audio callback can still own old: it must finish under the same
		// mutex before the swap. Exit code is bounded but runs outside it.
		old.Close()
	}()
}

func (r *sndhReference) Close() { r.cancelSeek(); r.renderer.Close() }
