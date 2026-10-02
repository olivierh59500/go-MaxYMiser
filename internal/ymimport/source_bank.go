package ymimport

import (
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type SourceBankReport struct {
	Converted   []int          `json:"converted_source_instruments"`
	Unsupported map[int]string `json:"unsupported_source_instruments"`
	Warnings    []string       `json:"warnings"`
}

// SourceVoiceBank translates known envelope, arpeggio and noise-attack behavior
// to native editable sequences. Unsupported synthesis is explicit, not replaced
// by a generic sound. The source ID maps to MaxYMiser's one-based ID+1.
func SourceVoiceBank(score SourceScore) (model.VoiceBank, SourceBankReport, error) {
	bank := model.VoiceBank{Version: 1, SampleVersion: 1, SequenceCount: 1}
	report := SourceBankReport{Unsupported: map[int]string{}, Warnings: []string{"Converted definitions retain volume, arpeggio and validated noise-attack timing. Pattern-controlled vibrato, slides, other hardware programs and pitch-table rounding are not part of this bank conversion."}}
	sequenceIDs := map[model.Sequence]byte{}
	add := func(sequence model.Sequence) (byte, error) {
		if id, ok := sequenceIDs[sequence]; ok {
			return id, nil
		}
		if bank.SequenceCount == model.MaxSequences {
			return 0, fmt.Errorf("source: converted bank exceeds native sequence capacity")
		}
		id := byte(bank.SequenceCount)
		bank.Sequences[id] = sequence
		bank.SequenceCount++
		sequenceIDs[sequence] = id
		return id, nil
	}
	for _, source := range score.Instruments {
		if source.ID < 0 || source.ID >= model.MaxInstruments || len(source.Settings) != 6 {
			return bank, report, fmt.Errorf("source: invalid instrument definition")
		}
		var reason string
		switch {
		case source.Settings[0]&^0x1c != 0:
			reason = "native hardware-effect flags are not yet translated"
		case (source.Settings[0]&0x1c != 0 || len(source.NoiseProgram) > 0) && len(source.NoiseProgram) < 2:
			reason = "native noise-program data is missing"
		case source.Settings[1] >= 128:
			reason = "extended native pitch/noise program is not yet translated"
		case len(source.VolumeSequence) == 0:
			reason = "empty native volume sequence"
		case len(source.Arpeggio.Values) == 0:
			reason = "native arpeggio data is missing"
		}
		if reason != "" {
			report.Unsupported[source.ID] = reason
			continue
		}
		volume := model.Sequence{}
		for n, value := range source.VolumeSequence {
			count := int(source.Settings[5]) + 1
			if n == 0 {
				count = int(source.Settings[5])
				if len(source.VolumeSequence) == 1 {
					count = max(1, count)
				}
			}
			if value > 15 || int(volume.Length)+count > len(volume.Values) {
				reason = "volume program exceeds the native editable sequence range"
				break
			}
			// Native trigger preloads the first byte before decrementing its
			// counter. The first value lasts N calls; subsequent ones last N+1.
			for repeat := 0; repeat < count; repeat++ {
				volume.Values[volume.Length] = uint16(value)
				volume.Length++
			}
		}
		volume.Repeat = volume.Length - 1
		arpeggio := model.Sequence{}
		arp := source.Arpeggio
		if arp.StepFrames < 1 || arp.Repeat < 0 || arp.Repeat >= len(arp.Values) || len(arp.Values)*arp.StepFrames > len(arpeggio.Values) {
			reason = "arpeggio timing exceeds the native editable sequence range"
		} else {
			for _, value := range arp.Values {
				if value < -32768 || value > 32767 {
					return bank, report, fmt.Errorf("source: invalid signed arpeggio value")
				}
				for repeat := 0; repeat < arp.StepFrames; repeat++ {
					arpeggio.Values[arpeggio.Length] = uint16(int16(value))
					arpeggio.Length++
				}
			}
			arpeggio.Repeat = byte(arp.Repeat * arp.StepFrames)
			if arp.Repeat == len(arp.Values)-1 {
				arpeggio.Repeat = arpeggio.Length - 1
			}
		}
		if reason != "" {
			report.Unsupported[source.ID] = reason
			continue
		}
		mixer, noise := model.Sequence{Values: [63]uint16{0x100}, Length: 1}, model.Sequence{Length: 1}
		if len(source.NoiseProgram) > 0 {
			steps := source.NoiseProgram[1:]
			if len(steps) > len(mixer.Values) {
				report.Unsupported[source.ID] = "noise program exceeds native sequence capacity"
				continue
			}
			mixer.Length, noise.Length = byte(len(steps)), byte(len(steps))
			mixer.Repeat, noise.Repeat = byte(len(steps)-1), byte(len(steps)-1)
			for n, step := range steps {
				switch step.Mode {
				case 1:
					mixer.Values[n] = 0x1000
				case 2:
					mixer.Values[n] = 0x1100
				case 3:
					mixer.Values[n] = 0x0100
				default:
					return bank, report, fmt.Errorf("source: invalid noise mixer operation")
				}
				noise.Values[n] = uint16(step.Period & 31)
			}
		}
		sequences := []model.Sequence{volume, arpeggio, mixer, noise}
		inst := &bank.Instruments[source.ID]
		inst.SetName(fmt.Sprintf("Source %02X", source.ID))
		inst[17], inst[19], inst[32] = 4, 4, 1
		for n, offset := range []int{48, 49, 51, 52} {
			sequence := sequences[n]
			id, err := add(sequence)
			if err != nil {
				return bank, report, err
			}
			inst[offset] = id
		}
		report.Converted = append(report.Converted, source.ID)
	}
	return bank, report, nil
}
