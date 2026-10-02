package ui

import "github.com/hajimehoshi/ebiten/v2/text/v2"

// fitText preserves Unicode boundaries while reserving space for adjacent UI
// controls. The original status remains available in the editor's state.
func (a *App) fitText(value string, size, maxWidth float64) string {
	face := &text.GoTextFace{Source: a.font, Size: size}
	if width, _ := text.Measure(value, face, 0); width <= maxWidth {
		return value
	}
	runes := []rune(value)
	low, high := 0, len(runes)
	for low < high {
		middle := (low + high + 1) / 2
		width, _ := text.Measure(string(runes[:middle])+"…", face, 0)
		if width <= maxWidth {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return string(runes[:low]) + "…"
}
