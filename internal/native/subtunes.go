package native

import (
	"bytes"
	"strconv"
)

// DeclaredSubtunes reads the bounded ASCII ##nn tag in a SNDH header. The
// declared count is kept separate from the editable payloads actually decoded.
func DeclaredSubtunes(data []byte) int {
	plain, err := UnpackICE(data)
	if err != nil || len(plain) < 16 || !bytes.Equal(plain[12:16], []byte("SNDH")) {
		return 0
	}
	end := bytes.Index(plain, []byte("HDNS"))
	if end < 16 || end > 512 {
		return 0
	}
	for at := 16; at+4 <= end; at++ {
		if !bytes.Equal(plain[at:at+2], []byte("##")) || at > 16 && plain[at-1] != 0 {
			continue
		}
		count, err := strconv.Atoi(string(plain[at+2 : at+4]))
		if err == nil && count > 0 {
			return count
		}
	}
	return 1
}
