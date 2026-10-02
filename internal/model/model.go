// Package model describes editable MaxYMiser songs, instruments and sequences.
package model

const Rows, MaxPatterns, MaxInstruments, MaxSequences, MaxSamples = 64, 240, 32, 256, 8
const EmptyPattern, NoteOffPattern, LoopPattern byte = 255, 254, 253
const NoteOff byte = 1

// Cell retains the seven musical bytes of a native eight-byte tracker row.
// Its final native byte stores a run length, which is handled by the codec.
type Cell struct {
	Note, Instrument, Volume                 byte
	Effect1, Parameter1, Effect2, Parameter2 byte
}
type Pattern [Rows]Cell

type Song struct {
	Version        byte
	State          [64]byte
	Orders         [256][4]byte
	Length, Repeat byte
	Patterns       []Pattern
}

func (s *Song) Speed() int {
	if s.State[13] == 0 {
		return 6
	}
	return int(s.State[13])
}
func (s *Song) TickRate() int {
	if s.State[14] < 25 {
		return 50
	}
	return int(s.State[14])
}
func (s *Song) SetSpeed(speed int) { s.State[13] = byte(max(1, min(255, speed))) }
func (s *Song) SetTickRate(hz int) { s.State[14] = byte(max(25, min(200, hz))) }

// Instrument keeps reserved native bytes so loading and saving never destroys
// parameters unknown to an older editor. Bytes 16..55 contain playable settings.
type Instrument [64]byte

func (i *Instrument) Name() string {
	n := 0
	for n < 16 && i[n] != 0 {
		n++
	}
	return string(i[:n])
}
func (i *Instrument) SetName(name string) { clear(i[:16]); copy(i[:15], name) }

type Sequence struct {
	Values         [63]uint16
	Length, Repeat byte
}
type Sample struct {
	PCM        []byte
	Parameters [4]byte
	Trailer    []byte
}
type VoiceBank struct {
	Version       byte
	Instruments   [MaxInstruments]Instrument
	Sequences     [MaxSequences]Sequence
	SequenceCount int
	Samples       [MaxSamples]Sample
}

type Project struct {
	Title, Author string
	Song          Song
	Bank          VoiceBank
	// ReplaySource is a locally supplied MaxYMiser SNDH used for native export.
	// It is external runtime data, not part of the MYS/MYV editable payload.
	ReplaySource []byte
}

func New() *Project {
	p := &Project{Title: "Untitled", Song: Song{Version: 0, Length: 1, Patterns: make([]Pattern, 3)}, Bank: VoiceBank{Version: 1, SequenceCount: 16}}
	p.Song.State[8] = 1
	p.Song.State[12] = 3
	p.Song.State[36] = 7
	p.Song.State[37] = 0
	p.Song.State[49] = 2
	p.Song.State[56] = 7
	p.Song.SetSpeed(6)
	p.Song.SetTickRate(50)
	for i := range p.Song.Orders {
		p.Song.Orders[i] = [4]byte{EmptyPattern, EmptyPattern, EmptyPattern, EmptyPattern}
	}
	p.Song.Orders[0] = [4]byte{0, 1, 2, EmptyPattern}
	for i := range p.Bank.Sequences {
		p.Bank.Sequences[i].Length = 1
	}
	p.Bank.Sequences[1].Values[0] = 15
	p.Bank.Sequences[2].Values[0] = 0x0100
	p.Bank.Instruments[0].SetName("Square")
	p.Bank.Instruments[0][17] = 7
	p.Bank.Instruments[0][18] = 7
	p.Bank.Instruments[0][19] = 7
	p.Bank.Instruments[0][32] = 1
	p.Bank.Instruments[0][37] = 32
	p.Bank.Instruments[0][48] = 1
	p.Bank.Instruments[0][51] = 2
	return p
}
