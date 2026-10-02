package ymimport

import (
	"fmt"
	"sort"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type SourceProjectReport struct {
	Player                 string           `json:"player"`
	StartFrame             int              `json:"start_frame"`
	EndFrame               int              `json:"end_frame"`
	Events                 int              `json:"events"`
	Patterns               int              `json:"generated_patterns"`
	Positions              int              `json:"positions"`
	Bank                   SourceBankReport `json:"bank"`
	UnsupportedEvents      int              `json:"unsupported_instrument_events"`
	ModulationSegments     int              `json:"pitch_modulation_segments"`
	ScoreVolumeInstruments []int            `json:"pattern_volume_instruments,omitempty"`
	ScoreVolumeChanges     int              `json:"pattern_volume_changes,omitempty"`
	ScoreMixerInstruments  []int            `json:"pattern_mixer_instruments,omitempty"`
	ScoreMixerChanges      int              `json:"pattern_mixer_changes,omitempty"`
	UntranslatedCommands   map[byte]int     `json:"untranslated_pattern_commands"`
	Warnings               []string         `json:"warnings"`
}

// SourceProject places verified source notes on a one-frame editable grid.
// Source pattern identifiers remain separate from generated 64-row patterns.
// Unsupported instruments stay silent and are reported, never synthesized as
// invented replacement sounds. The selected range is an excerpt, not a proof
// of the original arrangement's loop boundaries or complete replay fidelity.
func SourceProject(score SourceScore, start, end int) (*model.Project, SourceProjectReport, error) {
	report := SourceProjectReport{Player: score.Player, UntranslatedCommands: map[byte]int{}}
	if end == 0 {
		end = score.Frames
	}
	report.StartFrame, report.EndFrame = start, end
	if score.Rate < 25 || score.Rate > 200 || start < 0 || end <= start || end > score.Frames {
		return nil, report, fmt.Errorf("source: invalid editable range or frame rate")
	}
	positions := (end - start + model.Rows - 1) / model.Rows
	if positions > 255 {
		return nil, report, fmt.Errorf("source: selected excerpt exceeds 255 arrangement positions")
	}
	bankScore, mixerCandidates := prepareClassicMixerScore(score)
	bank, bankReport, volumeInstruments, err := prepareSourceProjectBank(bankScore)
	if err != nil {
		return nil, report, err
	}
	report.Bank = bankReport
	report.ScoreVolumeInstruments = volumeInstruments
	for id := range bankReport.Unsupported {
		bank.Instruments[id].SetName(fmt.Sprintf("Source %02X ?", id))
	}
	for _, pattern := range score.Patterns {
		for _, command := range pattern.Commands {
			op := command.Opcode
			if op >= 128 && op < 0xb8 && op != 0x80 && op != 0x87 && op != 0x8e && (op != 0x90 || score.Player == madMaxClassic) && op != 0x91 && (op != 0x89 || score.Player != madMaxClassic) {
				report.UntranslatedCommands[op]++
			}
		}
	}
	rows := make([][3]model.Cell, end-start)
	var previous [3]*SourceEvent
	lastFrame := [3]int{-1, -1, -1}
	for i := range score.Events {
		event := &score.Events[i]
		if event.Channel < 0 || event.Channel >= 3 || event.Frame < 0 || event.Frame >= score.Frames || event.Frame <= lastFrame[event.Channel] {
			return nil, report, fmt.Errorf("source: invalid event channel, frame or ordering")
		}
		lastFrame[event.Channel] = event.Frame
		if !event.Rest && (event.Instrument < 0 || event.Instrument >= len(score.Instruments) || score.Instruments[event.Instrument].ID != event.Instrument) {
			return nil, report, fmt.Errorf("source: event references an invalid note or instrument")
		}
		if event.Frame < start {
			previous[event.Channel] = event
			continue
		}
		if event.Frame >= end {
			continue
		}
		if !event.Rest && (event.Note < 2 || event.Note > 127) {
			return nil, report, fmt.Errorf("source: pitch %d at frame %d has no verified tracker mapping; choose a range without it", event.Note, event.Frame)
		}
		cell := &rows[event.Frame-start][event.Channel]
		if event.Rest {
			cell.Note = model.NoteOff
		} else {
			cell.Note = byte(event.Note)
			if event.Retrigger || previous[event.Channel] == nil || previous[event.Channel].Rest || previous[event.Channel].Instrument != event.Instrument || event.Frame == start {
				cell.Instrument = byte(event.Instrument + 1)
			}
			if _, unsupported := bankReport.Unsupported[event.Instrument]; unsupported {
				report.UnsupportedEvents++
			}
		}
		previous[event.Channel] = event
		report.Events++
	}
	if start > 0 {
		// Locate the sounding note at the selection boundary independently
		// of the last event later encountered within the selected excerpt.
		var held [3]*SourceEvent
		for i := range score.Events {
			e := &score.Events[i]
			if e.Frame < start {
				held[e.Channel] = e
			}
		}
		for channel, e := range held {
			if e != nil && !e.Rest && rows[0][channel].Note == 0 {
				if e.Note < 2 || e.Note > 127 {
					return nil, report, fmt.Errorf("source: the pitch sounding on channel %d at the excerpt start has no verified tracker mapping; choose another start frame", channel)
				}
				rows[0][channel] = model.Cell{Note: byte(e.Note), Instrument: byte(e.Instrument + 1)}
				if _, unsupported := bankReport.Unsupported[e.Instrument]; unsupported {
					report.UnsupportedEvents++
				}
			}
		}
		report.Warnings = append(report.Warnings, "The excerpt's initial sounding notes restart their envelopes; the original earlier modulation phase is not restored.")
	}
	volumeChanges, err := applySourceVolume(score, rows, start, volumeInstruments)
	if err != nil {
		return nil, report, err
	}
	report.ScoreVolumeChanges = volumeChanges
	if len(volumeInstruments) > 0 {
		report.Warnings = append(report.Warnings, "Long ordinary-tone envelopes use exact pattern volume levels. These sound definitions need their generated score to retain envelope timing; standalone MYV preview holds a constant level.")
	}
	segments, err := applySourceModulation(score, rows, &bank, start)
	if err != nil {
		return nil, report, err
	}
	report.ModulationSegments = segments
	mixerIDs, mixerChanges, err := applyClassicMixer(score, rows, &bank, start, mixerCandidates)
	if err != nil {
		return nil, report, err
	}
	report.ScoreMixerInstruments, report.ScoreMixerChanges = mixerIDs, mixerChanges
	if len(mixerIDs) > 0 {
		report.Warnings = append(report.Warnings, "Verified classic fixed-pitch sounds use generated M/N commands for their alternating mixer and shared noise period. Their bank definitions need this score for original timbre.")
	}
	if segments > 0 {
		delete(report.UntranslatedCommands, 0x81)
		delete(report.UntranslatedCommands, 0x82)
		delete(report.UntranslatedCommands, 0x84)
		report.Warnings = append(report.Warnings, "Native vibrato/slide timing is translated for ordinary tone programs with constant zero arpeggio. Other pitch programs remain outside this conversion.")
	}
	// A partial final native pattern ends at the exact selected frame instead
	// of extending the excerpt with unrequested empty rows.
	if len(rows)%model.Rows != 0 {
		inserted := false
		for channel := range 3 {
			cell := &rows[len(rows)-1][channel]
			if cell.Effect1 == 0 {
				cell.Effect1 = 'B'
				inserted = true
				break
			}
			if cell.Effect2 == 0 {
				cell.Effect2 = 'B'
				inserted = true
				break
			}
		}
		if !inserted {
			return nil, report, fmt.Errorf("source: excerpt boundary needs a free effect column; choose another end frame")
		}
	}
	p := model.New()
	p.Title = "Source score excerpt"
	p.Bank = bank
	p.Song.Patterns = nil
	p.Song.SetSpeed(1)
	p.Song.SetTickRate(score.Rate)
	p.Song.State[49] = 0
	p.Song.Length, p.Song.Repeat = byte(positions), 0
	patterns := map[model.Pattern]byte{}
	for position := 0; position < positions; position++ {
		order := [4]byte{255, 255, 255, 255}
		for channel := 0; channel < 3; channel++ {
			var pattern model.Pattern
			for row := 0; row < model.Rows && position*model.Rows+row < len(rows); row++ {
				pattern[row] = rows[position*model.Rows+row][channel]
			}
			id, exists := patterns[pattern]
			if !exists {
				if len(p.Song.Patterns) == model.MaxPatterns {
					return nil, report, fmt.Errorf("source: selected excerpt exceeds 240 distinct native patterns")
				}
				id = byte(len(p.Song.Patterns))
				patterns[pattern] = id
				p.Song.Patterns = append(p.Song.Patterns, pattern)
			}
			order[channel] = id
		}
		p.Song.Orders[position] = order
	}
	report.Patterns, report.Positions = len(p.Song.Patterns), positions
	report.Warnings = append(report.Warnings, "Verified source note timing is retained on a one-frame grid. Generated 64-row patterns are not original source pattern identifiers.")
	report.Warnings = append(report.Warnings, bankReport.Warnings...)
	if report.UnsupportedEvents > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d note events use unsupported source instruments, whose definitions remain silent.", report.UnsupportedEvents))
	}
	if len(report.UntranslatedCommands) > 0 {
		var commands []int
		for command := range report.UntranslatedCommands {
			commands = append(commands, int(command))
		}
		sort.Ints(commands)
		report.Warnings = append(report.Warnings, fmt.Sprintf("Source pattern commands %X have not been translated into editable effects.", commands))
	}
	return p, report, nil
}
