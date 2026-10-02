package ymimport

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type PairAlignment struct {
	Offset         int     `json:"ym_frame_offset"`
	Transpose      int     `json:"transpose_semitones"`
	Checked        int     `json:"checked_tonal_events"`
	Matched        int     `json:"matched_tonal_events"`
	PitchAgreement float64 `json:"pitch_agreement"`
}

type PairedPrototype struct {
	Instrument int     `json:"source_instrument"`
	Features   []int16 `json:"features"`
	Examples   int     `json:"examples"`
}

type PairedProfile struct {
	Version           int                `json:"version"`
	Source            SourceScore        `json:"source"`
	YMFile            string             `json:"ym_file"`
	Alignment         PairAlignment      `json:"alignment"`
	TrainingEnd       int                `json:"training_end_source_frame"`
	Prototypes        []PairedPrototype  `json:"prototypes"`
	Validation        PairValidation     `json:"validation"`
	Warnings          []string           `json:"warnings"`
	Recipes           []InstrumentRecipe `json:"instrument_recipes,omitempty"`
	Patterns          []PatternPrototype `json:"pattern_prototypes,omitempty"`
	PatternValidation PatternValidation  `json:"pattern_validation"`
	ReferenceIdentity string             `json:"reference_register_identity,omitempty"`
	ReferencePatterns []PatternEvidence  `json:"reference_source_patterns,omitempty"`
}

type PairValidation struct {
	Events    int     `json:"held_out_events"`
	Known     int     `json:"known_instrument_events"`
	Correct   int     `json:"correct_known_labels"`
	Abstained int     `json:"abstained_known_events"`
	Accuracy  float64 `json:"known_label_accuracy"`
	Precision float64 `json:"accepted_label_precision"`
}

type SourceEvidence struct {
	Channel    int     `json:"channel"`
	Start      int     `json:"ym_start_frame"`
	End        int     `json:"ym_end_frame"`
	Instrument int     `json:"source_instrument"`
	Distance   float64 `json:"feature_distance"`
	Margin     float64 `json:"runner_up_distance_margin"`
}

func tracePitch(trace Trace, ch, at int) (int, bool) {
	if at < 0 || at >= len(trace.Frames) {
		return 0, false
	}
	r := trace.Frames[at]
	period := int(r[2*ch]) | int(r[2*ch+1]&15)<<8
	if period == 0 || r[8+ch]&31 == 0 || r[7]&(1<<ch) != 0 {
		return 0, false
	}
	return int(math.Round(69 + 12*math.Log2(float64(trace.Clock)/16/float64(period)/440))), true
}

