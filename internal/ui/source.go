package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

// inspectSource retains original labels without replacing the editable project.
// Import is a separate action after the conversion's limitations are visible.
func (a *App) inspectSource(raw []byte, path string) error {
	score, err := ymimport.DecodeSource(raw, 0, 6000)
	if err != nil {
		return err
	}
	a.inspectDecodedSource(score, path)
	a.sourceData = append([]byte(nil), raw...)
	return nil
}

// Recognized source data remain inspectable when the selected native excerpt
// cannot be encoded. Failed conversion never becomes an importable preview.
func (a *App) inspectDecodedSource(score ymimport.SourceScore, path string) {
	a.cancelSNDHImport()
	p, report, err := ymimport.SourceProject(score, 0, score.Frames)
	a.sourceScore, a.sourcePreview, a.sourceReport = &score, p, &report
	a.sourcePath, a.sourcePage = path, 0
	a.sourceConversionError = ""
	a.tab = "YM"
	a.status = fmt.Sprintf("Recognized %s; source labels are available; current composition retained", score.Player)
	if err != nil {
		a.sourcePreview = nil
		a.sourceConversionError = err.Error()
		// Capacity or pitch failures can occur before a bank report is built.
		// Original sound definitions remain useful independently of the excerpt.
		if report.Bank.Unsupported == nil {
			_, bankReport, bankErr := ymimport.SourceVoiceBank(score)
			if bankErr == nil {
				report.Bank = bankReport
			}
		}
		a.status = "Source data recognized; choose another excerpt: " + err.Error()
	}
}

func (a *App) sourceAction(action string) bool {
	switch action {
	case "source:close":
		a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
		a.sourcePath = ""
		a.sourceData = nil
		a.sourceConversionError = ""
	case "source:previous":
		a.sourcePage = max(0, a.sourcePage-1)
	case "source:next":
		if a.sourceScore != nil {
			a.sourcePage = min(max(0, (len(a.sourceScore.Instruments)-1)/8), a.sourcePage+1)
		}
	case "source:previous-song", "source:next-song":
		if a.sourceScore == nil {
			return true
		}
		index := a.sourceScore.Subtune
		if action == "source:previous-song" {
			index--
		} else {
			index++
		}
		if index < 0 || index >= a.sourceScore.Subtunes {
			return true
		}
		raw := a.sourceData
		var err error
		if len(raw) == 0 {
			raw, err = os.ReadFile(a.sourcePath)
		}
		if err == nil {
			var score ymimport.SourceScore
			score, err = ymimport.DecodeSource(raw, index, a.sourceScore.Frames)
			if err == nil {
				a.inspectDecodedSource(score, a.sourcePath)
				return true
			}
		}
		a.status = "Source song retained: " + err.Error()
	case "source:range":
		if a.sourceReport != nil {
			a.modal = fmt.Sprintf("Source excerpt start:end (0–%d frames)", a.sourceScore.Frames)
			a.entry = fmt.Sprintf("%d:%d", a.sourceReport.StartFrame, a.sourceReport.EndFrame)
		}
	case "source:import":
		if a.sourcePreview == nil || a.sourceReport == nil {
			a.status = "Choose a convertible source excerpt before importing"
			if a.sourceConversionError != "" {
				a.status += ": " + a.sourceConversionError
			}
			return true
		}
		p := a.sourcePreview.Clone()
		p.Title = strings.TrimSuffix(filepath.Base(a.sourcePath), filepath.Ext(a.sourcePath)) + " excerpt"
		score, preview, report, path, raw := a.sourceScore, a.sourcePreview, a.sourceReport, a.sourcePath, a.sourceData
		a.acceptProject(p, a.sourcePath, "")
		a.sourceScore, a.sourcePreview, a.sourceReport, a.sourcePath, a.sourceData = score, preview, report, path, raw
		a.dirty = true
		a.status = fmt.Sprintf("Imported source excerpt; %d notes use unsupported sounds; Save as keeps the source separate", a.sourceReport.UnsupportedEvents)
	default:
		return false
	}
	return true
}

