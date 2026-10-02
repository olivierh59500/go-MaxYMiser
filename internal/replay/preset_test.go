package replay

import (
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"testing"
)

func TestNativeNoteOffPresetStopsBothPCMVoicesInEveryEnabledMode(t *testing.T) {
	for _, mode := range []byte{1, 2, 3, 4} {
		p := model.New()
		p.Song.State[49] = mode
		p.Song.Orders[0][3] = model.NoteOffPattern
		e := New(p)
		e.DMA[0] = PCMVoice{Sample: 1, Note: 60, Volume: 3}
		e.DMA[1] = PCMVoice{Sample: 2, Note: 64, Volume: 5}
		e.Mutes = 24
		e.Play(false)
		e.Tick()
		for i, v := range e.DMA {
			if v.Sample != 0 || v.Note != 0 || !v.Triggered || v.Volume != []byte{3, 5}[i] {
				t.Fatalf("mode %d preset left PCM lane %d active or changed its level: %+v", mode, i, v)
			}
		}
	}
}

func TestEmptyPresetAndLaterRowsKeepLivePCMPreview(t *testing.T) {
	for _, pattern := range []byte{model.EmptyPattern, model.NoteOffPattern} {
		p := model.New()
		p.Song.Orders[0][3] = pattern
		e := New(p)
		e.PlayFrom(true, 1)
		e.TriggerSample(1, 60, 2)
		e.Tick()
		if e.DMA[1].Sample != 2 || e.DMA[1].Note != 60 {
			t.Fatal("preset affected a live preview outside its first-row stop")
		}
	}
}
