package ymimport

import (
	"fmt"
	"math"
	"sort"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

// InstrumentRecipe is an editable synthesis candidate learned from labelled
// register output. It carries its source identity and measured replay error.
type InstrumentRecipe struct {
	SourceInstrument int               `json:"source_instrument"`
	SourceFrame      int               `json:"training_source_frame"`
	TrainingSource   string            `json:"training_source_identity,omitempty"`
	Note             int               `json:"training_midi_note"`
	Frames           int               `json:"training_frames"`
	Features         []int16           `json:"training_features"`
	Instrument       model.Instrument  `json:"instrument"`
	Sequences        [5]model.Sequence `json:"volume_arpeggio_vibrato_mixer_noise"`
	RegisterError    float64           `json:"mean_register_error"`
}

func learnRecipes(score SourceScore, trace Trace, alignment PairAlignment, trainingEnd int) []InstrumentRecipe {
	byProgram := map[string]InstrumentRecipe{}
	for _, f := range sourceFeatures(score, trace, alignment) {
		e := f.event
		if e.Frame >= trainingEnd || f.end > trainingEnd+alignment.Offset {
			continue
		}
		start := f.start
		// Limit recipes to the actual source event boundary, rather than a
		// guessed fixed envelope duration. Every sequence is one frame per word.
		end := min(start+63, f.end)
		if end-start < 3 {
			continue
		}
		recipe := InstrumentRecipe{SourceInstrument: e.Instrument, SourceFrame: e.Frame, Note: e.Note + alignment.Transpose, Frames: end - start, Features: eventFeatures(trace, e.Channel, start, end)}
		recipe.Instrument.SetName(fmt.Sprintf("Source %02X", e.Instrument))
		recipe.Instrument[17], recipe.Instrument[18], recipe.Instrument[19], recipe.Instrument[32] = 4, 4, 4, 1
		// Envelope/timer instruments need their own period/retrigger model.
		valid := true
		basePeriod := 0
		for at := start; at < end; at++ {
			r := trace.Frames[at]
			if r[8+e.Channel]&16 != 0 {
				valid = false
				break
			}
			i := at - start
			tone := r[7]&(1<<e.Channel) == 0
			period := int(r[2*e.Channel]) | int(r[2*e.Channel+1]&15)<<8
			mix := uint16(0)
			if tone {
				mix |= 0x100
				if period == 0 {
					valid = false
					break
				}
				if basePeriod == 0 {
					basePeriod = period
				}
				arp := int(math.Round(12 * math.Log2(float64(basePeriod)/float64(period))))
				// Learn the pitch trajectory relative to the source note. A
				// vibrato word corrects the native MaxYMiser period quantization.
				predicted := int(replay.TonePeriod(recipe.Note + arp))
				recipe.Sequences[1].Values[i] = uint16(int16(arp))
				recipe.Sequences[2].Values[i] = uint16(int16(predicted - period))
			}
			if r[7]&(1<<(e.Channel+3)) == 0 {
				mix |= 0x1000
			}
			recipe.Sequences[0].Values[i] = uint16(r[8+e.Channel] & 15)
			recipe.Sequences[3].Values[i] = mix
			recipe.Sequences[4].Values[i] = uint16(r[6] & 31)
		}
		if !valid {
			continue
		}
		for i := range recipe.Sequences {
			recipe.Sequences[i].Length = byte(recipe.Frames)
			recipe.Sequences[i].Repeat = byte(recipe.Frames - 1)
		}
		recipe.RegisterError = recipeError(recipe, trace, e.Channel, start, end)
		if recipe.RegisterError > 0.5 {
			continue
		}
		key := fmt.Sprint(e.Instrument, ":", recipe.Frames, ":", recipe.Sequences)
		old, exists := byProgram[key]
		if !exists || recipe.RegisterError < old.RegisterError {
			byProgram[key] = recipe
		}
	}
	var recipes []InstrumentRecipe
	for _, r := range byProgram {
		recipes = append(recipes, r)
	}
	sort.Slice(recipes, func(i, j int) bool {
		if recipes[i].SourceInstrument != recipes[j].SourceInstrument {
			return recipes[i].SourceInstrument < recipes[j].SourceInstrument
		}
		return recipes[i].SourceFrame < recipes[j].SourceFrame
	})
	return recipes
}

func recipeError(recipe InstrumentRecipe, trace Trace, ch, start, end int) float64 {
	p := model.New()
	p.Bank.Instruments[0] = recipe.Instrument
	for i, off := range []int{48, 49, 50, 51, 52} {
		p.Bank.Sequences[i+1] = recipe.Sequences[i]
		p.Bank.Instruments[0][off] = byte(i + 1)
	}
	e := replay.New(p)
	e.Trigger(ch, byte(recipe.Note), 1)
	errorSum, values := 0.0, 0
	for at := start; at < end; at++ {
		e.Tick()
		r := trace.Frames[at]
		if r[8+ch] != 0 && r[7]&(1<<ch) == 0 {
			wanted := int(r[ch*2]) | int(r[ch*2+1]&15)<<8
			actual := int(e.Registers[ch*2]) | int(e.Registers[ch*2+1]&15)<<8
			errorSum += math.Abs(float64(wanted - actual))
			values++
		}
		errorSum += math.Abs(float64(int(r[8+ch]) - int(e.Registers[8+ch])))
		values++
		wantedMix := (r[7] >> ch) & 9
		actualMix := (e.Registers[7] >> ch) & 9
		if wantedMix != actualMix {
			errorSum += 16
		}
		values++
	}
	return errorSum / float64(max(1, values))
}
