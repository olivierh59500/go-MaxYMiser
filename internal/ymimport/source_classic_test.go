package ymimport

import (
	"encoding/binary"
	"testing"
)

// Constructed music tables exercise the independently relocated data regions.
// No original replay program or composition is included in this fixture.
func classicTableFixture() ([]byte, sourceDataLayout) {
	b := make([]byte, 0x1800)
	music := sourceReader{data: b, origin: 0x10000}
	program := sourceReader{data: b, origin: 0x20000}
	pointer := func(at, target int, origin uint32) { binary.BigEndian.PutUint32(b[at:], origin+uint32(target)) }
	layout := sourceDataLayout{player: madMaxClassic, music: music, program: program, instruments: 0x610, patterns: 0x614, songs: 0x618, speed: 0x9f0, arpeggios: 0x400, noise: [3]int{0x100, 0x104, 0x108}}
	pointer(layout.instruments, 0x700, music.origin)
	pointer(layout.patterns, 0x900, music.origin)
	pointer(layout.songs, 0xa00, music.origin)
	pointer(0xa00, 0xa10, music.origin)
	b[layout.speed] = 3
	for id := 0; id < 32; id++ {
		at := 0xb00 + id*16
		pointer(0x700+id*4, at+6, music.origin)
		copy(b[at:], []byte{0, 0, 1, 2, 1, 1, 15, 10, 0, 255})
	}
	for channel := 0; channel < 3; channel++ {
		at := 0xa20 + channel*4
		pointer(0xa10+channel*4, at, music.origin)
		copy(b[at:], []byte{0, 255})
	}
	pointer(0x400, 0x1401, program.origin)
	copy(b[0x1400:], []byte{0, 0, 0x8f})
	for _, at := range layout.noise {
		pointer(at, 0x1450, program.origin)
	}
	copy(b[0x1450:], []byte{1, 15, 3, 0, 255})
	pointer(0x900, 0x1000, music.origin)
	copy(b[0x1000:], []byte{0xc1, 0xe1, 60, 0x90, 0x87, 0x8e, 0x80, 0x8c, 62, 0x87})
	return b, layout
}

func TestClassicWaitAndLegatoRestRetainTheSoundingNote(t *testing.T) {
	b, layout := classicTableFixture()
	score, err := decodeSourceTables(b, layout, 23)
	if err != nil {
		t.Fatal(err)
	}
	var events []SourceEvent
	for _, event := range score.Events {
		if event.Channel == 0 {
			events = append(events, event)
		}
	}
	if len(events) != 2 || events[0].Frame != 0 || events[1].Frame != 18 || events[0].Note != 72 || events[1].Note != 74 || events[0].Rest || events[1].Rest {
		t.Fatalf("wait/legato rest stopped or retimed the voice: %+v", events)
	}
	commands := score.Patterns[0].Commands
	if commands[3].Opcode != 0x90 || len(commands[3].Operand) != 1 || commands[3].Operand[0] != 0x87 || commands[6].Opcode != 0x8c || len(commands[6].Operand) != 0 {
		t.Fatal("classic command lengths followed the later player format")
	}
	if len(score.Instruments) != 32 || len(score.Instruments[1].Arpeggio.Values) != 1 || score.Instruments[1].Arpeggio.Values[0] != 0 {
		t.Fatal("program pointers were decoded using the music relocation base")
	}
}

func TestClassicFixedPitchTriggerConsumesOneOpaqueByte(t *testing.T) {
	b, layout := classicTableFixture()
	b[0xb30] = 2
	copy(b[0x1000:], []byte{0xe1, 0xc3, 0x87, 0xc3, 63, 64, 0x87})
	score, err := decodeSourceTables(b, layout, 17)
	if err != nil {
		t.Fatal(err)
	}
	var events []SourceEvent
	for _, event := range score.Events {
		if event.Channel == 0 {
			events = append(events, event)
		}
	}
	if len(events) != 3 || events[0].Frame != 0 || events[1].Frame != 6 || events[2].Frame != 12 {
		t.Fatalf("opaque bytes changed fixed-pitch trigger timing: %+v", events)
	}
	for _, event := range events {
		if !event.FixedPitch || event.Instrument != 3 || event.Note != 28 {
			t.Fatalf("fixed pitch was replaced by the ignored score byte: %+v", event)
		}
	}
	if events[0].NativeNote != 16 || events[2].NativeNote != 64 || events[0].Offset != 0x1002 {
		t.Fatal("forced triggers or ordinary note commands lost their native source data")
	}
}

func TestClassicOrderStopSilencesAllVoicesAtOneBoundary(t *testing.T) {
	b, layout := classicTableFixture()
	b[0xa21] = 0xfe
	score, err := decodeSourceTables(b, layout, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(score.Stops) != 1 || score.Stops[0].Channel != 0 || score.Stops[0].Order != 1 {
		t.Fatalf("native stop marker was lost: %+v", score.Stops)
	}
	for channel := 0; channel < 3; channel++ {
		var last SourceEvent
		for _, event := range score.Events {
			if event.Channel == channel {
				last = event
			}
		}
		if last.Frame != 24 || !last.Rest {
			t.Fatalf("channel %d did not stop with the native active flag: %+v", channel, last)
		}
	}
	last := score.Controls[len(score.Controls)-1]
	if last.Opcode != 0xfe || last.Frame != 24 || last.Offset != 0xa21 {
		t.Fatalf("global stop lost its source location or execution frame: %+v", last)
	}
}

func TestClassicTablesRejectBadPointersAndUnknownCommandSets(t *testing.T) {
	for _, at := range []int{0x610, 0x614, 0x618, 0x700, 0x900, 0xa00, 0xa10, 0x400} {
		b, layout := classicTableFixture()
		binary.BigEndian.PutUint32(b[at:], 0xffffffff)
		if _, err := decodeSourceTables(b, layout, 30); err == nil {
			t.Fatalf("invalid classic table pointer at %#x was accepted", at)
		}
	}
	for _, op := range []byte{0x91, 0x92, 0x85, 0x86, 0x89} {
		b, layout := classicTableFixture()
		b[0x1000] = op
		if _, err := decodeSourceTables(b, layout, 30); err == nil {
			t.Fatalf("unverified classic command %#x was accepted", op)
		}
	}
	b, _ := classicTableFixture()
	copy(b[12:], "SNDH")
	copy(b[0x10f6:], []byte{0x53, 0x28, 0, 0x1b, 0x66, 0, 1, 0x6e, 0x11, 0x7c, 0, 0, 0, 0x2e, 0x10, 0xbc})
	if _, err := DecodeSource(b, 0, 30); err == nil {
		t.Fatal("a short classic parser signature selected an unverified complete player")
	}
}

func FuzzClassicSourceTables(f *testing.F) {
	seed, _ := classicTableFixture()
	f.Add(seed)
	f.Add([]byte("SNDH"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		_, layout := classicTableFixture()
		layout.music.data, layout.program.data = data, data
		decodeSourceTables(data, layout, 128)
	})
}
