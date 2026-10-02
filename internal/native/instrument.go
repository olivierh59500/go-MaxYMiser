package native

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

// InstrumentFile stores the definitions carried by MYI independently of their
// sequence IDs in a voice bank. Versions 0/1 use 31 words and seven sequences;
// version 2 uses 63 words and seven sequences; version 3 adds the PWM sequence.
type InstrumentFile struct {
	Version    byte
	Instrument model.Instrument
	Sequences  [8]model.Sequence
	Sample     []byte
	Trailing   []byte
}

// Legacy MYI0 samples contain four-bit DAC levels and a negative end marker.
// This conversion matches the original editor's inverse DAC table.
var legacyDACPCM = model.DACPCM

func DecodeInstrument(data []byte) (InstrumentFile, error) {
	var file InstrumentFile
	var err error
	data, err = UnpackICE(data)
	if err != nil {
		return file, err
	}
	if len(data) < 56 || !bytes.Equal(data[:3], []byte("MYM")) || !bytes.Equal(data[4:8], []byte(".MYI")) || data[3] < '0' || data[3] > '3' {
		return file, fmt.Errorf("native: invalid MYI header")
	}
	file.Version = data[3] - '0'
	copy(file.Instrument[:48], data[8:56])
	stride, count := 64, 7
	if file.Version >= 2 {
		stride = 128
	}
	if file.Version == 3 {
		count = 8
	}
	end := 56 + count*stride
	if len(data) < end {
		return file, fmt.Errorf("native: truncated MYI sequences")
	}
	for i := 0; i < count; i++ {
		record := data[56+i*stride : 56+(i+1)*stride]
		sequence := &file.Sequences[i]
		for n := 0; n < stride/2-1; n++ {
			sequence.Values[n] = binary.BigEndian.Uint16(record[n*2:])
		}
		sequence.Length, sequence.Repeat = record[stride-2], record[stride-1]
		if int(sequence.Length) > stride/2-1 {
			return file, fmt.Errorf("native: MYI sequence %d exceeds its version's word range", i)
		}
	}
	if file.Instrument[36] != 0 {
		if len(data)-end > 32768 {
			return file, fmt.Errorf("native: MYI sample exceeds 32 KiB")
		}
		if file.Version == 0 {
			for _, level := range data[end:] {
				if level >= 128 {
					break
				}
				if level > 15 {
					return file, fmt.Errorf("native: invalid legacy MYI0 DAC level")
				}
				file.Sample = append(file.Sample, legacyDACPCM[level])
			}
		} else {
			file.Sample = append([]byte(nil), data[end:]...)
		}
	} else {
		file.Trailing = append([]byte(nil), data[end:]...)
	}
	return file, nil
}

func EncodeInstrument(file InstrumentFile) ([]byte, error) {
	if file.Version > 3 || len(file.Sample) > 32768 || file.Version == 0 && len(file.Sample) > 0 {
		return nil, fmt.Errorf("native: unsupported MYI dimensions")
	}
	stride, count := 64, 7
	if file.Version >= 2 {
		stride = 128
	}
	if file.Version == 3 {
		count = 8
	}
	out := make([]byte, 56+count*stride)
	copy(out, "MYM3.MYI")
	out[3] = '0' + file.Version
	copy(out[8:56], file.Instrument[:48])
	for i := 0; i < count; i++ {
		sequence := file.Sequences[i]
		if int(sequence.Length) > stride/2-1 {
			return nil, fmt.Errorf("native: MYI sequence %d is too long", i)
		}
		at := 56 + i*stride
		for n := 0; n < stride/2-1; n++ {
			binary.BigEndian.PutUint16(out[at+n*2:], sequence.Values[n])
		}
		out[at+stride-2], out[at+stride-1] = sequence.Length, sequence.Repeat
	}
	if file.Instrument[36] != 0 {
		out = append(out, file.Sample...)
	} else {
		out = append(out, file.Trailing...)
	}
	return out, nil
}

func ExportInstrument(bank *model.VoiceBank, index int) (InstrumentFile, error) {
	if index < 0 || index >= model.MaxInstruments {
		return InstrumentFile{}, fmt.Errorf("native: invalid instrument index")
	}
	file := InstrumentFile{Version: 3, Instrument: bank.Instruments[index]}
	for i := range file.Sequences {
		file.Sequences[i] = bank.Sequences[file.Instrument[48+i]]
	}
	if sample := int(file.Instrument[36]); sample > 0 {
		if sample > model.MaxSamples {
			return InstrumentFile{}, fmt.Errorf("native: invalid instrument sample ID")
		}
		file.Sample = append([]byte(nil), bank.Samples[sample-1].PCM...)
	}
	return file, nil
}

// ImportInstrument allocates unused sequence/sample slots before updating the
// destination instrument. It never overwrites definitions used by other sounds.
func ImportInstrument(bank *model.VoiceBank, index int, file InstrumentFile) error {
	if index < 0 || index >= model.MaxInstruments {
		return fmt.Errorf("native: invalid instrument index")
	}
	result := *bank
	if file.Version > 3 || len(file.Sample) > 32768 {
		return fmt.Errorf("native: invalid instrument file dimensions")
	}
	var used [256]bool
	used[0] = true
	for _, instrument := range bank.Instruments {
		for _, id := range instrument[48:56] {
			used[id] = true
		}
	}
	for id, sequence := range bank.Sequences {
		if sequence.Length > 1 || sequence.Repeat > 0 {
			used[id] = true
		}
		for _, value := range sequence.Values {
			if value != 0 {
				used[id] = true
				break
			}
		}
	}
	inst := file.Instrument
	clear(inst[48:56])
	count := 7
	if file.Version == 3 {
		count = 8
	}
	for i := 0; i < count; i++ {
		sequence := file.Sequences[i]
		empty := sequence.Length <= 1 && sequence.Repeat == 0
		for _, value := range sequence.Values {
			empty = empty && value == 0
		}
		if empty {
			continue
		}
		id := 1
		for id < 256 && used[id] {
			id++
		}
		if id == 256 {
			return fmt.Errorf("native: no unused sequence slots for this instrument")
		}
		used[id] = true
		inst[48+i] = byte(id)
		result.Sequences[id] = sequence
		result.SequenceCount = max(result.SequenceCount, id+1)
	}
	inst[36] = 0
	if len(file.Sample) > 0 {
		var sampleUsed [8]bool
		for _, existing := range bank.Instruments {
			if id := existing[36]; id > 0 && id <= 8 {
				sampleUsed[id-1] = true
			}
		}
		slot := 0
		for slot < 8 && (len(result.Samples[slot].PCM) > 0 || sampleUsed[slot]) {
			slot++
		}
		if slot == 8 {
			return fmt.Errorf("native: no empty sample slot for this instrument")
		}
		inst[36] = byte(slot + 1)
		result.Samples[slot] = model.Sample{PCM: append([]byte(nil), file.Sample...), Trailer: []byte{0}}
	}
	result.Instruments[index] = inst
	result.Version = 1
	*bank = result
	return nil
}
