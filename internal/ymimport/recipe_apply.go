package ymimport

import (
	"fmt"
	"math"
	"sort"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

type RecipeApplication struct {
	SourceInstrument  int     `json:"source_instrument"`
	TrackerInstrument int     `json:"tracker_instrument"`
	Channel           int     `json:"channel"`
	Start             int     `json:"start_frame"`
	End               int     `json:"end_frame"`
	BeforeError       float64 `json:"before_register_error"`
	AfterError        float64 `json:"after_register_error"`
}

// ApplyPairedRecipes uses source-labelled candidates, then checks their output
// against the target YM. A candidate is kept only if replay error decreases.
// It never uses the training score's event times or note IDs for the target.
func ApplyPairedRecipes(project *model.Project, report *Report, trace Trace, profile PairedProfile) error {
	if project == nil || report == nil || report.StartFrame < 0 || report.EndFrame > len(trace.Frames) || report.EndFrame <= report.StartFrame {
		return fmt.Errorf("pair: invalid reconstruction selection")
	}
	selection := trace
	selection.Frames = trace.Frames[report.StartFrame:report.EndFrame]
	labels := profile.SourceEvidence(selection)
	for i := range labels {
		labels[i].Start += report.StartFrame
		labels[i].End += report.StartFrame
	}
	report.SourcePlayer, report.SourceLabelRate, report.SourceLabels = profile.Source.Player, trace.Rate, labels
	if profile.Corpus != nil {
		report.SourceCorpusGroups = append([]string(nil), profile.Corpus.Groups...)
		report.Warnings = appendUnique(report.Warnings, "Corpus sound labels are experimental similarity candidates; an unseen native definition can resemble a stored sound.")
	}
	known, matched := profile.KnownPatternEvidence(trace, report.StartFrame, report.EndFrame)
	if matched {
		report.SourcePatterns = known
		report.KnownSourcePair = true
	} else {
		report.SourcePatterns = profile.PatternEvidence(selection)
		for i := range report.SourcePatterns {
			report.SourcePatterns[i].Start += report.StartFrame
			report.SourcePatterns[i].End += report.StartFrame
		}
	}
	if report.FramesPerRow != 1 {
		report.Warnings = appendUnique(report.Warnings, "Source instrument recipes require the one-frame reconstruction grid; labels remain available on coarser grids.")
		return nil
	}
	byID := map[int][]InstrumentRecipe{}
	for _, recipe := range profile.Recipes {
		if recipe.RegisterError <= 0.5 {
			byID[recipe.SourceInstrument] = append(byID[recipe.SourceInstrument], recipe)
		}
	}
	used := report.Instruments
	allocated := map[string]byte{}
	baseline, instrumentState := reconstructionFrames(project, report.EndFrame-report.StartFrame)
	editable := map[[2]int]bool{}
	for _, label := range labels {
		recipes := byID[label.Instrument]
		if len(recipes) == 0 || label.Start < report.StartFrame || label.End > report.EndFrame || label.End-label.Start < 3 {
			continue
		}
		features := eventFeatures(trace, label.Channel, label.Start, label.End)
		ranked := append([]InstrumentRecipe(nil), recipes...)
		sort.SliceStable(ranked, func(i, j int) bool {
			return featureDistance(ranked[i].Features, features) < featureDistance(ranked[j].Features, features)
		})
		// The learned first arpeggio offset may not be zero. Recover a target
		// base note from pitch evidence and that editable offset.
		note, tonal := tracePitch(trace, label.Channel, label.Start)
		if !tonal {
			continue
		}
		length := label.End - label.Start
		if length < 3 {
			continue
		}
		candidate := InstrumentRecipe{}
		after := math.Inf(1)
		for _, r := range ranked[:min(16, len(ranked))] {
			period := int(trace.Frames[label.Start][label.Channel*2]) | int(trace.Frames[label.Start][label.Channel*2+1]&15)<<8
			var ok bool
			r, ok = fitRecipePitch(r, note-int(int16(r.Sequences[1].Values[0])), period)
			if !ok {
				continue
			}
			error := recipeError(r, trace, label.Channel, label.Start, label.Start+length)
			if error < after && recipePreservesLevel(r, trace, baseline, label.Channel, label.Start, label.Start+length, report.StartFrame) {
				candidate, after = r, error
			}
		}
		// The baseline was rendered once, including preceding envelope state,
		// to compare against the complete existing candidate.
		before := scorePassageError(baseline, trace, label.Channel, label.Start, label.Start+length, report.StartFrame)
		if after > 0.5 || after >= before {
			continue
		}
		recipe := candidate
		note = candidate.Note
		needed := 0
		at := label.Start - report.StartFrame
		endAt := label.Start + length - report.StartFrame
		lastPos := (endAt - 1) / 64
		if endAt < len(baseline) {
			lastPos = endAt / 64
		}
		for pos := at / 64; pos <= lastPos; pos++ {
			if !editable[[2]int{pos, label.Channel}] {
				needed++
			}
		}
		if len(project.Song.Patterns)+needed > model.MaxPatterns {
			continue
		}
		programKey := fmt.Sprint(recipe.SourceInstrument, ":", recipe.TrainingSource, ":", recipe.SourceFrame, ":", recipe.Instrument, ":", recipe.Sequences)
		id, exists := allocated[programKey]
		if !exists {
			if used >= model.MaxInstruments || project.Bank.SequenceCount+5 > model.MaxSequences {
				continue
			}
			id = byte(used + 1)
			project.Bank.Instruments[id-1] = recipe.Instrument
			for i, off := range []int{48, 49, 50, 51, 52} {
				sequence := project.Bank.SequenceCount
				project.Bank.Sequences[sequence] = recipe.Sequences[i]
				project.Bank.Instruments[id-1][off] = byte(sequence)
				project.Bank.SequenceCount++
			}
			used++
			allocated[programKey] = id
		}
		// An inferred 64-frame pattern can be shared in several passages.
		// Clone it before a labelled occurrence is changed independently.
		position := at / 64
		for pos := position; pos <= (endAt-1)/64; pos++ {
			old := project.Song.Orders[pos][label.Channel]
			if old >= 240 || int(old) >= len(project.Song.Patterns) {
				return fmt.Errorf("pair: invalid reconstructed pattern reference")
			}
			newID := old
			key := [2]int{pos, label.Channel}
			if !editable[key] {
				if len(project.Song.Patterns) >= model.MaxPatterns {
					report.Warnings = appendUnique(report.Warnings, "Source recipe insertion reached the native pattern capacity.")
					return nil
				}
				newID = byte(len(project.Song.Patterns))
				project.Song.Patterns = append(project.Song.Patterns, project.Song.Patterns[old])
				project.Song.Orders[pos][label.Channel] = newID
				editable[key] = true
			}
			pattern := &project.Song.Patterns[newID]
			for tick := max(at, pos*64); tick < min(endAt, (pos+1)*64); tick++ {
				pattern[tick%64] = model.Cell{}
				if tick == at {
					pattern[tick%64] = model.Cell{Note: byte(note), Instrument: id}
				}
			}
		}
		// The following frame must re-establish the transcription's voice
		// state, even if it previously inherited an instrument from this event.
		if endAt < len(baseline) {
			pos, row := endAt/64, endAt%64
			old := project.Song.Orders[pos][label.Channel]
			if !editable[[2]int{pos, label.Channel}] && len(project.Song.Patterns) < model.MaxPatterns {
				project.Song.Patterns = append(project.Song.Patterns, project.Song.Patterns[old])
				old = byte(len(project.Song.Patterns) - 1)
				project.Song.Orders[pos][label.Channel] = old
				editable[[2]int{pos, label.Channel}] = true
			}
			if editable[[2]int{pos, label.Channel}] {
				cell := &project.Song.Patterns[old][row]
				if cell.Note == 0 {
					if n, ok := tracePitch(trace, label.Channel, label.End); ok {
						cell.Note = byte(max(2, min(127, n)))
					}
				}
				if cell.Instrument == 0 {
					cell.Instrument = instrumentState[endAt][label.Channel]
				}
			}
		}
		report.RecipeApplications = append(report.RecipeApplications, RecipeApplication{label.Instrument, int(id), label.Channel, label.Start, label.Start + length, before, after})
	}
	report.Instruments, report.Patterns = used, len(project.Song.Patterns)
	if len(report.RecipeApplications) > 0 {
		report.Warnings = appendUnique(report.Warnings, "Source-labelled instrument recipes were accepted only where their measured register error improved the candidate; other passages keep the frame transcription.")
	}
	return nil
}

// fitRecipePitch preserves the learned relative period trajectory, anchored to
// the target's first observed tone. Absolute vibrato words are recalculated for
// the target note instead of copying a correction measured at another pitch.
// The caller still measures the complete candidate against the target passage.
func fitRecipePitch(r InstrumentRecipe, note, period int) (InstrumentRecipe, bool) {
	if note < 2 || note > 127 || period < 1 || period > 4095 || r.Sequences[2].Length < 1 || r.Sequences[2].Length > 63 || r.Sequences[3].Values[0]&0x100 == 0 {
		return r, false
	}
	firstNote := r.Note + int(int16(r.Sequences[1].Values[0]))
	if firstNote < 2 || firstNote > 127 {
		return r, false
	}
	firstPeriod := int(replay.TonePeriod(firstNote)) - int(int16(r.Sequences[2].Values[0]))
	if firstPeriod < 1 || firstPeriod > 4095 {
		return r, false
	}
	for i := 0; i < int(r.Sequences[2].Length); i++ {
		if r.Sequences[3].Values[i]&0x100 == 0 {
			continue
		}
		arp := int(int16(r.Sequences[1].Values[i]))
		if r.Note+arp < 2 || r.Note+arp > 127 || note+arp < 2 || note+arp > 127 {
			return r, false
		}
		original := int(replay.TonePeriod(r.Note+arp)) - int(int16(r.Sequences[2].Values[i]))
		if original < 1 || original > 4095 {
			return r, false
		}
		wanted := int(math.Round(float64(original) * float64(period) / float64(firstPeriod)))
		if wanted < 1 || wanted > 4095 {
			return r, false
		}
		r.Sequences[2].Values[i] = uint16(int16(int(replay.TonePeriod(note+arp)) - wanted))
	}
	r.Note = note
	return r, true
}

func recipePreservesLevel(recipe InstrumentRecipe, trace Trace, baseline [][14]byte, ch, start, end, selectionStart int) bool {
	p := model.New()
	p.Bank.Instruments[0] = recipe.Instrument
	for i, off := range []int{48, 49, 50, 51, 52} {
		p.Bank.Sequences[i+1] = recipe.Sequences[i]
		p.Bank.Instruments[0][off] = byte(i + 1)
	}
	e := replay.New(p)
	e.Trigger(ch, byte(recipe.Note), 1)
	before, after := 0, 0
	for frame := start; frame < end; frame++ {
		e.Tick()
		r := trace.Frames[frame]
		old := baseline[frame-selectionStart]
		before += abs(int(old[8+ch]) - int(r[8+ch]))
		after += abs(int(e.Registers[8+ch]) - int(r[8+ch]))
		if (e.Registers[7]>>ch)&9 != (r[7]>>ch)&9 && (old[7]>>ch)&9 == (r[7]>>ch)&9 {
			return false
		}
	}
	return after <= before
}

func reconstructionFrames(project *model.Project, frames int) ([][14]byte, [][3]byte) {
	e := replay.New(project.Clone())
	e.Play(false)
	out := make([][14]byte, frames)
	state := make([][3]byte, frames)
	for i := range out {
		e.Tick()
		out[i] = e.Registers
		for ch := range 3 {
			state[i][ch] = e.Voices[ch].Instrument
		}
	}
	return out, state
}

func scorePassageError(baseline [][14]byte, trace Trace, ch, start, end, selectionStart int) float64 {
	sum, count := 0.0, 0
	for at := start; at < end; at++ {
		r := trace.Frames[at]
		actualRegisters := baseline[at-selectionStart]
		if r[8+ch] != 0 && r[7]&(1<<ch) == 0 {
			wanted := int(r[ch*2]) | int(r[ch*2+1]&15)<<8
			actual := int(actualRegisters[ch*2]) | int(actualRegisters[ch*2+1]&15)<<8
			sum += math.Abs(float64(wanted - actual))
			count++
		}
		sum += math.Abs(float64(int(r[8+ch]) - int(actualRegisters[8+ch])))
		count++
		if (r[7]>>ch)&9 != (actualRegisters[7]>>ch)&9 {
			sum += 16
		}
		count++
	}
	return sum / float64(max(1, count))
}
