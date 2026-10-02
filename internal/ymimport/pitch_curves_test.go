package ymimport

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestYMImportRetainsSubSemitonePitchAcrossPatternBoundariesAndReload(t *testing.T) {
	trace := Trace{Clock: 2000000, Rate: 50, Frames: make([][14]byte, 140)}
	for at := range trace.Frames {
		r := &trace.Frames[at]
		p0, p1 := 284+at%13-6, 239+at%17-8
		r[0], r[1], r[2], r[3] = byte(p0), byte(p0>>8), byte(p1), byte(p1>>8)
		r[7], r[8], r[9] = 60, 12, 9
	}
	before := append([][14]byte(nil), trace.Frames...)
	p, report, err := Reconstruct(trace)
	if err != nil || !report.ExactTonePeriods {
		t.Fatalf("exact pitch curves unavailable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "curves.mys")
	if err := project.Save(p, path); err != nil {
		t.Fatal(err)
	}
	again, err := project.Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	e := replay.New(again)
	e.Play(false)
	for at, r := range trace.Frames {
		e.Tick()
		for reg := 0; reg < 4; reg++ {
			if e.Registers[reg] != r[reg] {
				t.Fatalf("pitch rounded or curve restarted at frame %d R%d: %d, want %d", at, reg, e.Registers[reg], r[reg])
			}
		}
	}
	if !reflect.DeepEqual(trace.Frames, before) {
		t.Fatal("reconstruction changed the YM reference")
	}
}

func TestCoarseYMGridHoldsEachSelectedTonePeriodForItsWholeRow(t *testing.T) {
	trace, err := Decode(simpleYM3(90))
	if err != nil {
		t.Fatal(err)
	}
	for at := range trace.Frames {
		trace.Frames[at][0] = byte(282 + (at/6)%5)
		trace.Frames[at][1] = 1
	}
	p, report, err := ReconstructSelection(trace, ReconstructionOptions{StartFrame: 6, EndFrame: 84, FramesPerRow: 6})
	if err != nil || !report.ExactTonePeriods {
		t.Fatal(err)
	}
	e := replay.New(p)
	e.Play(false)
	for frame := 0; frame < 78; frame++ {
		e.Tick()
		want := trace.Frames[6+(frame/6)*6]
		if e.Registers[0] != want[0] || e.Registers[1] != want[1] {
			t.Fatalf("sampled curve ran faster than its %d-frame row", report.FramesPerRow)
		}
	}
}

func TestPitchCurveAllocationFailureRetainsTheUnmodifiedCandidate(t *testing.T) {
	trace, _ := Decode(simpleYM3(130))
	p := model.New()
	p.Song.Length = 3
	for pos := 0; pos < 3; pos++ {
		p.Song.Orders[pos] = [4]byte{0, 1, 2, 255}
	}
	p.Song.Patterns[0][0] = model.Cell{Note: 69, Instrument: 1}
	p.Bank.SequenceCount = model.MaxSequences
	before := p.Clone()
	if _, err := preserveToneCurves(p, trace); err == nil {
		t.Fatal("sequence capacity was exceeded without an error")
	}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("failed pitch allocation modified the candidate")
	}
}
