package native

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// literalICE is a hand-built format fixture. Bits encode one literal and the
// no-picture flag, followed by the backwards bit-stream sentinel.
func TestICEOneLiteralAndMalformedDimensions(t *testing.T) {
	packed := []byte{'I', 'C', 'E', '!', 0, 0, 0, 14, 0, 0, 0, 1, 'A', 0x90}
	got, err := UnpackICE(packed)
	if err != nil || !bytes.Equal(got, []byte("A")) {
		t.Fatalf("literal fixture: %q %v", got, err)
	}
	for at := 4; at < len(packed); at++ {
		if _, err := UnpackICE(packed[:at]); err == nil {
			t.Fatalf("truncated ICE accepted at %d", at)
		}
	}
	copy := append([]byte(nil), packed...)
	binary.BigEndian.PutUint32(copy[8:], maxICEOutput+1)
	if _, err := UnpackICE(copy); err == nil {
		t.Fatal("unbounded ICE output accepted")
	}
	plain := []byte("MYM0TRAK")
	if got, err := UnpackICE(plain); err != nil || !bytes.Equal(got, plain) {
		t.Fatal("ordinary native file was changed")
	}
}

func TestICERejectsInvalidBackReferenceAndLiteralRun(t *testing.T) {
	for _, control := range []byte{0x10, 0xff, 0xe0} {
		packed := []byte{'I', 'C', 'E', '!', 0, 0, 0, 14, 0, 0, 0, 1, 'A', control}
		if _, err := UnpackICE(packed); err == nil {
			t.Fatalf("invalid stream was accepted: %02X", control)
		}
	}
}

func TestICEMatchesTheOriginalEditorsCompressor(t *testing.T) {
	// Original synthetic data, compressed by the native editor under Hatari.
	// No original song or executable is included in this compatibility fixture.
	packed, err := hex.DecodeString("494345210000005c0000024904f579413c5ef08627b4e11f476f204d6178594d69736572206e61746976652049434520666978747572652e20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f67fee0")
	if err != nil {
		t.Fatal(err)
	}
	phrase := []byte("Go MaxYMiser native ICE fixture. ")
	for i := 0; i < 32; i++ {
		phrase = append(phrase, byte(i))
	}
	want := bytes.Repeat(phrase, 9)
	got, err := UnpackICE(packed)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("native compressor interoperability failed: %d bytes, %v", len(got), err)
	}
	for at := 4; at < len(packed); at++ {
		if _, err := UnpackICE(packed[:at]); err == nil {
			t.Fatalf("truncated native stream accepted at %d", at)
		}
	}
}

func FuzzICEInputNeverPanics(f *testing.F) {
	f.Add([]byte("ICE!"))
	f.Add([]byte{'I', 'C', 'E', '!', 0, 0, 0, 14, 0, 0, 0, 1, 'A', 0x90})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) >= 12 && bytes.EqualFold(data[:4], []byte("ICE!")) && binary.BigEndian.Uint32(data[8:12]) > 4096 {
			return
		}
		_, _ = UnpackICE(data)
	})
}
