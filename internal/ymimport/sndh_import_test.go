package ymimport

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

func sndhImportWriteRegister(code []byte, register, value byte) []byte {
	return append(code, 0x13, 0xfc, 0, register, 0, 0xff, 0x88, 0,
		0x13, 0xfc, 0, value, 0, 0xff, 0x88, 2)
}

func sndhImportFixture(rate int, init, play []byte) []byte {
	data := make([]byte, 16)
	copy(data[12:], "SNDH")
	data = append(data, []byte(fmt.Sprintf("TITLGenerated import fixture\x00COMMTest\x00YEAR1991\x00TC%d\x00FLAG~y\x00HDNS", rate))...)
	if len(data)%2 != 0 {
		data = append(data, 0)
	}
	initAt := len(data)
	data = append(data, init...)
	exitAt := len(data)
	data = append(data, 0x4e, 0x75)
	playAt := len(data)
	data = append(data, play...)
	for i, target := range []int{initAt, exitAt, playAt} {
		at := i * 4
		binary.BigEndian.PutUint16(data[at:], 0x6000)
		binary.BigEndian.PutUint16(data[at+2:], uint16(target-at-2))
	}
	return data
}

func sndhImportDACFixture(rate int) []byte {
	init := sndhImportWriteRegister(nil, 7, 0x3f)
	init = append(init, 0x13, 0xfc, 0, 8, 0, 0xff, 0x88, 0, 0x72, 0, 0x4e, 0x75)
	// EORI.B #15,D1; MOVE.B D1,$ff8802; RTS. With tone and noise disabled,
	// this is a volume DAC waveform that PSG-note reconstruction cannot retain.
	play := []byte{0x0a, 1, 0, 15, 0x13, 0xc1, 0, 0xff, 0x88, 2, 0x4e, 0x75}
	return sndhImportFixture(rate, init, play)
}

func sndhImportCapacityFixture() []byte {
	var init []byte
	for _, pair := range [][2]byte{{0, 80}, {1, 0}, {7, 0x36}, {8, 15}} {
		init = sndhImportWriteRegister(init, pair[0], pair[1])
	}
	init = append(init, 0x72, 0, 0x4e, 0x75)
	play := []byte{
		0x52, 1, // ADDQ.B #1,D1.
		0x13, 0xfc, 0, 6, 0, 0xff, 0x88, 0,
		0x13, 0xc1, 0, 0xff, 0x88, 2,
		0x13, 0xfc, 0, 7, 0, 0xff, 0x88, 0,
		0x0c, 1, 0, 32, 0x65, 10, // CMPI.B #32,D1; BCS.S first mixer.
		0x13, 0xfc, 0, 0x37, 0, 0xff, 0x88, 2, 0x60, 8,
		0x13, 0xfc, 0, 0x36, 0, 0xff, 0x88, 2, 0x4e, 0x75,
	}
	return sndhImportFixture(50, init, play)
}

func sndhImportLevels(t *testing.T, reader interface{ Read([]byte) (int, error) }, frames int) []float64 {
	t.Helper()
	buffer := make([]byte, frames*4)
	n, err := reader.Read(buffer)
	if err != nil || n != len(buffer) {
		t.Fatalf("audio: bytes=%d/%d, error=%v", n, len(buffer), err)
	}
	levels := make([]float64, frames)
	for i := range levels {
		levels[i] = float64(int16(binary.LittleEndian.Uint16(buffer[i*4:])))
	}
	return levels
}

func sndhImportCorrelation(a, b []float64) float64 {
	var aa, bb, ab, sumA, sumB float64
	for i := range a {
		aa += a[i] * a[i]
		bb += b[i] * b[i]
		ab += a[i] * b[i]
		sumA += a[i]
		sumB += b[i]
	}
	n := float64(len(a))
	denominator := math.Sqrt((aa - sumA*sumA/n) * (bb - sumB*sumB/n))
	if denominator == 0 {
		return 0
	}
	return (ab - sumA*sumB/n) / denominator
}

