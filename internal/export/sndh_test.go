package export

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

func exportSNDHFixture() []byte {
	b := make([]byte, 128)
	for i, target := range []int{64, 66, 68} {
		binary.BigEndian.PutUint16(b[i*4:], 0x6000)
		binary.BigEndian.PutUint16(b[i*4+2:], uint16(target-i*4-2))
	}
	copy(b[12:], "SNDHTITLConstructed\x00TC50\x00HDNS")
	copy(b[64:], []byte{0x4e, 0x75, 0x4e, 0x75})
	at := 68
	for _, value := range []uint32{0x00001c00, 0x01000100, 0x07003e00, 0x08000f00} {
		copy(b[at:], []byte{0x23, 0xfc, byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), 0, 0xff, 0x88, 0})
		at += 10
	}
	copy(b[at:], []byte{0x4e, 0x75})
	return b
}

func TestSNDHWAVRetainsOriginalExecutableAudio(t *testing.T) {
	raw := exportSNDHFixture()
	path := filepath.Join(t.TempDir(), "reference.wav")
	if err := SNDH(raw, 0, path, time.Second/10); err != nil {
		t.Fatal(err)
	}
	wav, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := sndh.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	r, err := sndh.NewRenderer(file, 1, 48000)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	want := make([]byte, 4800*4)
	if _, err := io.ReadFull(r, want); err != nil {
		t.Fatal(err)
	}
	if len(wav) != len(want)+44 || !bytes.Equal(wav[44:], want) {
		t.Fatal("reference export changed executable audio")
	}
	before := append([]byte(nil), wav...)
	raw[68], raw[69] = 0xff, 0xff
	if err := SNDH(raw, 1, path, time.Second); err == nil {
		t.Fatal("faulty executable export succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed rendering replaced the complete previous WAV")
	}
}
