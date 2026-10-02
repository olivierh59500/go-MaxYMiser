package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// SetYMSourcePatterns selects the candidate-phrase view for initial displays.
func (a *App) SetYMSourcePatterns(enabled bool) { a.ymPatternView = enabled }

func (a *App) ymPatternAction(action string) bool {
	switch action {
	case "ym:source-patterns":
		a.ymPatternView = !a.ymPatternView
		return true
	case "ym:patterns-prev":
		a.ymPatternPage = max(0, a.ymPatternPage-1)
		return true
	case "ym:patterns-next":
		if a.ymReport != nil {
			a.ymPatternPage = min(max(0, (len(a.ymReport.SourcePatterns)-1)/6), a.ymPatternPage+1)
		}
		return true
	}
	if !strings.HasPrefix(action, "ym:pattern-hit:") {
		return false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(action, "ym:pattern-hit:"))
	if err != nil || a.ymReport == nil || id < 0 || id >= len(a.ymReport.SourcePatterns) || a.ymReport.SourceLabelRate < 1 {
		return true
	}
	hit := a.ymReport.SourcePatterns[id]
	a.synth.SelectReference(true)
	a.synth.SeekYM(uint32(hit.Start * 1000 / a.ymReport.SourceLabelRate))
	a.status = "Listening to the candidate phrase in the original YM; composition retained"
	return true
}

func (a *App) drawYMPatterns(dst *ebiten.Image) {
	title := "SOURCE PATTERN CANDIDATES"
	if a.ymReport != nil && a.ymReport.KnownSourcePair {
		title = "PATTERNS FROM THE PAIRED SOURCE"
	}
	a.text(dst, title, 42, 303, 15, fg)
	if a.ymReport == nil || len(a.ymReport.SourcePatterns) == 0 {
		a.text(dst, "Load a paired profile, then Reconstruct to search for known source phrases.", 42, 351, 12, dim)
		return
	}
	hits := a.ymReport.SourcePatterns
	a.ymPatternPage = min(a.ymPatternPage, max(0, (len(hits)-1)/6))
	for n := 0; n < 6; n++ {
		id := a.ymPatternPage*6 + n
		if id >= len(hits) {
			break
		}
		hit := hits[id]
		var ids []string
		for _, p := range hit.Patterns {
			ids = append(ids, fmt.Sprintf("%02X", p))
		}
		label := fmt.Sprintf("%s  %5d:%5d  source %s  distance %.2f", []string{"A", "B", "C"}[hit.Channel], hit.Start, hit.End, strings.Join(ids, " / "), hit.Distance)
		if hit.Known {
			label = fmt.Sprintf("%s  %5d:%5d  source %s  paired recording", []string{"A", "B", "C"}[hit.Channel], hit.Start, hit.End, strings.Join(ids, " / "))
		}
		a.btn(dst, label, 42, 343+n*37, 1152, 33, fmt.Sprintf("ym:pattern-hit:%d", id), false)
	}
	instruction := "Click a passage to hear the original YM. Multiple IDs mean the source pattern is ambiguous."
	if a.ymReport.KnownSourcePair {
		instruction = "Click a passage to hear the paired recording. Source IDs are known; time ranges use the profile alignment."
	}
	a.text(dst, instruction, 42, 590, 12, dim)
	a.btn(dst, "Previous", 42, 625, 142, 30, "ym:patterns-prev", false)
	a.btn(dst, "Next", 198, 625, 142, 30, "ym:patterns-next", false)
	kind := "candidate passages"
	if a.ymReport.KnownSourcePair {
		kind = "source passages"
	}
	a.text(dst, fmt.Sprintf("Page %d / %d · %d %s", a.ymPatternPage+1, (len(hits)+5)/6, len(hits), kind), 368, 635, 12, accent)
}
