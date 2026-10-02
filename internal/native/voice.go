package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

const bankHeader = 32 + 8 + 32*64 + 8*4

func DecodeVoiceBank(data []byte) (model.VoiceBank, error) {
	var bank model.VoiceBank
	var err error
	data, err = UnpackICE(data)
	if err != nil {
		return bank, err
	}
	if len(data) < bankHeader+8 || !bytes.Equal(data[32:35], []byte("MYM")) || !bytes.Equal(data[36:40], []byte("INST")) {
		return bank, fmt.Errorf("native: invalid MYV header")
	}
	bank.Version = data[35] - '0'
	if bank.Version > 1 {
		return bank, fmt.Errorf("native: unsupported MYV version %d", bank.Version)
	}
	var offsets [8]int
	for i := range offsets {
		offsets[i] = i*4 + int(binary.BigEndian.Uint32(data[i*4:]))
		if offsets[i] < bankHeader+8 || offsets[i] > len(data) {
			return bank, fmt.Errorf("native: invalid sample pointer %d", i)
		}
	}
	end := offsets[0] - 8
	if !bytes.Equal(data[end:end+3], []byte("MYM")) || !bytes.Equal(data[end+4:end+8], []byte("DIGI")) {
		return bank, fmt.Errorf("native: missing sample tag")
	}
	bank.SampleVersion = data[end+3] - '0'
	if bank.SampleVersion > 1 {
		return bank, fmt.Errorf("native: unsupported DIGI version")
	}
	stride := 128
	if bank.Version == 0 {
		stride = 64
	}
	if (end-bankHeader)%stride != 0 {
		return bank, fmt.Errorf("native: misaligned sequence data")
	}
	bank.SequenceCount = (end - bankHeader) / stride
	if bank.SequenceCount > model.MaxSequences {
		return bank, fmt.Errorf("native: too many sequences")
	}
	for i := range bank.Instruments {
		copy(bank.Instruments[i][:], data[40+i*64:104+i*64])
	}
	for i := 0; i < bank.SequenceCount; i++ {
		record := data[bankHeader+i*stride : bankHeader+(i+1)*stride]
		s := &bank.Sequences[i]
		for n := 0; n < stride/2-1; n++ {
			s.Values[n] = binary.BigEndian.Uint16(record[n*2:])
		}
		s.Length, s.Repeat = record[stride-2], record[stride-1]
		if int(s.Length) > stride/2-1 {
			return bank, fmt.Errorf("native: invalid sequence %d length %d", i, s.Length)
		}
	}
	for i, start := range offsets {
		end := len(data)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		if end < start {
			return bank, fmt.Errorf("native: sample offsets are not ordered")
		}
		sample := &bank.Samples[i]
		copy(sample.Parameters[:], data[2088+i*4:2092+i*4])
		length := int(binary.BigEndian.Uint16(sample.Parameters[:2]))
		if length > end-start {
			return bank, fmt.Errorf("native: truncated sample %d", i)
		}
		sample.PCM = append([]byte(nil), data[start:start+length]...)
		sample.Trailer = append([]byte(nil), data[start+length:end]...)
	}
	return bank, nil
}

func EncodeVoiceBank(bank model.VoiceBank) ([]byte, error) {
	if bank.Version > 1 || bank.SequenceCount < 1 || bank.SequenceCount > model.MaxSequences {
		return nil, fmt.Errorf("native: invalid voice bank dimensions")
	}
	stride := 128
	if bank.Version == 0 {
		stride = 64
	}
	out := make([]byte, bankHeader+stride*bank.SequenceCount+8)
	copy(out[32:], []byte("MYM1INST"))
	out[35] = '0' + bank.Version
	for i, inst := range bank.Instruments {
		copy(out[40+i*64:], inst[:])
	}
	for i, s := range bank.Sequences[:bank.SequenceCount] {
		if int(s.Length) > stride/2-1 {
			return nil, fmt.Errorf("native: sequence %d too long", i)
		}
		at := bankHeader + i*stride
		for n := 0; n < stride/2-1; n++ {
			binary.BigEndian.PutUint16(out[at+n*2:], s.Values[n])
		}
		out[at+stride-2], out[at+stride-1] = s.Length, s.Repeat
	}
	copy(out[len(out)-8:], []byte("MYM1DIGI"))
	out[len(out)-5] = '0' + bank.SampleVersion
	for i, s := range bank.Samples {
		if len(s.PCM) > 65535 {
			return nil, fmt.Errorf("native: sample %d too long", i)
		}
		binary.BigEndian.PutUint32(out[i*4:], uint32(len(out)-i*4))
		params := s.Parameters
		binary.BigEndian.PutUint16(params[:2], uint16(len(s.PCM)))
		copy(out[2088+i*4:], params[:])
		out = append(out, s.PCM...)
		if len(s.Trailer) > 0 {
			out = append(out, s.Trailer...)
		} else {
			out = append(out, 0)
		}
	}
	return out, nil
}