// AlignPair searches recording lead-in and a fixed pitch transpose. At least
// twenty tonal events with 90% agreement are required; titles are not evidence.
// Timing changes and channel permutations require a different alignment model.
func AlignPair(score SourceScore, trace Trace) (PairAlignment, error) {
	if score.Rate != trace.Rate || trace.Rate < 1 || trace.Clock == 0 || len(score.Events) == 0 {
		return PairAlignment{}, fmt.Errorf("pair: source and YM must have the same frame rate and source notes")
	}
	lastFrames := [3]int{-1, -1, -1}
	for _, e := range score.Events {
		if e.Channel < 0 || e.Channel >= 3 || e.Frame < 0 || e.Frame <= lastFrames[e.Channel] || e.Instrument < -1 || e.Instrument >= len(score.Instruments) {
			return PairAlignment{}, fmt.Errorf("pair: invalid source event channel, ordering or instrument")
		}
		lastFrames[e.Channel] = e.Frame
	}
	best := PairAlignment{}
	bestQuality := -1.0
	for offset := -200; offset <= 500; offset++ {
		var differences [51]int
		checked := 0
		for _, e := range score.Events {
			if e.Rest || e.Channel < 0 || e.Channel >= 3 || e.Instrument < 0 || e.Instrument >= len(score.Instruments) || !e.Retrigger {
				continue
			}
			inst := score.Instruments[e.Instrument]
			// Noise/envelope/slide instruments do not expose a direct note.
			if len(inst.Settings) < 6 || inst.Settings[0]&0x7f != 0 {
				continue
			}
			note, ok := tracePitch(trace, e.Channel, e.Frame+offset)
			if !ok {
				continue
			}
			checked++
			if difference := note - e.Note; difference >= -25 && difference <= 25 {
				differences[difference+25]++
			}
		}
		for transpose := -24; transpose <= 24; transpose++ {
			at := transpose + 25
			current := PairAlignment{Offset: offset, Transpose: transpose, Checked: checked, Matched: differences[at-1] + differences[at] + differences[at+1]}
			if current.Checked < 20 {
				continue
			}
			current.PitchAgreement = float64(current.Matched) / float64(current.Checked)
			quality := current.PitchAgreement * math.Sqrt(float64(current.Checked))
			if quality > bestQuality || quality == bestQuality && (abs(current.Offset) < abs(best.Offset) || abs(current.Offset) == abs(best.Offset) && abs(current.Transpose) < abs(best.Transpose)) {
				best, bestQuality = current, quality
			}
		}
	}
	if best.Checked < 20 || best.PitchAgreement < 0.9 {
		return best, fmt.Errorf("pair: alignment not verified (%d/%d tonal events agree)", best.Matched, best.Checked)
	}
	return best, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type labelledFeatures struct {
	event      SourceEvent
	features   []int16
	start, end int
}

func sourceFeatures(score SourceScore, trace Trace, alignment PairAlignment) []labelledFeatures {
	var results []labelledFeatures
	for ch := 0; ch < 3; ch++ {
		var events []SourceEvent
		for _, e := range score.Events {
			if e.Channel == ch {
				events = append(events, e)
			}
		}
		for i, e := range events {
			if i+1 == len(events) {
				// The next decoded source event is needed to bound the example.
				continue
			}
			start := alignedSourceOnset(score, trace, e, alignment)
			end := min(start+64, len(trace.Frames))
			end = min(end, alignedSourceOnset(score, trace, events[i+1], alignment))
			if e.Rest || !e.Retrigger || e.Instrument < 0 || start < 0 || start >= len(trace.Frames) || end-start < 3 {
				continue
			}
			if trace.Frames[start][8+ch]&31 == 0 {
				continue
			}
			results = append(results, labelledFeatures{e, eventFeatures(trace, ch, start, end), start, end})
		}
	}
	return results
}

// Register recordings may sample a trigger one frame before or after the
// globally aligned boundary. Native envelope values locate that boundary;
// no target-only inference uses source note times from this operation.
func alignedSourceOnset(score SourceScore, trace Trace, e SourceEvent, alignment PairAlignment) int {
	at := e.Frame + alignment.Offset
	if e.Rest || !e.Retrigger || e.Instrument < 0 || e.Instrument >= len(score.Instruments) {
		return at
	}
	inst := score.Instruments[e.Instrument]
	if len(inst.Settings) != 6 || len(inst.VolumeSequence) == 0 {
		return at
	}
	best, error := at, math.Inf(1)
	for candidate := at - 2; candidate <= at+2; candidate++ {
		if candidate < 0 || candidate+3 > len(trace.Frames) {
			continue
		}
		current := 0.0
		index, count := 0, int(inst.Settings[5])
		for frame := 0; frame < 3; frame++ {
			count--
			if count < 0 {
				count = int(inst.Settings[5])
				index = min(index+1, len(inst.VolumeSequence)-1)
			}
			got := trace.Frames[candidate+frame][8+e.Channel]
			if got&16 != 0 {
				current += 16
			} else {
				current += math.Abs(float64(int(got) - int(inst.VolumeSequence[index])))
			}
		}
		if note, ok := tracePitch(trace, e.Channel, candidate); ok && inst.Settings[0]&0x7f == 0 {
			current += float64(abs(note - e.Note - alignment.Transpose))
		}
		if current < error || current == error && abs(candidate-at) < abs(best-at) {
			best, error = candidate, current
		}
	}
	return best
}

// LearnPair uses native event boundaries and IDs for the training targets. The
// final chronological quarter is held out before prototypes are constructed.
// This is a labelled nearest-prototype model, not a general neural network.
func LearnPair(score SourceScore, trace Trace, ymFile string) (PairedProfile, error) {
	alignment, err := AlignPair(score, trace)
	p := PairedProfile{Version: 1, Source: score, YMFile: ymFile, Alignment: alignment}
	if err != nil {
		return p, err
	}
	labelled := sourceFeatures(score, trace, alignment)
	last := 0
	for _, f := range labelled {
		last = max(last, f.event.Frame)
	}
	p.TrainingEnd = last * 3 / 4
	if score.Frames > 0 {
		p.TrainingEnd = min(score.Frames, max(0, len(trace.Frames)-alignment.Offset)) * 3 / 4
	}
	byID := map[string]int{}
	known := map[int]bool{}
	for _, f := range labelled {
		if f.event.Frame >= p.TrainingEnd || f.end > p.TrainingEnd+alignment.Offset {
			continue
		}
		id := fmt.Sprint(f.event.Instrument) + ":" + signature("source", f.features)
		if index, ok := byID[id]; ok {
			p.Prototypes[index].Examples++
		} else {
			byID[id] = len(p.Prototypes)
			p.Prototypes = append(p.Prototypes, PairedPrototype{f.event.Instrument, f.features, 1})
		}
		known[f.event.Instrument] = true
	}
	if len(p.Prototypes) == 0 {
		return p, fmt.Errorf("pair: no labelled training events")
	}
	p.Recipes = learnRecipes(score, trace, alignment, p.TrainingEnd)
	p.Patterns = learnPatternPrototypes(score, trace, alignment, p.TrainingEnd)
	p.PatternValidation = validatePatternEvidence(score, trace, p)
	if !trace.Effects {
		p.ReferenceIdentity = registerIdentity(trace)
		for _, passage := range sourcePassages(score, trace, alignment) {
			p.ReferencePatterns = append(p.ReferencePatterns, PatternEvidence{Channel: passage.channel, Start: passage.start, End: passage.end, Patterns: []int{passage.pattern}, Known: true})
		}
	}
	for _, f := range labelled {
		if f.event.Frame < p.TrainingEnd {
			continue
		}
		p.Validation.Events++
		if !known[f.event.Instrument] {
			continue
		}
		p.Validation.Known++
		id, _, _, ok := p.MatchSource(f.features)
		if !ok {
			p.Validation.Abstained++
		} else if id == f.event.Instrument {
			p.Validation.Correct++
		}
	}
	if p.Validation.Known > 0 {
		p.Validation.Accuracy = float64(p.Validation.Correct) / float64(p.Validation.Known)
	}
	if accepted := p.Validation.Known - p.Validation.Abstained; accepted > 0 {
		p.Validation.Precision = float64(p.Validation.Correct) / float64(accepted)
	}
	p.Warnings = []string{
		"This profile labels sounds from one decoded player and arrangement; source IDs are local to its instrument bank.",
		"Chronological validation still contains recurring music from the same song; cross-song accuracy has not been established.",
		"The source labels do not make these definitions equivalent to native MaxYMiser instruments.",
	}
	return p, nil
}

func featureDistance(a, b []int16) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return math.Inf(1)
	}
	sum := 0.0
	for i := range a {
		weight := 1.0
		if i%5 == 2 {
			weight = 16
		}
		d := float64(a[i]) - float64(b[i])
		sum += d * d * weight
	}
	return math.Sqrt(sum / float64(len(a)))
}

