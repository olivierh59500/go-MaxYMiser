package replay

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

// Controlled readers make seek ordering deterministic without executing hours
// of music or assuming a particular scheduler or computer speed.
type seekTestRenderer struct {
	position    atomic.Uint64
	readGate    <-chan struct{}
	readStarted chan struct{}
	startOnce   sync.Once
	closed      chan struct{}
	closeOnce   sync.Once
}

func newSeekTestRenderer() *seekTestRenderer { return &seekTestRenderer{closed: make(chan struct{})} }
func (r *seekTestRenderer) Read(p []byte) (int, error) {
	if r.readStarted != nil {
		r.startOnce.Do(func() { close(r.readStarted) })
	}
	if r.readGate != nil {
		<-r.readGate
	}
	for at := 0; at+4 <= len(p); at += 4 {
		binary.LittleEndian.PutUint16(p[at:], 1234)
		binary.LittleEndian.PutUint16(p[at+2:], 1234)
	}
	r.position.Add(uint64(len(p) / 4))
	return len(p), nil
}
func (r *seekTestRenderer) PositionSamples() uint64 { return r.position.Load() }
func (r *seekTestRenderer) Registers() [14]byte     { return [14]byte{8, 0, 0, 0, 0, 0, 0, 62, 15} }
func (r *seekTestRenderer) HasEffects() bool        { return false }
func (r *seekTestRenderer) Close() error            { r.closeOnce.Do(func() { close(r.closed) }); return nil }

func seekTestSynth(factory sndhRendererFactory) (*Synth, *sndhReference, *seekTestRenderer) {
	s := NewSynth(New(model.Demo()), 8000)
	old := newSeekTestRenderer()
	r := &sndhReference{file: &sndh.File{Metadata: sndh.Metadata{Title: "Async seek", Rate: 50, Subtunes: 1, DefaultSubtune: 1}}, renderer: old, subtune: 1, sampleRate: 8000, playing: true, newRenderer: factory}
	s.replaceReference(r)
	return s, r, old
}
func waitSeekEvent(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("seek worker did not reach its controlled checkpoint")
	}
}
func waitReference(t *testing.T, s *Synth, match func(Reference) bool) Reference {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		info, ok := s.Reference()
		if ok && match(info) {
			return info
		}
		select {
		case <-deadline.C:
			t.Fatalf("reference did not reach expected state: %+v", info)
		case <-tick.C:
		}
	}
}

func TestSNDHSeekDoesNotHoldAudioMutex(t *testing.T) {
	gate := make(chan struct{})
	defer close(gate)
	next := newSeekTestRenderer()
	next.readGate = gate
	next.readStarted = make(chan struct{})
	s, _, old := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) { return next, nil })
	defer s.CloseYM()
	done := make(chan struct{})
	go func() { s.SeekYM(1000); close(done) }()
	waitSeekEvent(t, done)
	waitSeekEvent(t, next.readStarted)
	info, ok := s.Reference()
	if !ok || !info.Seeking || !info.Playing {
		t.Fatalf("pending seek %+v", info)
	}
	readDone := make(chan struct{})
	go func() { s.Read(make([]byte, 32)); close(readDone) }()
	waitSeekEvent(t, readDone)
	if old.PositionSamples() != 8 {
		t.Fatal("old audio did not continue during seek")
	}
}

func TestSNDHLatestSeekWinsAndCancelsOlderResult(t *testing.T) {
	gate := make(chan struct{})
	first := newSeekTestRenderer()
	first.readGate = gate
	first.readStarted = make(chan struct{})
	second := newSeekTestRenderer()
	var count atomic.Int32
	s, ref, _ := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) {
		if count.Add(1) == 1 {
			return first, nil
		}
		return second, nil
	})
	defer s.CloseYM()
	s.SeekYM(1000)
	waitSeekEvent(t, first.readStarted)
	s.SeekYM(250)
	info := waitReference(t, s, func(info Reference) bool { return !info.Seeking && info.Position == 250 })
	if info.Error != "" {
		t.Fatal(info.Error)
	}
	close(gate)
	waitSeekEvent(t, first.closed)
	// Wait through worker scheduling while asserting the retired job cannot
	// replace the committed renderer or publish its cancellation as an error.
	for i := 0; i < 20; i++ {
		time.Sleep(time.Millisecond)
		s.mu.Lock()
		current := ref.renderer
		s.mu.Unlock()
		info, _ = s.Reference()
		if current != second || info.Seeking || info.Error != "" {
			t.Fatalf("stale seek published: %+v", info)
		}
	}
}

func TestSNDHSeekReplacementAndCloseCancelStaleWork(t *testing.T) {
	for _, replacement := range []string{"YM", "close"} {
		t.Run(replacement, func(t *testing.T) {
			gate := make(chan struct{})
			next := newSeekTestRenderer()
			next.readGate = gate
			next.readStarted = make(chan struct{})
			s, _, _ := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) { return next, nil })
			defer s.CloseYM()
			s.SeekYM(1000)
			waitSeekEvent(t, next.readStarted)
			if replacement == "YM" {
				raw := make([]byte, 4+14*4)
				copy(raw, "YM3!")
				if err := s.LoadYM(raw); err != nil {
					t.Fatal(err)
				}
			} else {
				s.CloseYM()
			}
			close(gate)
			waitSeekEvent(t, next.closed)
			for i := 0; i < 10; i++ {
				time.Sleep(time.Millisecond)
				info, ok := s.Reference()
				if replacement == "YM" {
					if !ok || info.Format == "SNDH" || info.Seeking || info.Error != "" {
						t.Fatalf("stale SNDH replaced YM: %+v", info)
					}
				} else if ok {
					t.Fatal("stale seek resurrected a closed reference")
				}
			}
		})
	}
}

