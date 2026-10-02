package native

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestSNDHDurationsUseOuterCallRateAndUpdateBothRepresentations(t *testing.T) {
	prefix := make([]byte, 128)
	copy(prefix[12:], "SNDH##02\x00TC200\x00TITLFRMS and TIME in a title\x00")
	copy(prefix[64:], "TIME")
	binary.BigEndian.PutUint16(prefix[68:], 1)
	binary.BigEndian.PutUint16(prefix[70:], 1)
	copy(prefix[72:], "FRMS")
	binary.BigEndian.PutUint32(prefix[76:], 0x54494d45) // Binary value spells TIME.
	copy(prefix[84:], "HDNS")
	rate, err := sndhDurationRate(prefix)
	if err != nil || rate != 200 {
		t.Fatalf("outer replay rate was not read: %d %v", rate, err)
	}
	before := append([]byte(nil), prefix...)
	for attempt := 0; attempt < 2; attempt++ {
		if err := setSNDHDurations(prefix, []time.Duration{1250 * time.Millisecond, time.Second + time.Nanosecond}, rate); err != nil {
			t.Fatal(err)
		}
		if binary.BigEndian.Uint16(prefix[68:]) != 1 || binary.BigEndian.Uint16(prefix[70:]) != 1 || binary.BigEndian.Uint32(prefix[76:]) != 250 || binary.BigEndian.Uint32(prefix[80:]) != 201 {
			t.Fatal("seconds/frame duration metadata was not updated consistently")
		}
	}
	if !bytes.Equal(prefix[:64], before[:64]) || !bytes.Equal(prefix[84:], before[84:]) {
		t.Fatal("duration updates changed text or executable boundaries")
	}
}

func TestFrameOnlySNDHDurationsAndIndefinitePlayback(t *testing.T) {
	prefix := make([]byte, 64)
	copy(prefix[12:], "SNDH##02\x00TC200\x00FRMS")
	copy(prefix[42:], "HDNS")
	at := bytes.Index(prefix, []byte("FRMS"))
	if err := setSNDHDurations(prefix, []time.Duration{time.Minute, 0}, 200); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(prefix[at+4:]) != 12000 || binary.BigEndian.Uint32(prefix[at+8:]) != 0 {
		t.Fatal("FRMS used a song's internal clock or lost indefinite playback")
	}
	if err := setSNDHDurations(prefix, []time.Duration{0, 0}, 200); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(prefix[at+4:]) != 0 {
		t.Fatal("indefinite export retained an old frame duration")
	}
}

func TestSNDHDurationsRejectTruncationAndOverflowWithoutChangingHeader(t *testing.T) {
	for _, tag := range []string{"TIME", "FRMS"} {
		prefix := make([]byte, 64)
		copy(prefix[12:], "SNDH")
		copy(prefix[20:], tag)
		copy(prefix[26:], "HDNS")
		before := append([]byte(nil), prefix...)
		if err := setSNDHDurations(prefix, []time.Duration{0, 0}, 200); err == nil || !bytes.Equal(prefix, before) {
			t.Fatal("incomplete duration array was accepted or changed")
		}
	}
	prefix := make([]byte, 64)
	copy(prefix[12:], "SNDHFRMS")
	copy(prefix[24:], "HDNS")
	before := append([]byte(nil), prefix...)
	if err := setSNDHDurations(prefix, []time.Duration{time.Duration(1<<63 - 1)}, 2000); err == nil || !bytes.Equal(prefix, before) {
		t.Fatal("frame overflow was accepted or changed the header")
	}
}