// MatchSource abstains when an unrelated or equally plausible sound is found.
// Distance and runner-up margin are measurements, not calibrated probabilities.
func (p PairedProfile) MatchSource(features []int16) (int, float64, float64, bool) {
	distances := map[int]float64{}
	for _, prototype := range p.Prototypes {
		d := featureDistance(features, prototype.Features)
		previous, ok := distances[prototype.Instrument]
		if !ok || d < previous {
			distances[prototype.Instrument] = d
		}
	}
	best, second, id := math.Inf(1), math.Inf(1), -1
	ids := make([]int, 0, len(distances))
	for i := range distances {
		ids = append(ids, i)
	}
	sort.Ints(ids)
	for _, i := range ids {
		d := distances[i]
		if d < best {
			second, best, id = best, d, i
		} else if d < second {
			second = d
		}
	}
	margin := second - best
	if math.IsInf(second, 1) {
		margin = 100
	}
	return id, best, margin, id >= 0 && best <= 3 && margin >= 0.15
}

// SourceEvidence applies learned labels to independently detected YM events.
// The native score is not consulted to assign event IDs in the target trace.
func (p PairedProfile) SourceEvidence(trace Trace) []SourceEvidence {
	var results []SourceEvidence
	for ch := 0; ch < 3; ch++ {
		for _, e := range ExtractEvents(trace, ch) {
			id, distance, margin, ok := p.MatchSource(e.Features)
			if ok {
				results = append(results, SourceEvidence{ch, e.Start, e.End, id, distance, margin})
			}
		}
	}
	return results
}

