package sndh

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func headerFixture(tags []byte) []byte {
	data := make([]byte, 16)
	copy(data[12:], "SNDH")
	data = append(data, tags...)
	data = append(data, "HDNS"...)
	for _, at := range []int{0, 4, 8} {
		binary.BigEndian.PutUint16(data[at:], 0x6000)
		binary.BigEndian.PutUint16(data[at+2:], uint16(len(data)-at-2))
	}
	return append(data, 0x4e, 0x75)
}

func headerLongs(values ...uint32) []byte {
	data := make([]byte, len(values)*4)
	for i, value := range values {
		binary.BigEndian.PutUint32(data[i*4:], value)
	}
	return data
}

func TestHeaderMetadataDurationsAndOwnedInput(t *testing.T) {
	tags := []byte("TITLHDNS TIME FRMS are title text\x00COMMÉric\x00RIPPTester\x00CONVFixture\x00YEAR1991\x00##02\x00!#02\x00TIME")
	tags = append(tags, 0, 3, 0, 0)
	tags = append(tags, "TC60\x00FLAG~ay\x00"...)
	raw := headerFixture(tags)
	file, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	m := file.Metadata
	if m.Title != "HDNS TIME FRMS are title text" || m.Author != "Éric" || m.Ripper != "Tester" || m.Converter != "Fixture" || m.Year != "1991" {
		t.Fatalf("text metadata: %+v", m)
	}
	if m.Rate != 60 || m.Subtunes != 2 || m.DefaultSubtune != 2 || !reflect.DeepEqual(m.Frames, []uint32{180, 0}) || !reflect.DeepEqual(m.Flags, []string{"ay", "ay"}) {
		t.Fatalf("playback metadata: %+v", m)
	}
	if m.HeaderEnd != 16+len(tags)+4 {
		t.Fatalf("header end = %d", m.HeaderEnd)
	}
	before := append([]byte(nil), file.Data...)
	clear(raw)
	if !bytes.Equal(file.Data, before) {
		t.Fatal("parsed executable aliases caller input")
	}
}

func TestHeaderPerSubtuneFlagsAndNames(t *testing.T) {
	tags := []byte("##03\x00#!03\x00FLAG")
	// Both aliases and suffix pointers occur in real headers. Offsets are
	// relative to the tag, including its four-byte identifier.
	tags = append(tags, 0, 10, 0, 11, 0, 10)
	tags = append(tags, "abdy\x00!#SN"...)
	tags = append(tags, 0, 10, 0, 14, 0, 19)
	tags = append(tags, "One\x00Deux\x00Three\x00FRMS"...)
	tags = append(tags, headerLongs(300, 0, 900)...)
	file, err := Parse(headerFixture(tags))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(file.Metadata.Flags, []string{"abdy", "bdy", "abdy"}) || !reflect.DeepEqual(file.Metadata.SubtuneNames, []string{"One", "Deux", "Three"}) {
		t.Fatalf("string tables: %+v", file.Metadata)
	}
	if file.Metadata.DefaultSubtune != 3 || !reflect.DeepEqual(file.Metadata.Frames, []uint32{300, 0, 900}) {
		t.Fatalf("subtunes: %+v", file.Metadata)
	}
}

func TestHeaderDurationPrecedenceAndCountAfterArray(t *testing.T) {
	tags := []byte("FRMS")
	tags = append(tags, headerLongs(37, 4000)...)
	tags = append(tags, "##02\x00TIME"...)
	tags = append(tags, 0, 9, 0, 12)
	tags = append(tags, "TC200\x00"...)
	file, err := Parse(headerFixture(tags))
	if err != nil {
		t.Fatal(err)
	}
	if file.Metadata.Subtunes != 2 || !reflect.DeepEqual(file.Metadata.Frames, []uint32{37, 4000}) {
		t.Fatalf("FRMS lost its original call counts: %+v", file.Metadata)
	}
}

