package native

import (
	"bytes"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestNativeConfigurationMappingRetainsHardwareAndEditorBytes(t *testing.T) {
	raw := []byte{255, 0, 7, 6, 0, 0, 7, 0, 0, 0, 255, 255, 0, 50, 48, 50, 53, 0, 1, 2, 3, 4, 1, 1, 1, 1, 1, 0, 0}
	config, err := DecodeConfiguration(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := model.New()
	config.Apply(&p.Song)
	if p.Song.State[11] != 255 || p.Song.State[19] != 7 || p.Song.State[43] != 3 || p.Song.State[51] != 4 || p.Song.State[35] != 1 || p.Song.State[31] != 0 {
		t.Fatal("native CNF changed the wrong state offsets")
	}
	got := CaptureConfiguration(p.Song, config)
	if !bytes.Equal(got[:], raw) {
		t.Fatal("configuration round-trip lost non-Go hardware fields")
	}
	if _, err := DecodeConfiguration(raw[:28]); err == nil {
		t.Fatal("truncated configuration accepted")
	}
}
