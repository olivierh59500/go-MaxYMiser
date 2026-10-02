package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type EmbeddedProject struct {
	Song          model.Song
	Bank          model.VoiceBank
	Title, Author string
}

// DecodeContainer extracts editable MaxYMiser data from its own unpacked SNDH
// exports. It does not execute a 68000 program or accept unrelated SNDH players.
func DecodeContainer(data []byte) (EmbeddedProject, error) {
	var result EmbeddedProject
	var err error
	data, err = UnpackICE(data)
	if err != nil {
		return result, err
	}
	if len(data) < 16 || !bytes.Equal(data[12:16], []byte("SNDH")) {
		return result, fmt.Errorf("native: not an unpacked MaxYMiser container")
	}
	inst := bytes.Index(data, []byte("MYM1INST"))
	if inst < 32 {
		inst = bytes.Index(data, []byte("MYM0INST"))
	}
	if inst < 32 {
		return result, fmt.Errorf("native: container has no editable voice bank")
	}
	songAt := bytes.Index(data[inst:], []byte("MYM0TRAK"))
	if songAt < 0 {
		return result, fmt.Errorf("native: container has no editable song")
	}
	songAt += inst
	digi := bytes.Index(data[songAt:], []byte("MYM1DIGI"))
	if digi < 0 {
		digi = bytes.Index(data[songAt:], []byte("MYM0DIGI"))
	}
	if digi < 0 {
		return result, fmt.Errorf("native: container has no sample boundary")
	}
	digi += songAt
	song, err := DecodeSong(data[songAt:digi])
	if err != nil {
		return result, err
	}
	result.Song = song
	bankStart := inst - 32
	bank := append([]byte(nil), data[bankStart:songAt]...)
	bank = append(bank, data[digi:]...)
	gap := digi - songAt
	for i := 0; i < 8; i++ {
		offset := binary.BigEndian.Uint32(bank[i*4:])
		if offset < uint32(gap) {
			return result, fmt.Errorf("native: invalid embedded sample pointer")
		}
		binary.BigEndian.PutUint32(bank[i*4:], offset-uint32(gap))
	}
	result.Bank, err = DecodeVoiceBank(bank)
	if err != nil {
		return result, err
	}
	result.Title = containerText(data, "TITL")
	result.Author = containerText(data, "COMM")
	return result, nil
}
func containerText(data []byte, tag string) string {
	at := bytes.Index(data[:min(256, len(data))], []byte(tag))
	if at < 0 {
		return ""
	}
	tail := data[at+4:]
	end := bytes.IndexByte(tail, 0)
	if end < 0 {
		return ""
	}
	return string(tail[:end])
}