func TestImportSNDHKeepsPlayableRegisterScoreAndSelection(t *testing.T) {
	var init []byte
	for _, pair := range [][2]byte{{0, 80}, {1, 0}, {7, 0x3e}, {8, 15}} {
		init = sndhImportWriteRegister(init, pair[0], pair[1])
	}
	init = append(init, 0x4e, 0x75)
	raw := sndhImportFixture(50, init, []byte{0x4e, 0x75})
	trace, p, report, err := ImportSNDHSelection(context.Background(), raw, 1, ReconstructionOptions{StartFrame: 20, EndFrame: 100, FramesPerRow: 2})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourcePlayer != sndhInferredPlayer || report.StartFrame != 20 || report.EndFrame != 100 || report.SourceLabelRate != 50 || len(trace.Frames) != 100 || p.Song.Speed() != 2 || sndhPSGNotes(p) == 0 || p.Year != "1991" {
		t.Fatalf("inferred score: %+v", report)
	}
	if len(p.Bank.Samples[0].PCM) != 0 || !strings.Contains(strings.Join(report.Warnings, " "), "original SNDH instrument names and programs are not recovered") {
		t.Fatal("ordinary PSG score was replaced with sampled audio or lacked provenance")
	}
}

func TestImportSNDHSampledDACSurvivesSaveReloadAndMatchesSource(t *testing.T) {
	raw := sndhImportDACFixture(50)
	trace, p, report, err := ImportSNDH(context.Background(), raw, 1, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Frames) != 80 || report.SourcePlayer != sndhSampledPlayer || report.Instruments != 2 || report.EndFrame != 80 || p.Song.State[49] != 3 || p.Song.Speed() != 1 || sndhPSGNotes(p) != 0 {
		t.Fatalf("sampled excerpt: %+v", report)
	}
	if !strings.Contains(strings.Join(report.Warnings, " "), "not recovered original instruments") {
		t.Fatal("sampled excerpt was not labeled honestly")
	}
	for slot := 0; slot < 2; slot++ {
		pcm := p.Bank.Samples[slot].PCM
		if len(pcm) < 2 || len(pcm) > sndhSampleLimit || pcm[0] != 0 {
			t.Fatalf("sample %d guard/capacity: %d bytes", slot, len(pcm))
		}
	}
	path := filepath.Join(t.TempDir(), "sampled.mys")
	if err := project.Save(p, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	beforeSong, _ := native.EncodeSong(p.Song)
	afterSong, _ := native.EncodeSong(loaded.Song)
	beforeBank, _ := native.EncodeVoiceBank(p.Bank)
	afterBank, _ := native.EncodeVoiceBank(loaded.Bank)
	if !bytes.Equal(beforeSong, afterSong) || !bytes.Equal(beforeBank, afterBank) {
		t.Fatal("sampled native save/reload changed song or sample bytes")
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	original, err := sndh.NewRenderer(file, 1, sndhSampleRate)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	frames := 80 * sndhSampleRate / 50
	source := sndhImportLevels(t, original, frames)
	engine := replay.New(loaded)
	engine.Play(false)
	audio := sndhImportLevels(t, replay.NewSynth(engine, sndhSampleRate), frames)
	// Exclude filter startup. Correlation compares the actual source waveform,
	// so a chip-floor transient cannot make an empty editable score pass.
	skip := sndhSampleRate / 10
	if correlation := sndhImportCorrelation(source[skip:], audio[skip:]); correlation < 0.98 {
		t.Fatalf("source/sample waveform correlation = %.6f", correlation)
	}
	minimum, maximum := audio[skip], audio[skip]
	for _, value := range audio[skip:] {
		minimum, maximum = min(minimum, value), max(maximum, value)
	}
	if maximum-minimum < 1000 {
		t.Fatal("sampled editable output is only a small startup transient")
	}
	edited := loaded.Clone()
	for i := range edited.Bank.Samples {
		clear(edited.Bank.Samples[i].PCM)
	}
	editedEngine := replay.New(edited)
	editedEngine.Play(false)
	muted := sndhImportLevels(t, replay.NewSynth(editedEngine, sndhSampleRate), frames)
	if sndhImportCorrelation(audio[skip:], muted[skip:]) > 0.1 {
		t.Fatal("editing native samples did not remove the source waveform")
	}
	clock := replay.New(loaded)
	clock.Play(false)
	for i := 0; i < 79; i++ {
		clock.Tick()
	}
	if clock.Loops != 0 {
		t.Fatal("sampled excerpt ended early")
	}
	clock.Tick()
	if clock.Loops != 1 {
		t.Fatal("partial final pattern extended the sampled excerpt")
	}
}

func TestImportSNDHSamplesCapacityFailureAndBoundsLowRateRange(t *testing.T) {
	_, p, report, err := ImportSNDH(context.Background(), sndhImportCapacityFixture(), 1, 80)
	if err != nil {
		t.Fatal(err)
	}
	if report.SourcePlayer != sndhSampledPlayer || len(p.Bank.Samples[0].PCM) == 0 || !strings.Contains(strings.Join(report.Warnings, " "), "more than 32 distinct timbres") {
		t.Fatalf("capacity failure did not retain sampled audio: %+v", report)
	}
	trace, p, report, err := ImportSNDHSelection(context.Background(), sndhImportDACFixture(25), 1, ReconstructionOptions{StartFrame: 5, EndFrame: 405, FramesPerRow: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Frames) != 405 || report.StartFrame != 5 || report.EndFrame != 261 || report.Frames != 256 || report.Instruments != 8 || report.FramesPerRow != 1 || p.Song.TickRate() != 25 {
		t.Fatalf("bounded later excerpt: %+v", report)
	}
	if !strings.Contains(strings.Join(report.Warnings, " "), "at most eight samples") {
		t.Fatal("truncated sampled excerpt was not reported")
	}
	for _, sample := range p.Bank.Samples {
		if len(sample.PCM) < 2 || len(sample.PCM) > 32768 {
			t.Fatalf("native sample capacity = %d", len(sample.PCM))
		}
	}
}

func TestImportSNDHCancellationAndLaterDefaultRange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, p, _, err := ImportSNDH(ctx, sndhImportDACFixture(50), 1, 400)
	if !errors.Is(err, context.Canceled) || p != nil {
		t.Fatalf("cancelled import returned an editable project: %v", err)
	}
	trace, p, report, err := ImportSNDHSelection(context.Background(), sndhImportDACFixture(50), 1, ReconstructionOptions{StartFrame: 450})
	if err != nil || len(trace.Frames) != 850 || p == nil || report.StartFrame != 450 || report.EndFrame != 850 {
		t.Fatalf("later default range: %+v, error=%v", report, err)
	}
}

func TestImportSNDHFastCaptureUsesSampleCapacityAcrossPatterns(t *testing.T) {
	_, p, report, err := ImportSNDH(context.Background(), sndhImportDACFixture(200), 1, 1600)
	if err != nil {
		t.Fatal(err)
	}
	if report.SourcePlayer != sndhSampledPlayer || report.EndFrame != 1600 || report.Frames != 1600 || report.Instruments != 7 || p.Song.TickRate() != 200 || p.Song.Length != 25 {
		t.Fatalf("fast capture was truncated at pattern boundaries: %+v", report)
	}
	second := p.Song.Patterns[p.Song.Orders[4][3]][5]
	if second.Note != sndhSampleNote || second.Instrument != 2 {
		t.Fatal("a sample spanning four patterns did not retain its absolute trigger row")
	}
	for _, sample := range p.Bank.Samples {
		if len(sample.PCM) > sndhSampleLimit {
			t.Fatal("sample exceeded native capacity")
		}
	}
}
