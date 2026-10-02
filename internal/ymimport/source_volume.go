package ymimport

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// prepareSourceProjectBank retains standalone bank conversion for short
// envelopes. Long ordinary-tone envelopes use absolute pattern volume instead;
// the definition holds level 15 so column attenuation can encode levels 0–15.
func prepareSourceProjectBank(score SourceScore) (model.VoiceBank, SourceBankReport, []int, error) {
	bank, report, err := SourceVoiceBank(score)
	if err != nil {
		return bank, report, nil, err
	}
	var ids []int
	for _, source := range score.Instruments {
		if report.Unsupported[source.ID] != sourceVolumeRangeReason || source.Settings[0] != 0 {
			continue
		}
		valid := len(source.VolumeSequence) > 0
		for _, value := range source.VolumeSequence {
			valid = valid && value <= 15
		}
		if valid {
			ids = append(ids, source.ID)
		}
	}
	if len(ids) == 0 {
		return bank, report, nil, nil
	}
	editable := score
	editable.Instruments = append([]SourceInstrument(nil), score.Instruments...)
	for index := range editable.Instruments {
		definition := &editable.Instruments[index]
		matched := false
		for _, id := range ids {
			matched = matched || definition.ID == id
		}
		if !matched {
			continue
		}
		definition.Settings = append([]byte(nil), definition.Settings...)
		definition.Settings[5] = 0
		definition.VolumeSequence = []byte{15}
	}
	bank, report, err = SourceVoiceBank(editable)
	if err != nil {
		return bank, report, nil, err
	}
	recovered := ids[:0]
	for _, id := range ids {
		if _, unsupported := report.Unsupported[id]; !unsupported {
			recovered = append(recovered, id)
		}
	}
	return bank, report, recovered, nil
}

type sourceVolumeState struct {
	instrument, envelope, index, counter int
	value                                byte
	active, fade                         bool
	fadeRate, fadeCounter                byte
}

// sourceVolumeFrames follows the original independent envelope counters. A
// legato note reloads cadence without rewinding the current volume pointer.
// Classic 90 pauses that pointer and applies its separate timed decay.
func sourceVolumeFrames(score SourceScore) ([][3]byte, error) {
	if score.Frames < 1 || score.Frames > 1000000 {
		return nil, fmt.Errorf("source: invalid volume timeline length")
	}
	events := make([][]SourceEvent, score.Frames)
	controls := make([][]SourceControl, score.Frames)
	for _, event := range score.Events {
		if event.Channel < 0 || event.Channel >= 3 || event.Frame < 0 || event.Frame >= score.Frames {
			return nil, fmt.Errorf("source: invalid volume note position")
		}
		events[event.Frame] = append(events[event.Frame], event)
	}
	for _, control := range score.Controls {
		if control.Channel < 0 || control.Channel >= 3 || control.Frame < 0 || control.Frame >= score.Frames {
			return nil, fmt.Errorf("source: invalid volume control position")
		}
		controls[control.Frame] = append(controls[control.Frame], control)
	}
	values := make([][3]byte, score.Frames)
	state := [3]sourceVolumeState{{instrument: -1, envelope: -1}, {instrument: -1, envelope: -1}, {instrument: -1, envelope: -1}}
	for frame := range values {
		for _, control := range controls[frame] {
			voice := &state[control.Channel]
			switch {
			case control.Opcode >= 0xc0 && control.Opcode < 0xe0:
				voice.instrument = int(control.Opcode - 0xc0)
				if voice.instrument >= len(score.Instruments) || len(score.Instruments[voice.instrument].Settings) != 6 {
					return nil, fmt.Errorf("source: volume control references an invalid instrument")
				}
			case score.Player == madMaxClassic && control.Opcode == 0x90:
				if len(control.Operand) != 1 {
					return nil, fmt.Errorf("source: truncated timed volume decay")
				}
				voice.active, voice.fade = false, true
				voice.fadeRate, voice.fadeCounter = control.Operand[0], control.Operand[0]
			}
		}
		for _, event := range events[frame] {
			voice := &state[event.Channel]
			if event.Rest {
				voice.value, voice.active, voice.fade = 0, false, false
				continue
			}
			if event.Instrument < 0 || event.Instrument >= len(score.Instruments) {
				return nil, fmt.Errorf("source: volume note references an invalid instrument")
			}
			definition := score.Instruments[event.Instrument]
			if len(definition.Settings) != 6 || len(definition.VolumeSequence) == 0 {
				return nil, fmt.Errorf("source: missing volume definition")
			}
			voice.instrument = event.Instrument
			if event.Retrigger || voice.envelope < 0 {
				voice.envelope, voice.index = event.Instrument, 0
			}
			voice.counter = int(definition.Settings[5])
			voice.value = score.Instruments[voice.envelope].VolumeSequence[voice.index]
			voice.active, voice.fade = true, false
		}
		for channel := range state {
			voice := &state[channel]
			if voice.active && voice.envelope >= 0 {
				voice.counter--
				if voice.counter < 0 {
					voice.counter = int(score.Instruments[voice.instrument].Settings[5])
					sequence := score.Instruments[voice.envelope].VolumeSequence
					voice.index = min(voice.index+1, len(sequence)-1)
					voice.value = sequence[voice.index]
				}
			}
			if voice.fade {
				previous := voice.fadeCounter
				voice.fadeCounter--
				if previous == 0 {
					voice.fadeCounter = voice.fadeRate
					if voice.value == 0 {
						voice.fade = false
					} else {
						voice.value--
					}
				}
			}
			values[frame][channel] = voice.value
		}
	}
	return values, nil
}

func applySourceVolume(score SourceScore, rows [][3]model.Cell, start int, recovered []int) (int, error) {
	if len(recovered) == 0 {
		return 0, nil
	}
	values, err := sourceVolumeFrames(score)
	if err != nil {
		return 0, err
	}
	ids := map[int]bool{}
	for _, id := range recovered {
		ids[id] = true
	}
	events := make([][]SourceEvent, score.Frames)
	for _, event := range score.Events {
		events[event.Frame] = append(events[event.Frame], event)
	}
	active := [3]int{-1, -1, -1}
	previous := [3]int{-1, -1, -1}
	changes := 0
	for frame := 0; frame < start+len(rows); frame++ {
		for _, event := range events[frame] {
			if event.Rest {
				active[event.Channel] = -1
			} else {
				active[event.Channel] = event.Instrument
			}
		}
		if frame < start {
			continue
		}
		for channel, id := range active {
			if !ids[id] {
				previous[channel] = -1
				continue
			}
			value := int(values[frame][channel])
			if value > 15 {
				return 0, fmt.Errorf("source: volume program has an invalid chip level")
			}
			cell := &rows[frame-start][channel]
			if value != previous[channel] || cell.Instrument != 0 {
				cell.Volume = byte(15 - value)
				if cell.Volume == 0 {
					cell.Volume = 16
				}
				changes++
			}
			previous[channel] = value
		}
	}
	return changes, nil
}
