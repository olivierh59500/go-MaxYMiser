package ymimport

import (
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"math"
)

type Report struct {
	AuthorProfile                            string
	Evidence                                 []Evidence
	Frames, Instruments, Patterns, Positions int
	Warnings                                 []string
	StartFrame, EndFrame, FramesPerRow       int
	GridCandidates                           []GridCandidate
	SourceLabels                             []SourceEvidence
	SourcePlayer                             string
	SourceCorpusGroups                       []string
	SourceLabelRate                          int
	RecipeApplications                       []RecipeApplication
	SourcePatterns                           []PatternEvidence
	KnownSourcePair                          bool
	ExactTonePeriods                         bool
}
type timbre struct {
	Tone, Noise, Envelope bool
	NoisePeriod, Shape    byte
}

// Reconstruct creates a candidate score. Register traces do not contain the
// source tracker's instrument names, pattern boundaries or effect assignments.
func Reconstruct(trace Trace) (*model.Project, Report, error) {
	p := model.New()
	p.Title = trace.Name
	if p.Title == "" {
		p.Title = "YM reconstruction"
	}
	p.Author = trace.Author
	p.Song.Patterns = nil
	p.Song.SetSpeed(1)
	p.Song.SetTickRate(trace.Rate)
	p.Bank = model.VoiceBank{Version: 1, SampleVersion: 1, SequenceCount: 1}
	report := Report{Frames: len(trace.Frames)}
	if trace.Rate < 25 || trace.Rate > 200 {
		return nil, report, fmt.Errorf("ymimport: native tracker rate is limited to 25–200 Hz")
	}
	positions := (len(trace.Frames) + 63) / 64
	if positions > 255 {
		return nil, report, fmt.Errorf("ymimport: recording exceeds the 255-position reconstruction limit")
	}
	p.Song.Length = byte(positions)
	p.Song.Repeat = 0
	instruments := map[timbre]byte{}
	patterns := map[model.Pattern]byte{}
	sequence := 1
	var previousNote [3]byte
	var previousInstrument [3]byte
	var active [3]bool
	for position := 0; position < positions; position++ {
		order := [4]byte{255, 255, 255, 255}
		for ch := 0; ch < 3; ch++ {
			var pattern model.Pattern
			for row := 0; row < 64; row++ {
				at := position*64 + row
				if at >= len(trace.Frames) {
					break
				}
				regs := trace.Frames[at]
				volume := regs[8+ch] & 31
				tone, noise := regs[7]&(1<<ch) == 0, regs[7]&(1<<(ch+3)) == 0
				on := volume != 0 && (tone || noise || volume&16 != 0)
				if !on {
					if active[ch] {
						pattern[row].Note = 1
					}
					active[ch] = false
					continue
				}
				color := timbre{Tone: tone, Noise: noise, Envelope: volume&16 != 0}
				if noise {
					color.NoisePeriod = regs[6] & 31
				}
				if color.Envelope {
					color.Shape = regs[13] & 15
				}
				instrument, exists := instruments[color]
				if !exists {
					if len(instruments) >= 32 || sequence+3 >= 256 {
						return nil, report, fmt.Errorf("ymimport: more than 32 distinct timbres; reduce the selection before reconstruction")
					}
					instrument = byte(len(instruments) + 1)
					instruments[color] = instrument
					inst := &p.Bank.Instruments[instrument-1]
					inst.SetName(fmt.Sprintf("YM %02d", instrument))
					inst[17], inst[18], inst[19] = 7, 7, 7
					inst[32] = 1
					inst[34] = color.Shape
					mix := uint16(0)
					if tone {
						mix |= 0x0100
					}
					if noise {
						mix |= 0x1000
					}
					if color.Envelope {
						mix |= 0x0010
						report.Warnings = appendUnique(report.Warnings, "Hardware envelope periods and retrigger timing are approximate in the proposed score.")
					}
					p.Bank.Sequences[sequence] = model.Sequence{Values: [63]uint16{15}, Length: 1}
					inst[48] = byte(sequence)
					sequence++
					p.Bank.Sequences[sequence] = model.Sequence{Values: [63]uint16{mix}, Length: 1}
					inst[51] = byte(sequence)
					sequence++
					p.Bank.Sequences[sequence] = model.Sequence{Values: [63]uint16{uint16(color.NoisePeriod)}, Length: 1}
					inst[52] = byte(sequence)
					sequence++
				}
				period := int(regs[ch*2]) | (int(regs[ch*2+1]&15) << 8)
				note := byte(60)
				if tone && period > 0 {
					freq := float64(trace.Clock) / 16 / float64(period)
					n := int(math.Round(69 + 12*math.Log2(freq/440)))
					note = byte(max(2, min(127, n)))
				}
				cell := &pattern[row]
				if !active[ch] || note != previousNote[ch] || instrument != previousInstrument[ch] {
					cell.Note = note
					cell.Instrument = instrument
				}
				attenuation := byte(0)
				if volume&16 == 0 {
					attenuation = 15 - (volume & 15)
				}
				cell.Volume = attenuation
				if cell.Volume == 0 {
					cell.Volume = 16
				}
				previousNote[ch], previousInstrument[ch], active[ch] = note, instrument, true
			}
			id, exists := patterns[pattern]
			if !exists {
				if len(p.Song.Patterns) >= 240 {
					return nil, report, fmt.Errorf("ymimport: more than 240 distinct patterns")
				}
				id = byte(len(p.Song.Patterns))
				patterns[pattern] = id
				p.Song.Patterns = append(p.Song.Patterns, pattern)
			}
			order[ch] = id
		}
		p.Song.Orders[position] = order
	}
	p.Bank.SequenceCount = sequence
	report.Instruments = len(instruments)
	report.Patterns = len(patterns)
	report.Positions = positions
	report.Warnings = appendUnique(report.Warnings, "Notes and repeated 64-frame patterns are inferred; they are not the original tracker structure.")
	if trace.Effects {
		report.Warnings = appendUnique(report.Warnings, "DigiDrums and timer effects remain in the YM reference and are not recovered as source instruments.")
	}
	if exact, err := preserveToneCurves(p, trace); err == nil {
		p, report.ExactTonePeriods = exact, true
		report.Patterns = len(p.Song.Patterns)
	} else {
		report.Warnings = appendUnique(report.Warnings, "Exact tone-period curves were not retained: "+err.Error())
	}
	return p, report, nil
}
func appendUnique(values []string, value string) []string {
	for _, s := range values {
		if s == value {
			return values
		}
	}
	return append(values, value)
}