func SavePairedProfile(profile PairedProfile, path string) error {
	b, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

func LoadPairedProfile(path string) (PairedProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PairedProfile{}, err
	}
	var p PairedProfile
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	if p.Version != 1 || len(p.Prototypes) == 0 || len(p.Prototypes) > 100000 || p.Source.Rate < 1 || p.Source.Rate > 2000 || len(p.Source.Instruments) > 256 {
		return p, fmt.Errorf("pair: unsupported or empty labelled profile")
	}
	for _, prototype := range p.Prototypes {
		if prototype.Instrument < 0 || prototype.Instrument >= len(p.Source.Instruments) || len(prototype.Features) != 80 {
			return p, fmt.Errorf("pair: invalid instrument label or feature dimensions")
		}
	}
	if len(p.Recipes) > 100000 {
		return p, fmt.Errorf("pair: too many instrument recipes")
	}
	if len(p.Patterns) > 4096 {
		return p, fmt.Errorf("pair: too many source pattern prototypes")
	}
	for _, pattern := range p.Patterns {
		if pattern.Pattern < 0 || pattern.Pattern > 255 || pattern.Frames < 12 || pattern.Frames > 2048 || len(pattern.Features) != 320 {
			return p, fmt.Errorf("pair: invalid source pattern prototype")
		}
	}
	if len(p.ReferencePatterns) > 100000 {
		return p, fmt.Errorf("pair: too many reference pattern passages")
	}
	for _, pattern := range p.ReferencePatterns {
		if pattern.Channel < 0 || pattern.Channel >= 3 || pattern.Start < 0 || pattern.End <= pattern.Start || len(pattern.Patterns) != 1 || pattern.Patterns[0] < 0 || pattern.Patterns[0] > 255 {
			return p, fmt.Errorf("pair: invalid reference pattern passage")
		}
	}
	for _, recipe := range p.Recipes {
		if recipe.SourceInstrument < 0 || recipe.SourceInstrument >= len(p.Source.Instruments) || recipe.Frames < 1 || recipe.Frames > 63 || recipe.Note < 2 || recipe.Note > 127 || len(recipe.Features) != 80 || math.IsNaN(recipe.RegisterError) || math.IsInf(recipe.RegisterError, 0) || recipe.RegisterError < 0 {
			return p, fmt.Errorf("pair: invalid instrument recipe")
		}
		for _, s := range recipe.Sequences {
			if s.Length < 1 || s.Length > 63 || s.Repeat >= s.Length {
				return p, fmt.Errorf("pair: invalid instrument recipe sequence")
			}
		}
	}
	return p, nil
}
