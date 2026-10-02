package midi

import (
	"testing"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func TestCompleteMMCMessagesControlNativePlaybackAndRejectBrokenFrames(t *testing.T) {
	e := replay.New(model.New())
	ApplyMapped(e, []byte{0xf0, 0x7f, 0x7f, 6, 2, 0xf7})
	if !e.Playing {
		t.Fatal("complete MMC play did not start the tracker")
	}
	for _, message := range [][]byte{{0xf0, 0x7f, 0x7f, 6, 1, 0}, {0xf0, 0x7f, 0x7f, 6, 1, 0xf7, 0}, {0xf0, 0x7f, 0x80, 6, 1, 0xf7}, {0xf0, 0x7e, 0x7f, 6, 1, 0xf7}} {
		ApplyMapped(e, message)
		if !e.Playing {
			t.Fatal("malformed or unrelated SysEx stopped the song")
		}
	}
	ApplyMapped(e, []byte{0xf0, 0x7f, 0, 6, 1, 0xf7})
	if e.Playing {
		t.Fatal("complete MMC stop did not stop the tracker")
	}
}
