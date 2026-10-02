package native

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

type EmbeddedProject struct {
	Song              model.Song
	Bank              model.VoiceBank
	Title, Author     string
	Subtune, Subtunes int
}

// DecodeContainer extracts the first editable native subtune. The bank, samples
// and tracker can appear in either order in optimized or multi-tune SNDH files.
func DecodeContainer(data []byte) (EmbeddedProject, error) {
	projects, err := DecodeContainers(data)
	if err != nil {
		return EmbeddedProject{}, err
	}
	if len(projects) == 0 {
		return EmbeddedProject{}, fmt.Errorf("native: container has no editable MaxYMiser data")
	}
	return projects[0], nil
}

func DecodeContainers(data []byte) ([]EmbeddedProject, error) {
	plain, err := UnpackICE(data)
	if err != nil {
		return nil, err
	}
	if len(plain) < 16 || !bytes.Equal(plain[12:16], []byte("SNDH")) {
		return nil, fmt.Errorf("native: not a MaxYMiser SNDH container")
	}
	var insts, songs []int
	for at := 0; at+8 <= len(plain); at++ {
		if bytes.Equal(plain[at:at+3], []byte("MYM")) {
			if (plain[at+3] == '0' || plain[at+3] == '1') && bytes.Equal(plain[at+4:at+8], []byte("INST")) {
				insts = append(insts, at)
			}
			if plain[at+3] == '0' && bytes.Equal(plain[at+4:at+8], []byte("TRAK")) {
				songs = append(songs, at)
			}
		}
	}
	if len(insts) == 0 || len(songs) == 0 {
		return nil, fmt.Errorf("native: container has no editable voice bank/song")
	}
	var projects []EmbeddedProject
	var bankStarts []int
	var firstError error
	usedSongs := map[int]bool{}
	for _, inst := range insts {
		if inst < 32 || inst+2088 > len(plain) {
			continue
		}
		start := inst - 32
		digi := start + int(binary.BigEndian.Uint32(plain[start:])) - 8
		if digi < inst || digi+8 > len(plain) || !bytes.Equal(plain[digi:digi+3], []byte("MYM")) || !bytes.Equal(plain[digi+4:digi+8], []byte("DIGI")) {
			continue
		}
		stride := 128
		if plain[inst+3] == '0' {
			stride = 64
		}
		candidates := []int{}
		for _, songAt := range songs {
			if !usedSongs[songAt] {
				candidates = append(candidates, songAt)
			}
		}
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				distance := func(at int) int {
					d := at - inst
					if d < 0 {
						d = -d
					}
					return d
				}
				if distance(candidates[j]) < distance(candidates[i]) {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}
		for _, songAt := range candidates {
			songEnd := len(plain)
			// Binary replay wrappers initialize three long values: voice
			// offset, song offset, song byte length. Read this known sequence
			// rather than treating alignment/footer bytes as more patterns.
			for at := 16; at+18 <= min(len(plain), 2048); at += 2 {
				if bytes.Equal(plain[at:at+6], []byte{0x20, 0xfc, 0, 0, 0, 12}) && plain[at+6] == 0x20 && plain[at+7] == 0xfc && plain[at+12] == 0x20 && plain[at+13] == 0xfc {
					length := int(binary.BigEndian.Uint32(plain[at+14:]))
					offset := int(binary.BigEndian.Uint32(plain[at+8:]))
					if start-8+offset == songAt && length >= songHeader && songAt+length <= len(plain) {
						songEnd = min(songEnd, songAt+length)
					}
				}
			}
			for _, boundary := range songs {
				if boundary > songAt && boundary < songEnd {
					songEnd = boundary
				}
			}
			for _, boundary := range insts {
				if boundary-32 > songAt && boundary-32 < songEnd {
					songEnd = boundary - 32
				}
			}
			if digi > songAt && digi < songEnd {
				songEnd = digi
			}
			// Ignore only alignment padding; never invent missing pattern rows.
			for songEnd > songAt+songHeader && (songEnd-songAt-songHeader)%8 != 0 {
				songEnd--
			}
			song, err := DecodeSong(plain[songAt:songEnd])
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				continue
			}
			seqEnd := digi
			gap := 0
			if songAt > inst && songAt < digi {
				seqEnd = songAt
				gap = digi - songAt
			}
			if seqEnd < start+bankHeader || (seqEnd-start-bankHeader)%stride != 0 {
				continue
			}
			bankEnd := len(plain)
			for _, boundary := range songs {
				if boundary > digi && boundary < bankEnd {
					bankEnd = boundary
				}
			}
			for _, boundary := range insts {
				if boundary-32 > digi && boundary-32 < bankEnd {
					bankEnd = boundary - 32
				}
			}
			bank := append([]byte(nil), plain[start:seqEnd]...)
			bank = append(bank, plain[digi:bankEnd]...)
			valid := true
			for i := 0; i < 8; i++ {
				offset := binary.BigEndian.Uint32(bank[i*4:])
				if offset < uint32(gap) {
					valid = false
					break
				}
				binary.BigEndian.PutUint32(bank[i*4:], offset-uint32(gap))
			}
			if !valid {
				continue
			}
			voice, err := decodeContainerBank(bank)
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				continue
			}
			projects = append(projects, EmbeddedProject{Song: song, Bank: voice, Title: containerText(plain, "TITL"), Author: containerText(plain, "COMM")})
			bankStarts = append(bankStarts, start)
			usedSongs[songAt] = true
			if len(insts) > 1 {
				break
			}
		}
	}
	if len(projects) == 0 {
		if firstError != nil {
			return nil, firstError
		}
		return nil, fmt.Errorf("native: unsupported MaxYMiser payload layout")
	}
	projects = reorderCopySelector(plain, projects, bankStarts)
	for i := range projects {
		projects[i].Subtune = i + 1
		projects[i].Subtunes = max(len(projects), DeclaredSubtunes(plain))
	}
	return projects, nil
}

func containerText(data []byte, tag string) string {
	at := bytes.Index(data[:min(512, len(data))], []byte(tag))
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
