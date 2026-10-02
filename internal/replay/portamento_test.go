package replay

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestNegativeDetunePortamentoRetainsNativeWordOverflow(t *testing.T) {
	e := New(model.New())
	v := Voice{Note: 40, Porta: -1123}
	v.Parameters[0], v.Parameters[5], v.Parameters[23], v.Parameters[24] = 2, 2, 251, 253
	// The native MULS low word is +17247 after overflowing -48289.
	// ASR.W #5 gives +538, followed by buzzer rounding and fine detune.
	if got := e.Period(&v, 1); got != 161 {
		t.Fatalf("portamento word overflow was lost: %d, want 161", got)
	}
	v.Porta = -1148
	if got := e.Period(&v, 1); got != 159 {
		t.Fatalf("captured overflow boundary differs: %d, want 159", got)
	}
}