func TestSNDHSeekCommitPreservesPauseAndNativeSelection(t *testing.T) {
	gate := make(chan struct{})
	next := newSeekTestRenderer()
	next.readGate = gate
	next.readStarted = make(chan struct{})
	s, _, _ := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) { return next, nil })
	defer s.CloseYM()
	s.SeekYM(250)
	waitSeekEvent(t, next.readStarted)
	s.Engine.Position, s.Engine.Row = 1, 17
	s.PauseReference()
	close(gate)
	info := waitReference(t, s, func(info Reference) bool { return !info.Seeking && info.Position == 250 })
	if info.Playing || info.Active || s.Engine.Position != 1 || s.Engine.Row != 17 {
		t.Fatalf("seek changed current transport: %+v", info)
	}
}

func TestSNDHSeekFailureRetainsOldRendererAndReportsError(t *testing.T) {
	s, ref, old := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) { return nil, errors.New("test replay failure") })
	defer s.CloseYM()
	s.SeekYM(250)
	info := waitReference(t, s, func(info Reference) bool { return !info.Seeking && info.Error != "" })
	s.mu.Lock()
	unchanged := ref.renderer == old
	s.mu.Unlock()
	if !unchanged || !info.Playing || !info.Active {
		t.Fatalf("failed seek lost old playback: %+v", info)
	}
	s.SeekYM(3600001)
	info, _ = s.Reference()
	if info.Seeking || info.Error != "sndh: seek exceeds one hour" {
		t.Fatalf("invalid seek %+v", info)
	}
}

func TestSNDHSeekChecksCancellationAroundZeroPositionInit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := prepareSNDHSeek(ctx, nil, 1, 8000, 0, func(*sndh.File, int, int) (sndhRenderer, error) { called = true; return newSeekTestRenderer(), nil })
	if !errors.Is(err, context.Canceled) || called {
		t.Fatal("cancelled seek entered init")
	}
	ctx, cancel = context.WithCancel(context.Background())
	next := newSeekTestRenderer()
	_, err = prepareSNDHSeek(ctx, nil, 1, 8000, 0, func(*sndh.File, int, int) (sndhRenderer, error) { cancel(); return next, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("zero-position init ignored cancellation: %v", err)
	}
	waitSeekEvent(t, next.closed)
}

func TestSelectingSNDHReferenceStartsZeroSeekOutsideLock(t *testing.T) {
	initGate := make(chan struct{})
	initStarted := make(chan struct{})
	next := newSeekTestRenderer()
	s, _, _ := seekTestSynth(func(*sndh.File, int, int) (sndhRenderer, error) { close(initStarted); <-initGate; return next, nil })
	defer s.CloseYM()
	s.SelectReference(false)
	done := make(chan struct{})
	go func() { s.SelectReference(true); close(done) }()
	waitSeekEvent(t, done)
	waitSeekEvent(t, initStarted)
	info, _ := s.Reference()
	if !info.Seeking || !info.Active || !info.Playing {
		t.Fatalf("reference selection %+v", info)
	}
	close(initGate)
	waitReference(t, s, func(info Reference) bool { return !info.Seeking && info.Position == 0 })
}

func seekPlayerFixture() []byte {
	data := make([]byte, 512)
	for entry, target := range map[int]int{0: 96, 4: 128, 8: 144} {
		binary.BigEndian.PutUint16(data[entry:], 0x6000)
		binary.BigEndian.PutUint16(data[entry+2:], uint16(target-entry-2))
	}
	copy(data[12:], "SNDHTITLSeek fixture\x00COMMTest\x00##01\x00TC50\x00HDNS")
	copy(data[96:], []byte{0x4e, 0x75})
	copy(data[128:], []byte{0x4e, 0x75})
	at := 144
	for _, pair := range [][2]byte{{0, 100}, {1, 0}, {7, 0x3e}, {8, 15}} {
		for _, write := range [][2]byte{{0, pair[0]}, {2, pair[1]}} {
			copy(data[at:], []byte{0x13, 0xfc, 0, write[1], 0, 0xff, 0x88, write[0]})
			at += 8
		}
	}
	copy(data[at:], []byte{0x4e, 0x75})
	return data
}

func TestSNDHAsyncSeekMatchesFreshRendererAndTransport(t *testing.T) {
	s := NewSynth(New(model.Demo()), 8000)
	raw := seekPlayerFixture()
	if err := s.LoadSNDH(raw, 1); err != nil {
		t.Fatal(err)
	}
	defer s.CloseYM()
	if _, err := s.Read(make([]byte, 160*4)); err != nil {
		t.Fatal(err)
	}
	s.SeekYM(37)
	info := waitReference(t, s, func(info Reference) bool { return !info.Seeking && info.Position == 37 })
	if info.Error != "" || !info.Playing {
		t.Fatalf("completed seek %+v", info)
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := sndh.NewRenderer(file, 1, 8000)
	if err != nil {
		t.Fatal(err)
	}
	defer expected.Close()
	if _, err := io.ReadFull(expected, make([]byte, 296*4)); err != nil {
		t.Fatal(err)
	}
	want, got := make([]byte, 64*4), make([]byte, 64*4)
	if _, err := io.ReadFull(expected, want); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(s, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("seek retained primed audio or reached a different sample")
	}
	s.ToggleYM()
	if _, err := s.Read(got); err != nil {
		t.Fatal(err)
	}
	for _, value := range got {
		if value != 0 {
			t.Fatal("paused SNDH reference still produced sound")
		}
	}
	s.ToggleYM()
	if _, err := s.Read(got); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, make([]byte, len(got))) {
		t.Fatal("SNDH reference did not resume")
	}
}
