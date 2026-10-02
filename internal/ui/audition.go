package ui

// AuditionInstrument triggers a sound without editing a pattern or resetting
// the arrangement. A retained YM reference is paused while the synth is heard.
func (a *App) AuditionInstrument(note byte) {
	a.synth.PreviewInstrument(min(a.channel, 2), note, byte(a.instrument+1))
	a.status = "Instrument preview · note keys audition the selected sound"
}
