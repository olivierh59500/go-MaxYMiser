package ymimport

import (
	"encoding/binary"
	"testing"
)

// This constructed score has no original music or native program payload. Its
// byte locations exercise the pointer layout identified by the player decoder.
func sourceFixture() []byte {
	b := make([]byte, 0x2700)
	copy(b[12:], "SNDH")
	copy(b[0x1d1a:], []byte{0x2c, 0, 0xd0, 0x40, 0xd0, 0x40, 0x26, 0x7a, 1, 0xc6, 0xd7, 0xc0})
	copy(b[0x15b6:], []byte{0x53, 0x28, 0, 0x1b, 0x66, 0, 1, 0x80, 0x13, 0xfc, 0, 0x2f})
	copy(b[0x356:], []byte{0x48, 0xe7, 0xff, 0xfe, 0x41, 0xfa, 0x1b, 0x74})
	copy(b[0x1ee:], []byte{0x22, 0x3c})
	const origin = 0x10000
	binary.BigEndian.PutUint32(b[0x1f0:], origin+0x356)
	copy(b[0x1fa:], []byte{0x41, 0xfa, 1, 0x5a})
	pointer := func(at, offset int) { binary.BigEndian.PutUint32(b[at:], origin+uint32(offset)) }
	pointer(0x1ee0, 0x1f00)
	pointer(0x1ee4, 0x2400)
	pointer(0x1ee8, 0x2300)
	pointer(0x2300, 0x2310)
	b[0x22c0] = 3
	for i := 0; i < 32; i++ {
		at := 0x2000 + i*16
		pointer(0x1f00+i*4, at+6)
		copy(b[at:], []byte{0, 0, 1, 1, 0, 0, 15, 10, 0, 255})
	}
	for ch := 0; ch < 3; ch++ {
		at := 0x2340 + ch*8
		pointer(0x2310+ch*4, at)
		copy(b[at:], []byte{0x80, 12, 0, 255})
	}
	pointer(0x2400, 0x2500)
	copy(b[0x2500:], []byte{0xc3, 0xe1, 60, 0x8e, 62, 0x91, 3, 64, 0x87})
	return b
}

func TestSourceDecoderRetainsNativePatternsAndLegato(t *testing.T) {
	s, err := DecodeSource(sourceFixture(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Patterns) != 1 || len(s.Instruments) != 32 || s.Speed != 3 {
		t.Fatalf("wrong source tables: %+v", s)
	}
	var events []SourceEvent
	for _, e := range s.Events {
		if e.Channel == 0 {
			events = append(events, e)
		}
	}
	if len(events) != 4 || events[0].Frame != 0 || events[1].Frame != 6 || events[2].Frame != 12 || events[3].Frame != 24 {
		t.Fatalf("wrong native timing: %+v", events)
	}
	if events[0].Instrument != 3 || events[0].Note != 84 || !events[0].Retrigger || events[1].Retrigger || !events[2].Retrigger {
		t.Fatalf("wrong source instrument/transpose/legato: %+v", events)
	}
	if s.Patterns[0].Offset != 0x2500 || s.Instruments[3].Settings[2] != 1 {
		t.Fatal("original source locations or instrument parameters were lost")
	}
}

func TestSourceDecoderRejectsBrokenPointersAndUnrecognizedPlayers(t *testing.T) {
	for _, at := range []int{0x1ee0, 0x1ee4, 0x1ee8, 0x2300, 0x2310, 0x2400, 0x1f00} {
		b := sourceFixture()
		binary.BigEndian.PutUint32(b[at:], 0xffffffff)
		if _, err := DecodeSource(b, 0, 50); err == nil {
			t.Fatalf("accepted invalid pointer at %#x", at)
		}
	}
	b := sourceFixture()
	b[0x2500] = 0x9f
	if _, err := DecodeSource(b, 0, 50); err == nil {
		t.Fatal("accepted unknown source command")
	}
	if _, err := DecodeSource([]byte("SNDH Mad Max Last Ninja"), 0, 50); err == nil {
		t.Fatal("title or author selected the source decoder")
	}
	if _, err := DecodeSource(sourceFixture(), 1, 50); err == nil {
		t.Fatal("unverified source subtune accepted")
	}
}

func FuzzSourceDecoder(f *testing.F) {
	f.Add(sourceFixture())
	f.Add([]byte("SNDH"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		DecodeSource(b, 0, 128)
	})
}
