package ymimport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusDoesNotCountDuplicateCapturesAsIndependentEvidence(t *testing.T) {
	dir := t.TempDir()
	a := simpleYM3(64)
	if err := os.WriteFile(filepath.Join(dir, "first.ym"), a, 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "copy.ym"), a, 0600)
	b := simpleYM3(128)
	os.WriteFile(filepath.Join(dir, "other-song.ym"), b, 0600)
	profile, err := Learn(dir, "Composer", nil)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Files != 3 || profile.Decoded != 3 || profile.Unique != 2 {
		t.Fatalf("duplicates inflated the corpus: %+v", profile)
	}
	if len(profile.Instruments) == 0 {
		t.Fatal("shared timbre was not detected")
	}
	trace, _ := Decode(a)
	events := ExtractEvents(trace, 0)
	family, score, ok := profile.Match(events[0].Features)
	if !ok || family.Files != 2 || score > 40 {
		t.Fatalf("generic stationary tone was reported with unjustified specificity: %+v score=%f", family, score)
	}
}
func TestInactiveNoiseRegisterDoesNotAlterTimbreFingerprint(t *testing.T) {
	a, _ := Decode(simpleYM3(64))
	b := a
	b.Frames = append([][14]byte(nil), a.Frames...)
	for i := range b.Frames {
		b.Frames[i][6] = byte(i % 32)
	}
	x := ExtractEvents(a, 0)
	y := ExtractEvents(b, 0)
	if signature("timbre", x[0].Features) != signature("timbre", y[0].Features) {
		t.Fatal("unrelated shared noise register changed a tone-only instrument")
	}
}
