package project

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

func TestStartupDefaultSNDPrecedesNativePairAndRetainsItsReplay(t *testing.T) {
	files := defaultFiles(t)
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 72, Instrument: 1}
	p.Bank.Instruments[0].SetName("SND sound")
	bank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	song, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	prefix := make([]byte, 64)
	copy(prefix[12:], "SNDHTITLStartup\x00COMMAuthor\x00YEAR2024\x00HDNS")
	raw := append(prefix, bank...)
	raw = append(raw, song...)
	files["DEFAULT.SND"] = &fstest.MapFile{Data: raw}
	got, err := LoadDefaultsFS(files)
	if err != nil || got.SourcePath != "DEFAULT.SND" || got.ProjectPath != "DEFAULT.SND" || got.Project.Song.Patterns[0][0].Note != 72 || got.Project.Bank.Instruments[0].Name() != "SND sound" || got.Project.Year != "2024" || !bytes.Equal(got.Project.ReplaySource, raw) {
		t.Fatalf("default SND priority or runtime source was lost: %+v %v", got, err)
	}
}

func defaultFiles(t *testing.T) fstest.MapFS {
	t.Helper()
	p := model.New()
	p.Song.Patterns[0][0] = model.Cell{Note: 60, Instrument: 1}
	p.Song.State[40] = 2
	p.Bank.Instruments[0].SetName("Default bank")
	song, err := native.EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := native.EncodeVoiceBank(p.Bank)
	if err != nil {
		t.Fatal(err)
	}
	instrument, err := native.ExportInstrument(&p.Bank, 0)
	if err != nil {
		t.Fatal(err)
	}
	myi, err := native.EncodeInstrument(instrument)
	if err != nil {
		t.Fatal(err)
	}
	c := native.Configuration{}
	c[0] = 255
	c[17] = 5
	copy(c[13:17], "2026")
	return fstest.MapFS{
		"mym.cnf": {Data: c[:]}, "default.mys": {Data: song},
		"DeFaUlT.MyV": {Data: bank}, "DEFAULT.MYI": {Data: myi},
	}
}

func TestStartupDefaultsLoadsNativePairAndHonorsReloadPreference(t *testing.T) {
	for _, reload := range []bool{false, true} {
		files := defaultFiles(t)
		if reload {
			files["mym.cnf"].Data[10] = 255
		}
		before := append([]byte(nil), files["default.mys"].Data...)
		got, err := LoadDefaultsFS(files)
		if err != nil {
			t.Fatal(err)
		}
		wantChannel := byte(2)
		if reload {
			wantChannel = 5
		}
		if got.Configuration == nil || got.Project.Song.State[40] != wantChannel || got.Project.Song.Patterns[0][0].Note != 60 || got.Project.Bank.Instruments[0].Name() != "Default bank" || got.Project.Year != "2026" {
			t.Fatal("startup lost the native score, bank or configuration precedence")
		}
		if got.SourcePath != "default.mys" || got.ProjectPath != "default.mys" || !reflect.DeepEqual(before, files["default.mys"].Data) {
			t.Fatal("startup modified a native file or lost its save destination")
		}
	}
}

func TestStartupDefaultsFallsBackWithoutBorrowingAnUnrelatedBank(t *testing.T) {
	files := defaultFiles(t)
	files["DEFAULT.SND"] = &fstest.MapFile{Data: []byte("other player")}
	files["DeFaUlT.MyV"].Data = []byte("invalid bank")
	got, err := LoadDefaultsFS(files)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourcePath != "DEFAULT.MYI" || got.ProjectPath != "" || got.Project.Bank.Instruments[0].Name() != "Default bank" || got.Project.Song.Patterns[0][0].Note != 0 || got.Project.Song.State[40] != 5 {
		t.Fatal("fallback did not produce an independently loaded instrument workspace")
	}
	if len(got.Messages) != 3 {
		t.Fatalf("failed candidates were hidden: %v", got.Messages)
	}
	files = defaultFiles(t)
	delete(files, "DeFaUlT.MyV")
	got, err = LoadDefaultsFS(files)
	if err != nil || got.SourcePath != "DEFAULT.MYI" || len(got.Messages) != 1 || !strings.Contains(got.Messages[0], "missing") {
		t.Fatalf("song without its required bank replaced the sound workspace: %+v %v", got, err)
	}
}

func TestStartupDefaultsSupportsBankOnlyAndRejectsAmbiguousNames(t *testing.T) {
	files := defaultFiles(t)
	delete(files, "default.mys")
	got, err := LoadDefaultsFS(files)
	if err != nil || got.SourcePath != "DeFaUlT.MyV" || got.ProjectPath != "" || got.Project.Song.State[40] != 5 {
		t.Fatalf("bank-only startup failed: %+v %v", got, err)
	}
	files["DEFAULT.MYV"] = &fstest.MapFile{Data: files["DeFaUlT.MyV"].Data}
	if _, err := LoadDefaultsFS(files); err == nil {
		t.Fatal("ambiguous native startup filenames were chosen arbitrarily")
	}
}

func TestInvalidDefaultConfigurationDoesNotDiscardValidMusic(t *testing.T) {
	files := defaultFiles(t)
	files["mym.cnf"].Data = []byte{1}
	got, err := LoadDefaultsFS(files)
	if err != nil || got.Configuration != nil || got.SourcePath != "default.mys" || len(got.Messages) != 1 || got.Project.Song.State[40] != 2 {
		t.Fatalf("invalid optional configuration discarded a valid native pair: %+v %v", got, err)
	}
	got, err = LoadDefaultsFS(fstest.MapFS{})
	if err != nil || got.Project.Title != "First signal" || got.SourcePath != "" {
		t.Fatal("empty directory changed the ordinary startup composition")
	}
}
