package ymimport

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"testing"
)

func simpleYM3(frames int) []byte {
	data := append([]byte(nil), []byte("YM3!")...)
	regs := [14]byte{28, 1, 0, 0, 0, 0, 0, 62, 15, 0, 0, 0, 0, 0}
	for _, r := range regs {
		for range frames {
			data = append(data, r)
		}
	}
	return data
}
func TestReconstructionFindsA4AndKeepsOriginalReferenceSeparate(t *testing.T) {
	trace, err := Decode(simpleYM3(128))
	if err != nil {
		t.Fatal(err)
	}
	p, report, err := Reconstruct(trace)
	if err != nil {
		t.Fatal(err)
	}
	if report.Instruments != 1 || report.Positions != 2 || p.Song.Patterns[0][0].Note != 69 {
		t.Fatalf("incorrect note/timbre estimate: %+v", report)
	}
	e := replay.New(p)
	e.Play(false)
	e.Tick()
	period := uint16(e.Registers[0]) | uint16(e.Registers[1])<<8
	if period != 284 || e.Registers[8] != 15 {
		t.Fatalf("candidate does not reproduce simple squarewave: regs=%v", e.Registers)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("reconstruction ambiguity was hidden")
	}
}
func TestTruncatedRegisterStreamIsRejected(t *testing.T) {
	if _, err := Decode([]byte("YM6!")); err == nil {
		t.Fatal("truncated YM6 header accepted")
	}
}
