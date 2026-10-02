package replay

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// RegisterTick describes the main replay call. Subtick timer writes require a
// separate timestamped trace and must not be inferred from this fixture.
type RegisterTick struct {
	Registers     [14]byte
	EnvelopeWrite bool
}

// VerifyRegisterTrace compares every register and envelope write, rather than
// using PCM volume or successful decoding as evidence of replay fidelity.
func VerifyRegisterTrace(project *model.Project, expected []RegisterTick) error {
	if len(expected) == 0 {
		return fmt.Errorf("replay: reference trace is empty")
	}
	engine := New(project.Clone())
	engine.Play(false)
	for tick, reference := range expected {
		engine.Tick()
		for reg, value := range reference.Registers {
			if engine.Registers[reg] != value {
				return fmt.Errorf("replay: tick %d R%d: Go %02X, native %02X", tick, reg, engine.Registers[reg], value)
			}
		}
		if engine.EnvelopeWrite != reference.EnvelopeWrite {
			return fmt.Errorf("replay: tick %d envelope write: Go %t, native %t", tick, engine.EnvelopeWrite, reference.EnvelopeWrite)
		}
	}
	return nil
}
