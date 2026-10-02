package native

import (
	"bytes"
	"encoding/binary"
)

// reorderCopySelector recognizes the relative table used by wrappers that copy
// a selected native payload into a common replay buffer. Table slots, rather
// than physical data order, define the original subtune numbers.
func reorderCopySelector(data []byte, projects []EmbeddedProject, bankStarts []int) []EmbeddedProject {
	if len(projects) < 2 || len(projects) != len(bankStarts) {
		return projects
	}
	for at := 16; at+24 <= min(len(data), 2048); at += 2 {
		// subq #1,D0; asl #2,D0; lea (pc),A0; adda #base,A0;
		// adda #adjust,A0; adda (A0,D0.w),A0.
		if !bytes.Equal(data[at:at+6], []byte{0x53, 0x40, 0xe5, 0x40, 0x41, 0xfa}) || !bytes.Equal(data[at+8:at+10], []byte{0xd1, 0xfc}) || !bytes.Equal(data[at+14:at+16], []byte{0xd1, 0xfc}) || !bytes.Equal(data[at+20:at+24], []byte{0xd1, 0xf0, 0, 0}) {
			continue
		}
		table := int64(at+6) + int64(int16(binary.BigEndian.Uint16(data[at+6:]))) + int64(int32(binary.BigEndian.Uint32(data[at+10:]))) + int64(int32(binary.BigEndian.Uint32(data[at+16:])))
		if table < 0 || table+int64(len(projects)*4) > int64(len(data)) {
			continue
		}
		var order []int
		used := map[int]bool{}
		for slot := 0; slot < len(projects); slot++ {
			pointer := table + int64(int32(binary.BigEndian.Uint32(data[int(table)+slot*4:]))) + 8
			index := -1
			for i, start := range bankStarts {
				if int64(start) == pointer {
					index = i
					break
				}
			}
			if index < 0 || used[index] {
				order = nil
				break
			}
			used[index] = true
			order = append(order, index)
		}
		if len(order) != len(projects) {
			continue
		}
		out := make([]EmbeddedProject, len(projects))
		for slot, index := range order {
			out[slot] = projects[index]
		}
		return out
	}
	return projects
}
