package edit

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func arrangementFixture() model.Song {
	s := model.New().Song
	s.Length = 4
	s.Repeat = 2
	for i := 0; i < 4; i++ {
		s.Orders[i] = [4]byte{byte(i % 3), 255, 254, 255}
	}
	return s
}

func TestPositionInsertionDeletionAndCopyRetainMusicalRepeatAnchor(t *testing.T) {
	s := arrangementFixture()
	original := s.Orders
	block, err := CopyPositions(s, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertPositions(&s, 1, block); err != nil {
		t.Fatal(err)
	}
	if s.Length != 6 || s.Repeat != 4 || s.Orders[1] != original[0] || s.Orders[2] != original[1] || s.Orders[4] != original[2] {
		t.Fatal("inserted positions or repeat anchor changed")
	}
	if err := DeletePositions(&s, 1, 2); err != nil {
		t.Fatal(err)
	}
	if s.Length != 4 || s.Repeat != 2 || !reflect.DeepEqual(s.Orders[:4], original[:4]) {
		t.Fatal("deletion did not restore the original musical order")
	}
	raw, err := native.EncodeSong(s)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := native.DecodeSong(raw)
	if err != nil || decoded.Repeat != 2 || decoded.Orders[3] != s.Orders[3] {
		t.Fatalf("arrangement edit did not survive native saving: %v", err)
	}
}

func TestPositionDeletionMovesRemovedRepeatToItsSuccessor(t *testing.T) {
	s := arrangementFixture()
	if err := DeletePositions(&s, 1, 2); err != nil {
		t.Fatal(err)
	}
	if s.Length != 2 || s.Repeat != 1 || s.Orders[1][0] != 0 {
		t.Fatal("removed repeat section did not choose the successor")
	}
	before := s
	if err := DeletePositions(&s, 0, 1); err == nil || s.Orders != before.Orders || s.Length != before.Length {
		t.Fatal("invalid deletion changed the song")
	}
}

func TestCloningOneOrderPatternDoesNotChangeAnotherOccurrence(t *testing.T) {
	p := model.New()
	p.Song.Length = 2
	p.Song.Orders[1] = p.Song.Orders[0]
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	id, err := ClonePositionPattern(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	p.Song.Patterns[id][0].Note = 72
	if p.Song.Patterns[0][0].Note != 60 || p.Song.Orders[0][0] != 0 || p.Song.Orders[1][0] != id {
		t.Fatal("cloned occurrence retained shared mutable pattern storage")
	}
	before := p.Clone()
	p.Song.Orders[1][0] = 255
	if _, err := ClonePositionPattern(p, 1, 0); err == nil || len(p.Song.Patterns) != len(before.Song.Patterns) {
		t.Fatal("preset pattern was cloned as stored music")
	}
}

func TestOccurrenceClonePreservesPlaybackUntilItsOwnNotesAreEdited(t *testing.T) {
	p := model.Demo()
	before := p.Clone()
	if _, err := ClonePositionPattern(p, 1, 0); err != nil {
		t.Fatal(err)
	}
	left, right := replay.New(before), replay.New(p)
	left.Play(false)
	right.Play(false)
	for tick := 0; tick < 900; tick++ {
		left.Tick()
		right.Tick()
		if left.Registers != right.Registers || left.EnvelopeWrite != right.EnvelopeWrite {
			t.Fatalf("cloning an unchanged occurrence altered audio at tick %d", tick)
		}
	}
}