func (a *App) sourceModal(modal, entry string) bool {
	if !strings.HasPrefix(modal, "Source excerpt start:end (") {
		return false
	}
	if a.sourceScore == nil {
		return true
	}
	parts := strings.Split(entry, ":")
	if len(parts) != 2 {
		a.status = "Enter two frame numbers separated by a colon"
		return true
	}
	start, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil {
		a.status = "Enter decimal source-frame numbers"
		return true
	}
	p, report, err := ymimport.SourceProject(*a.sourceScore, start, end)
	if err != nil {
		a.status = err.Error()
		return true
	}
	a.sourcePreview, a.sourceReport = p, &report
	a.sourceConversionError = ""
	a.status = "Source excerpt updated; current composition retained"
	return true
}

func (a *App) drawSource(dst *ebiten.Image) {
	score, report := a.sourceScore, a.sourceReport
	a.text(dst, "SNDH SOURCE DATA · "+filepath.Base(a.sourcePath), 42, 212, 17, fg)
	a.text(dst, fmt.Sprintf("%s · %d source patterns · %d original instrument IDs", score.Player, len(score.Patterns), len(score.Instruments)), 42, 251, 12, accent)
	a.btn(dst, fmt.Sprintf("Excerpt %d:%d", report.StartFrame, report.EndFrame), 42, 279, 204, 32, "source:range", false)
	if a.sourcePreview != nil {
		a.btn(dst, "Import editable excerpt", 260, 279, 254, 32, "source:import", false)
	} else {
		a.btn(dst, "Choose another excerpt", 260, 279, 254, 32, "source:range", false)
	}
	a.btn(dst, "Close source", 1060, 279, 160, 32, "source:close", false)
	if score.Subtunes > 1 {
		a.btn(dst, "Previous song", 540, 279, 150, 32, "source:previous-song", false)
		a.btn(dst, "Next song", 704, 279, 128, 32, "source:next-song", false)
		a.text(dst, fmt.Sprintf("Song %d / %d", score.Subtune+1, score.Subtunes), 850, 289, 12, accent)
	}
	a.text(dst, fmt.Sprintf("%d translated sounds · %d unsupported · %d affected note events", len(report.Bank.Converted), len(report.Bank.Unsupported), report.UnsupportedEvents), 42, 330, 13, purple)
	detail := "Unsupported definitions stay silent. Pattern effects and some modulation are not converted."
	if report.ModulationSegments > 0 {
		detail = fmt.Sprintf("%d pitch segments · vibrato/slide for ordinary tones; unsupported programs stay silent.", report.ModulationSegments)
	}
	if a.sourceConversionError != "" {
		detail = a.sourceConversionError
	}
	a.text(dst, detail, 42, 358, 12, dim)
	for n := 0; n < 8; n++ {
		index := a.sourcePage*8 + n
		if index >= len(score.Instruments) {
			break
		}
		sound := score.Instruments[index]
		state := "translated volume / arpeggio"
		if report.Bank.Unsupported == nil {
			state = "original definition retained; conversion unavailable"
		}
		if reason, ok := report.Bank.Unsupported[sound.ID]; ok {
			state = reason
		}
		for _, id := range report.ScoreVolumeInstruments {
			if sound.ID == id {
				state = "volume in generated score / arpeggio in sound"
				break
			}
		}
		for _, id := range report.ScoreMixerInstruments {
			if sound.ID == id {
				state = "alternating mixer / shared noise in generated score"
				break
			}
		}
		a.text(dst, fmt.Sprintf("%02X  settings % X  ·  %s", sound.ID, sound.Settings, state), 42, float64(391+n*26), 11, fg)
	}
	a.btn(dst, "Execute source song", 422, 616, 220, 30, "sndh:select-source", false)
	a.btn(dst, "Previous sounds", 42, 616, 176, 30, "source:previous", false)
	a.btn(dst, "Next sounds", 232, 616, 176, 30, "source:next", false)
	var commands []int
	for command := range report.UntranslatedCommands {
		commands = append(commands, int(command))
	}
	sort.Ints(commands)
	a.text(dst, fmt.Sprintf("Untranslated pattern commands: %X · generated patterns: %d", commands, report.Patterns), 432, 627, 11, dim)
	a.text(dst, "Source pattern IDs and timing are known. Import creates an editable excerpt, not a complete original replay.", 42, 654, 11, dim)
}
