package ui

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

var helpTopics = []string{"Keyboard", "Effects", "Instruments", "Files / YM", "MIDI"}
var helpText = [][]string{
	{
		"Space: song play/stop · Right Ctrl: pattern play · Right Shift: pattern record",
		"Tab / Shift+Tab: track · arrows: cursor · Page Up/Down: 16 rows",
		"F1–F8: octave · F9: percussion keyboard · F10: Jam · Shift+F10: disable Jam",
		"Z S X D C V G B H N J M: lower octave · Q 2 W 3 E R 5 T 6 Y 7 U: upper",
		"Caps Lock: note-off · Backspace: clear cell · Insert/Delete: insert/delete row",
		"Ctrl+C/X/V: copy/cut/paste selected range · Ctrl+Z/Y: undo/redo",
		"Ctrl+Left/Right: song position · Shift+Left/Right: live track pattern",
		"Enter: edit/preview · Ctrl+O/S: open/save · Ctrl+Shift+S: Save as",
		"Right-click Song: from beginning · right-click Pattern: from cursor row",
		"Ctrl+N / Unused: blank unreferenced pattern or sequence in its workspace",
		"Hexadecimal values: notes use names; instrument IDs 01–20; sequences 00–FF",
		"Volume: 0 loudest, F quietest · track mutes keep live notes available",
	},
	{
		"1–6: portamento, arpeggio, vibrato, transpose, fixed-period and detune masks",
		"7: PWM slide (80 resets) · 8: PWM sequence · 9: fixed pulse width",
		"A: arp sequence · B: pattern break · C: start sync · D/E: coarse/fine detune",
		"F: fixed sequence · G: DigiDrum sample · H: pitch slide · I: timer sequence",
		"L: volume sequence · M: mixer sequence · N: noise sequence · O: noise transpose",
		"P: portamento · Q: sequence speed · R: drum rate · S: ticks per tracker row",
		"T: transpose · U: Microwire bass/treble/master/pan · V: vibrato sequence",
		"W: buzzer shape · X: extra arpeggio (47 major; FC two-step octave)",
		"Y: timer allocation (bit 2=A, 1=B, 0=D) · Z: demo synchronization code",
		"Both YM effect columns work independently. PCM columns hold notes/samples.",
	},
	{
		"Masks select components: bit 2=square tone, bit 1=buzzer, bit 0=timer",
		"Sequences are shared. Click displayed values to edit the linked definition.",
		"Volume: 000F maximum, 0000 silence · arp/vibrato: signed 16-bit words",
		"Mixer nibbles: noise / tone / envelope shape / timer mode",
		"Timer modes: 1 SyncSquare, 5 SID, 9 PWM, B SyncBuzzer, C SyncBuzzer FM",
		"D DigiDrum, E Buzzer FM, F Square FM · waveform words depend on mode",
		"Fixed sequence FFFF restores the normal note period for that step.",
		"Repeat on the last sequence step holds it. Other repeat points loop.",
		"Generate/morph/modify support signed words; samples support tuning and YMise.",
	},
	{
		"MYS stores arrangement/patterns; MYV instruments, sequences and samples.",
		"Open MYS auto-loads matching MYV. MYI import allocates independent links.",
		"SNDH: native extraction, source inspection or executable audio with editable excerpts.",
		"Listen SNDH / score compares original audio and inferred notes or sampled excerpts.",
		"Native SNDH export needs a replay template; Settings edits composition year.",
		"ICE applies to native saves; WAV export runs separately from playback.",
		"YM records chip output. Reconstruction proposes data, not unique source recovery.",
		"Source excerpts retain notes; unsupported sounds stay silent and marked ?.",
		"Composer/paired profiles attach evidence; verified recipes can improve sounds.",
		"Listen YM / Listen score retains the reference. Save writes editable data.",
	},
	{
		"Native input/output ports are on macOS. Output connects to a selected destination.",
		"Controllers must be enabled in Settings; native CC values are scaled.",
		"Settings A–E values are 00–0F, corresponding to MIDI channels 1–16.",
		"MIDI sounds: 00 disabled, 01–20 YM, DD middle-C drums, 01–08 PCM.",
		"Clock: six F8 pulses make a row; instruments/effects advance once per pulse.",
		"Start/continue/stop and Song Position Pointer are supported.",
		"CC16–20: mutes · 21: speed · 22: position · 23: Jam · 24: mode · 25: row",
		"CC44–47 queue patterns at the boundary; sound controllers change parameters.",
		"Instrument CC edits are saved/undoable. MMC play/stop accepts complete frames.",
		"Latency advances 0–255 pulses on Start/Continue. Physical Sync24 is unavailable.",
	},
}

func (a *App) drawDetailedHelp(dst *ebiten.Image) {
	rect(dst, 24, 192, 1232, 482, panel)
	for i, topic := range helpTopics {
		a.btn(dst, topic, 42+i*232, 207, 216, 32, fmt.Sprintf("help:%d", i), a.helpTopic == i)
	}
	for i, line := range helpText[a.helpTopic] {
		a.text(dst, line, 42, float64(265+i*32), 12, fg)
	}
	a.text(dst, "MaxYMiser: gwEm / Dma-Sc / STSurvivor · Go: Olivier Houte / Malakh Software", 42, 650, 11, dim)
}

func (a *App) helpAction(name string) bool {
	if !strings.HasPrefix(name, "help:") {
		return false
	}
	var topic int
	if _, err := fmt.Sscanf(name, "help:%d", &topic); err == nil && topic >= 0 && topic < len(helpTopics) {
		a.helpTopic = topic
	}
	return true
}
