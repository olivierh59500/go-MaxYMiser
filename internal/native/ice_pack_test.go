package native

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
)

func TestICEPackingRoundTripsLiteralBoundariesAndRepeatedMatches(t *testing.T) {
	for _, size := range []int{1, 2, 4, 5, 7, 8, 14, 15, 269, 270, 1024, 32000, 100000} {
		data := make([]byte, size)
		rand.New(rand.NewSource(int64(size))).Read(data)
		packed, err := PackICE(data)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		got, err := UnpackICE(packed)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("size %d: decoded=%d err=%v", size, len(got), err)
		}
	}
	for _, data := range [][]byte{bytes.Repeat([]byte{0}, 9000), bytes.Repeat([]byte("some repeating tracker pattern"), 300), bytes.Repeat([]byte{0, 1, 2, 3, 4}, 400)} {
		packed, err := PackICE(data)
		if err != nil {
			t.Fatal(err)
		}
		got, err := UnpackICE(packed)
		if err != nil || !bytes.Equal(got, data) || len(packed) >= len(data)/2 {
			t.Fatalf("repeated payload compression failed: in=%d packed=%d err=%v", len(data), len(packed), err)
		}
	}
}

func TestPackedNativeSongDecodesThroughNormalImporter(t *testing.T) {
	p := model.Demo()
	raw, err := EncodeSong(p.Song)
	if err != nil {
		t.Fatal(err)
	}
	packed, err := PackICE(raw)
	if err != nil {
		t.Fatal(err)
	}
	song, err := DecodeSong(packed)
	if err != nil || song.Patterns[0] != p.Song.Patterns[0] || song.Orders != p.Song.Orders {
		t.Fatalf("native packed song: %v", err)
	}
}
