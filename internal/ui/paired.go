package ui

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func (a *App) LoadPairedProfile(path string) error {
	profile, err := ymimport.LoadPairedProfile(path)
	if err != nil {
		return err
	}
	a.pairedProfile = &profile
	if profile.Corpus != nil {
		a.status = fmt.Sprintf("Experimental corpus: %d compositions, %d sound definitions; labels remain candidates", len(profile.Corpus.Groups), len(profile.Source.Instruments))
	} else {
		a.status = fmt.Sprintf("Loaded %s labelled profile; %d/%d held-out source labels correct", profile.Source.Player, profile.Validation.Correct, profile.Validation.Known)
	}
	return nil
}

// ReconstructYM applies the same editing workflow used by the YM workspace.
func (a *App) ReconstructYM(options ymimport.ReconstructionOptions) {
	a.ymOptions = options
	a.action("ym:infer")
}