func TestHeaderPackedDefaultsAndLegacyText(t *testing.T) {
	raw := headerFixture([]byte("TITLFixture\x00COMM\xe9\x00!#99\x00"))
	packed, err := native.PackICE(raw)
	if err != nil {
		t.Fatal(err)
	}
	file, err := Parse(packed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(file.Data, raw) || file.Metadata.Rate != 50 || file.Metadata.Subtunes != 1 || file.Metadata.DefaultSubtune != 1 || file.Metadata.Frames != nil {
		t.Fatalf("packed defaults: %+v", file.Metadata)
	}
	if file.Metadata.Author != "é" || !utf8.ValidString(file.Metadata.Author) {
		t.Fatalf("legacy composer = %q", file.Metadata.Author)
	}
}

func TestHeaderAcceptsOddMarkerAndPreInitEntryRewrite(t *testing.T) {
	raw := headerFixture([]byte("TITLX\x00\x00"))
	if bytes.Index(raw, []byte("HDNS"))%2 != 1 {
		t.Fatal("fixture must have an odd HDNS offset")
	}
	// Some initializers relocate the player and rewrite this instruction.
	binary.BigEndian.PutUint16(raw[10:], 0x8000)
	if _, err := Parse(raw); err != nil {
		t.Fatal(err)
	}
}

func TestHeaderRejectsMalformedTablesAndMetadata(t *testing.T) {
	for name, tags := range map[string][]byte{
		"invalid count":           []byte("##2x\x00"),
		"conflicting counts":      []byte("##02\x00##03\x00"),
		"zero rate":               []byte("TC0\x00"),
		"invalid rate":            []byte("TCxyz\x00"),
		"flag offset into table":  []byte("##02\x00FLAG\x00\x04\x00\x08ay\x00"),
		"flag offset out of file": []byte("FLAG\xff\xff"),
		"name offset into table":  []byte("!#SN\x00\x04X\x00"),
		"truncated frame array":   []byte("##02\x00FRMS\x00\x00\x00\x01"),
		"duplicate frames":        append(append([]byte("FRMS"), headerLongs(1)...), append([]byte("FRMS"), headerLongs(2)...)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(headerFixture(tags)); err == nil {
				t.Fatal("malformed header was accepted")
			}
		})
	}
	for name, raw := range map[string][]byte{
		"missing marker":    append(make([]byte, 12), []byte("SNDHTITLFixture\x00")...),
		"unterminated text": append(make([]byte, 12), []byte("SNDHTITLFixture")...),
		"truncated flag":    append(make([]byte, 12), []byte("SNDHFLAG")...),
		"oversized file":    make([]byte, maxFileSize+1),
		"bad ice":           []byte("ICE!\x00\x00\x00\x0d\xff\xff\xff\xff\x80"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatal("malformed file was accepted")
			}
		})
	}
}

func FuzzParseHeader(f *testing.F) {
	f.Add(headerFixture([]byte("TITLFixture\x00TC50\x00FLAG~y\x00")))
	f.Add(headerFixture([]byte("##02\x00FLAG\x00\x08\x00\x09ay\x00")))
	f.Add(append(make([]byte, 12), []byte("SNDHFLAG")...))
	f.Fuzz(func(t *testing.T, raw []byte) {
		file, err := Parse(raw)
		if err != nil {
			return
		}
		m := file.Metadata
		if m.Subtunes < 1 || m.Subtunes > 99 || m.Rate < 1 || m.HeaderEnd < 20 || m.HeaderEnd > len(file.Data) || len(file.Data) > maxFileSize {
			t.Fatalf("invalid accepted metadata: %+v", m)
		}
		if m.Frames != nil && len(m.Frames) != m.Subtunes || m.Flags != nil && len(m.Flags) != m.Subtunes || m.SubtuneNames != nil && len(m.SubtuneNames) != m.Subtunes {
			t.Fatalf("inconsistent metadata table lengths: %+v", m)
		}
	})
}
