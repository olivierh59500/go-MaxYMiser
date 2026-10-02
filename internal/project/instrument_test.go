package project

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestInstrumentImportProtectsBothStoredSequenceEffectColumns(t *testing.T) {
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Effect1: 'L', Parameter1: 3, Effect2: 'A', Parameter2: 4}
	file := native.InstrumentFile{Version: 3}
	file.Instrument.SetName("Imported")
	file.Sequences[0] = model.Sequence{Length: 2, Values: [63]uint16{15, 14}, Repeat: 1}
	file.Sequences[1] = model.Sequence{Length: 3, Values: [63]uint16{0, 4, 7}, Repeat: 0}
	before := p.Clone()
	if err := ImportInstrument(p, 7, file, native.InstrumentReservations{}); err != nil {
		t.Fatal(err)
	}
	if p.Bank.Sequences[3] != before.Bank.Sequences[3] || p.Bank.Sequences[4] != before.Bank.Sequences[4] || !reflect.DeepEqual(p.Song, before.Song) {
		t.Fatal("import overwrote sequences called only by the partition")
	}
	if p.Bank.Instruments[7][48] == 3 || p.Bank.Instruments[7][49] == 4 || p.Bank.Sequences[p.Bank.Instruments[7][49]] != file.Sequences[1] {
		t.Fatal("import did not allocate independent unreferenced sequence IDs")
	}
}

func TestInstrumentImportProtectsReservedPCMSampleLanesAndMIDIMappings(t *testing.T) {
	p := model.New()
	p.Song.State[49] = 0
	p.Song.Orders[200][3] = 2
	p.Song.Patterns[2][0] = model.Cell{Note: 60, Instrument: 2, Effect1: 64, Parameter1: 3}
	file := native.InstrumentFile{Version: 3, Sample: []byte{0, 127, 128, 255}}
	file.Instrument[36] = 1
	if err := ImportInstrument(p, 7, file, native.InstrumentReservations{}); err != nil {
		t.Fatal(err)
	}
	if p.Bank.Instruments[7][36] != 4 || !bytes.Equal(p.Bank.Samples[3].PCM, file.Sample) {
		t.Fatal("sample import reused a slot held by PCM rows or MIDI settings")
	}
	for slot := 0; slot < 3; slot++ {
		if len(p.Bank.Samples[slot].PCM) != 0 {
			t.Fatal("import changed a deliberately empty referenced sample")
		}
	}
}

func TestReferencedEmptySampleExhaustionKeepsTheInstrumentBankUnchanged(t *testing.T) {
	p := model.New()
	p.Song.Orders[200][3] = 2
	for id := byte(1); id <= 8; id++ {
		p.Song.Patterns[2][id].Instrument = id
	}
	before := p.Clone()
	file := native.InstrumentFile{Version: 3, Sample: []byte{1, 2, 3}}
	file.Instrument[36] = 1
	file.Sequences[0] = model.Sequence{Length: 2, Values: [63]uint16{15, 9}, Repeat: 1}
	if err := ImportInstrument(p, 7, file, native.InstrumentReservations{}); err == nil {
		t.Fatal("referenced empty sample slots were treated as available storage")
	}
	if !reflect.DeepEqual(p.Bank, before.Bank) || !reflect.DeepEqual(p.Song, before.Song) {
		t.Fatal("allocation exhaustion left a partially imported sound")
	}
}

func TestInstrumentImportProtectsAdditionalLiveReferences(t *testing.T) {
	p := model.New()
	reserved := native.InstrumentReservations{}
	reserved.Sequences[3], reserved.Samples[1] = true, true
	file := native.InstrumentFile{Version: 3, Sample: []byte{1, 2}}
	file.Instrument[36] = 1
	file.Sequences[0] = model.Sequence{Length: 2, Values: [63]uint16{15, 8}, Repeat: 1}
	if err := ImportInstrument(p, 7, file, reserved); err != nil {
		t.Fatal(err)
	}
	if p.Bank.Instruments[7][48] == 3 || p.Bank.Instruments[7][36] != 3 {
		t.Fatal("a live sequence/sample override was overwritten by instrument import")
	}
}

func TestInstrumentCanBeResavedWithEditedSequencesAndSample(t *testing.T) {
	for _, packed := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "ICE"}[packed], func(t *testing.T) {
			p := model.Demo()
			p.Bank.Instruments[1][36] = 1
			p.Bank.Samples[0].PCM = []byte{0, 127, 128, 255, 1, 2}
			root := t.TempDir()
			path := filepath.Join(root, "Chord.MYI")
			if err := SaveInstrument(&p.Bank, 1, path, packed); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0644 {
				t.Fatalf("new instrument has unexpected permissions: %v", err)
			}
			if err := os.Chmod(path, 0600); err != nil {
				t.Fatal(err)
			}
			p.Bank.Instruments[1].SetName("Edited chord")
			p.Bank.Instruments[1][38] = 4
			p.Bank.Sequences[3].Values[1] = 8
			p.Bank.Samples[0].PCM = []byte{128, 0, 127}
			if err := SaveInstrument(&p.Bank, 1, path, packed); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if packed != bytes.HasPrefix(raw, []byte("ICE!")) {
				t.Fatal("updated instrument lost its selected packing")
			}
			file, err := native.DecodeInstrument(raw)
			if err != nil {
				t.Fatal(err)
			}
			if file.Version != 3 || !bytes.Equal(file.Instrument[:48], p.Bank.Instruments[1][:48]) || file.Sequences[1] != p.Bank.Sequences[3] || !bytes.Equal(file.Sample, p.Bank.Samples[0].PCM) {
				t.Fatal("reopening the updated MYI lost the sound, sequence or sample edits")
			}
			info, err = os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("instrument update changed existing permissions: %v", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "Chord.MYI" {
				t.Fatalf("instrument update changed filename case or left temporary files: %v", err)
			}
		})
	}
}

func TestInvalidInstrumentKeepsTheSavedDefinition(t *testing.T) {
	p := model.Demo()
	path := filepath.Join(t.TempDir(), "sound.myi")
	if err := SaveInstrument(&p.Bank, 0, path, false); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Bank.Instruments[0][36] = 9
	if err := SaveInstrument(&p.Bank, 0, path, true); err == nil {
		t.Fatal("instrument with an invalid sample ID was exported")
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, old) {
		t.Fatalf("invalid instrument replaced the last complete sound: %v", err)
	}
}

func TestSampleCanBeResavedWithoutATrailerOrStaleTail(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "drum.pcm")
	sample := model.Sample{PCM: []byte{0, 127, 128, 255, 2, 3}, Trailer: []byte{128}}
	if err := SaveSample(sample, path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	sample.PCM = []byte{128, 127}
	if err := SaveSample(sample, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, sample.PCM) {
		t.Fatalf("updated sample contains an old tail or a bank trailer: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("sample update changed permissions: %v", err)
	}
	if err := SaveSample(model.Sample{PCM: make([]byte, 32769)}, path); err == nil {
		t.Fatal("oversized native sample was saved")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, raw) {
		t.Fatalf("invalid sample replaced the prior output: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("sample saves left temporary files: %v", err)
	}
}
