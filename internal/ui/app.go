// Package ui provides the modern keyboard-and-mouse tracker interface.
package ui

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/midi"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
	"golang.org/x/image/font/gofont/gomono"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const width, height = 1280, 800

var bg = color.RGBA{13, 17, 26, 255}
var panel = color.RGBA{22, 28, 40, 255}
var line = color.RGBA{43, 53, 73, 255}
var fg = color.RGBA{222, 230, 241, 255}
var dim = color.RGBA{117, 137, 161, 255}
var accent = color.RGBA{71, 213, 183, 255}
var purple = color.RGBA{170, 139, 249, 255}

type button struct {
	x, y, w, h int
	action     string
}
type App struct {
	corpus                                                             *ymimport.Corpus
	ymPath                                                             string
	ymReport                                                           *ymimport.Report
	midiInput                                                          *midi.Input
	midiData                                                           chan []byte
	midiDecoder                                                        midi.Decoder
	undo, redo                                                         []*model.Project
	copied                                                             model.Pattern
	hasCopy                                                            bool
	synth                                                              *replay.Synth
	player                                                             *audio.Player
	font                                                               *text.GoTextFaceSource
	projectPath, status, tab                                           string
	buttons                                                            []button
	row, channel, field, nibble, pattern, instrument, sequence, sample int
	octave, step, scroll                                               int
	editing, dirty                                                     bool
	modal, entry                                                       string
	listed                                                             []fs.DirEntry
	directory                                                          string
	mouseX, mouseY                                                     int
	ctrl                                                               bool
	lastSave                                                           time.Time
}

func New(p *model.Project, projectPath string, mute bool) (*App, error) {
	face, err := text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
	if err != nil {
		return nil, err
	}
	a := &App{synth: replay.NewSynth(replay.New(p), 48000), font: face, projectPath: projectPath, status: "Ready · Space plays · Enter edits · Ctrl+S saves", tab: "Patterns", midiData: make(chan []byte, 64), octave: 4, step: 1, directory: "."}
	if !mute {
		context := audio.CurrentContext()
		if context == nil {
			context = audio.NewContext(48000)
		}
		a.player, err = context.NewPlayer(a.synth)
		if err != nil {
			return nil, err
		}
		a.player.SetBufferSize(40 * time.Millisecond)
		a.player.Play()
	}
	return a, nil
}
func (a *App) Close() {
	if a.midiInput != nil {
		a.midiInput.Close()
	}
	if a.player != nil {
		a.player.Close()
	}
}
func (a *App) Layout(_, _ int) (int, int) { return width, height }
func (a *App) Synth() *replay.Synth       { return a.synth }
func (a *App) text(dst *ebiten.Image, s string, x, y float64, size float64, c color.RGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, &text.GoTextFace{Source: a.font, Size: size}, op)
}
func rect(dst *ebiten.Image, x, y, w, h float32, c color.RGBA) {
	vector.FillRect(dst, x, y, w, h, c, false)
}
func (a *App) btn(dst *ebiten.Image, label string, x, y, w, h int, action string, active bool) {
	c := line
	if active {
		c = color.RGBA{36, 75, 76, 255}
	}
	rect(dst, float32(x), float32(y), float32(w), float32(h), c)
	if a.mouseX >= x && a.mouseX < x+w && a.mouseY >= y && a.mouseY < y+h {
		vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 1, accent, false)
	}
	color := fg
	if active {
		color = accent
	}
	a.text(dst, label, float64(x+10), float64(y+9), 13, color)
	a.buttons = append(a.buttons, button{x, y, w, h, action})
}
func (a *App) Draw(dst *ebiten.Image) {
	e, wave := a.synth.Snapshot()
	p := e.Project
	a.buttons = a.buttons[:0]
	dst.Fill(bg)
	a.text(dst, "MaxYMiser Go", 24, 18, 24, fg)
	a.text(dst, "YM2149 + STe sample tracker", 25, 51, 12, dim)
	a.btn(dst, "New", 510, 20, 64, 36, "new", false)
	a.btn(dst, "Open", 582, 20, 72, 36, "open", false)
	a.btn(dst, "Save", 662, 20, 72, 36, "save", a.dirty)
	a.btn(dst, "Song", 764, 20, 78, 36, "play", e.Playing && !e.PatternMode)
	a.btn(dst, "Pattern", 850, 20, 94, 36, "pattern", e.Playing && e.PatternMode)
	a.btn(dst, "Stop", 952, 20, 74, 36, "stop", false)
	a.btn(dst, "Record", 1034, 20, 90, 36, "record", a.editing)
	a.btn(dst, "Export", 1132, 20, 122, 36, "export", false)
	tabs := []string{"Patterns", "Song", "Instruments", "Sequences", "Samples", "YM", "Settings", "Help"}
	for i, name := range tabs {
		a.btn(dst, name, 24+i*154, 88, 144, 36, "tab:"+name, a.tab == name)
	}
	rect(dst, 24, 136, 1232, 44, panel)
	a.text(dst, p.Title, 38, 147, 16, fg)
	a.text(dst, fmt.Sprintf("%03d Hz  ·  speed %02d  ·  %.1f BPM", p.Song.TickRate(), e.Speed, float64(p.Song.TickRate())*15/float64(max(1, e.Speed))), 520, 150, 13, accent)
	a.text(dst, fmt.Sprintf("Position %02X / %02X  ·  row %02X", e.Position, p.Song.Length, e.Row), 942, 150, 13, dim)
	switch a.tab {
	case "Patterns":
		a.drawPatterns(dst, &e)
	case "Song":
		a.drawSong(dst, &e)
	case "Instruments":
		a.drawInstruments(dst, &e)
	case "Sequences":
		a.drawSequences(dst, &e)
	case "Samples":
		a.drawSamples(dst, &e)
	case "YM":
		a.drawYM(dst)
	case "Settings":
		a.drawSettings(dst, &e)
	case "Help":
		a.drawHelp(dst)
	}
	rect(dst, 24, 692, 1232, 62, panel)
	for i := 0; i < len(wave)-1; i++ {
		x0 := float32(34) + float32(i)*1212/511
		y0 := float32(723) - wave[i]*24
		x1 := float32(34) + float32(i+1)*1212/511
		y1 := float32(723) - wave[i+1]*24
		vector.StrokeLine(dst, x0, y0, x1, y1, 1, accent, false)
	}
	a.text(dst, a.status, 24, 768, 12, dim)
	a.text(dst, fmt.Sprintf("Octave %d · step %d · %s", a.octave, a.step, map[bool]string{true: "EDIT", false: "PREVIEW"}[a.editing]), 984, 768, 12, purple)
	if a.modal != "" {
		a.drawModal(dst)
	}
}
func noteName(n byte) string {
	if n == 0 {
		return "---"
	}
	if n == 1 {
		return "OFF"
	}
	names := []string{"C-", "C#", "D-", "D#", "E-", "F-", "F#", "G-", "G#", "A-", "A#", "B-"}
	return fmt.Sprintf("%s%d", names[int(n)%12], int(n)/12-1)
}
func hexOrDash(n byte) string {
	if n == 0 {
		return "--"
	}
	return fmt.Sprintf("%02X", n)
}
func effect(code, value byte) string {
	if code == 0 {
		return "---"
	}
	return fmt.Sprintf("%c%02X", code, value)
}
func (a *App) drawPatterns(dst *ebiten.Image, e *replay.Engine) {
	p := e.Project
	rect(dst, 24, 192, 900, 482, panel)
	rect(dst, 938, 192, 318, 482, panel)
	a.text(dst, "PATTERN EDITOR", 40, 206, 12, dim)
	a.btn(dst, "−", 180, 199, 40, 32, "pat:-", false)
	a.text(dst, fmt.Sprintf("%02X", a.pattern), 238, 207, 14, accent)
	a.btn(dst, "+", 282, 199, 40, 32, "pat:+", false)
	a.btn(dst, "Copy", 344, 199, 70, 32, "copy-pattern", false)
	a.btn(dst, "Clear", 424, 199, 80, 32, "clear-pattern", false)
	a.btn(dst, "YM", 658, 199, 70, 32, "mode:ym", a.channel < 3)
	a.btn(dst, "DMA", 738, 199, 80, 32, "mode:dma", a.channel == 3)
	if a.channel == 3 {
		a.drawDMAPattern(dst, e)
		return
	}
	for ch := 0; ch < 3; ch++ {
		x := 88 + ch*272
		a.text(dst, fmt.Sprintf("YM %c   NOTE IN V   FX1  FX2", 'A'+ch), float64(x), 248, 12, accent)
		a.btn(dst, "Mute", x+196, 238, 60, 30, fmt.Sprintf("mute:%d", ch), e.Mutes&(1<<ch) != 0)
	}
	selected := a.pattern
	if selected >= len(p.Song.Patterns) {
		selected = 0
	}
	start := max(0, min(44, a.row-10))
	if e.Playing && !a.editing {
		start = max(0, min(44, e.Row-10))
	}
	for visible := 0; visible < 20; visible++ {
		row := start + visible
		y := 280 + visible*19
		if row%4 == 0 {
			rect(dst, 36, float32(y), 876, 19, color.RGBA{27, 35, 49, 255})
		}
		if e.Playing && row == e.Row {
			rect(dst, 36, float32(y), 876, 19, color.RGBA{31, 70, 67, 255})
		}
		a.text(dst, fmt.Sprintf("%02X", row), 42, float64(y+2), 12, dim)
		for ch := 0; ch < 3; ch++ {
			pattern := selected
			if e.Playing {
				pattern = int(e.Patterns[ch])
			} else if ch != a.channel {
				pattern = int(p.Song.Orders[e.Position][ch])
			}
			var cell model.Cell
			if pattern < len(p.Song.Patterns) {
				cell = p.Song.Patterns[pattern][row]
			}
			x := 88 + ch*272
			if row == a.row && ch == a.channel {
				rect(dst, float32(x-4), float32(y), 256, 19, color.RGBA{57, 43, 80, 255})
			}
			volume := "-"
			if cell.Volume > 0 {
				volume = fmt.Sprintf("%X", cell.Volume&15)
			}
			a.text(dst, fmt.Sprintf("%s %s %s   %s  %s", noteName(cell.Note), hexOrDash(cell.Instrument), volume, effect(cell.Effect1, cell.Parameter1), effect(cell.Effect2, cell.Parameter2)), float64(x), float64(y+1), 13, fg)
			a.buttons = append(a.buttons, button{x, y, 256, 19, fmt.Sprintf("cell:%d:%d", ch, row)})
		}
	}
	a.text(dst, "INSTRUMENTS", 954, 206, 12, dim)
	a.btn(dst, "I", 1120, 198, 50, 28, "bank:0", a.instrument < 16)
	a.btn(dst, "II", 1180, 198, 50, 28, "bank:1", a.instrument >= 16)
	for i := 0; i < 16; i++ {
		index := (a.instrument/16)*16 + i
		label := fmt.Sprintf("%02X  %s", index+1, p.Bank.Instruments[index].Name())
		a.btn(dst, label, 954, 234+i*24, 286, 22, fmt.Sprintf("instrument:%d", index), index == a.instrument)
	}
	a.text(dst, "Note keys: Z–M and Q–U", 954, 635, 12, dim)
	a.text(dst, "Tab changes channel", 954, 653, 12, dim)
}
func (a *App) drawSong(dst *ebiten.Image, e *replay.Engine) {
	p := e.Project
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, "SONG ORDER · four independent pattern lists", 42, 208, 15, fg)
	a.btn(dst, "Add position", 1010, 204, 150, 34, "order:add", false)
	for pos := 0; pos < int(p.Song.Length) && pos < 18; pos++ {
		y := 252 + pos*22
		if pos == e.Position {
			rect(dst, 40, float32(y), 1200, 22, color.RGBA{31, 70, 67, 255})
		}
		a.text(dst, fmt.Sprintf("%02X", pos), 50, float64(y+2), 13, dim)
		for ch := 0; ch < 4; ch++ {
			a.btn(dst, fmt.Sprintf("%02X", p.Song.Orders[pos][ch]), 160+ch*248, y, 206, 22, fmt.Sprintf("order:%d:%d", pos, ch), false)
		}
	}
}
func (a *App) drawInstruments(dst *ebiten.Image, e *replay.Engine) {
	p := e.Project
	rect(dst, 24, 192, 260, 482, panel)
	rect(dst, 298, 192, 958, 482, panel)
	for i := 0; i < 16; i++ {
		index := (a.instrument/16)*16 + i
		a.btn(dst, fmt.Sprintf("%02X %s", index+1, p.Bank.Instruments[index].Name()), 36, 208+i*27, 236, 24, fmt.Sprintf("instrument:%d", index), index == a.instrument)
	}
	inst := p.Bank.Instruments[a.instrument]
	a.text(dst, fmt.Sprintf("%02X · %s", a.instrument+1, inst.Name()), 320, 210, 20, fg)
	a.btn(dst, "I", 846, 202, 52, 32, "bank:0", a.instrument < 16)
	a.btn(dst, "II", 906, 202, 52, 32, "bank:1", a.instrument >= 16)
	a.btn(dst, "Rename", 1100, 202, 126, 34, "rename-instrument", false)
	labels := []string{"Portamento mask", "Arpeggio mask", "Vibrato mask", "Transpose mask", "Fixed frequency", "Fixed detune", "Sequence speed", "Pulse width", "Envelope shape", "Start sync", "Digi sample", "Digi rate", "Attenuation", "Detune coarse", "Detune fine", "Frequency resolution"}
	offsets := []int{16, 17, 18, 19, 20, 21, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41}
	for i, label := range labels {
		x := 320 + (i%2)*450
		y := 254 + (i/2)*42
		a.text(dst, label, float64(x), float64(y+9), 12, dim)
		a.btn(dst, fmt.Sprintf("%02X", inst[offsets[i]]), x+262, y, 140, 32, fmt.Sprintf("parameter:%d", offsets[i]), false)
	}
	names := []string{"Volume", "Arpeggio", "Vibrato", "Mixer", "Noise", "Fixed", "Timer", "PWM"}
	for i, name := range names {
		x := 320 + (i%4)*225
		y := 603 + (i/4)*30
		a.text(dst, name, float64(x), float64(y+6), 12, dim)
		a.btn(dst, fmt.Sprintf("%02X", inst[48+i]), x+118, y, 78, 26, fmt.Sprintf("parameter:%d", 48+i), false)
	}
}
func (a *App) drawSequences(dst *ebiten.Image, e *replay.Engine) {
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, "WORD SEQUENCES", 42, 208, 16, fg)
	a.btn(dst, "−", 280, 203, 42, 32, "seq:-", false)
	a.text(dst, fmt.Sprintf("%02X", a.sequence), 342, 209, 15, accent)
	a.btn(dst, "+", 388, 203, 42, 32, "seq:+", false)
	s := e.Project.Bank.Sequences[a.sequence]
	a.btn(dst, fmt.Sprintf("Length %02X", s.Length), 472, 203, 146, 32, "seq-length", false)
	a.btn(dst, fmt.Sprintf("Repeat %02X", s.Repeat), 632, 203, 146, 32, "seq-repeat", false)
	for i := 0; i < 63; i++ {
		x := 42 + (i%9)*134
		y := 260 + (i/9)*49
		a.text(dst, fmt.Sprintf("%02X", i), float64(x), float64(y), 11, dim)
		a.btn(dst, fmt.Sprintf("%04X", s.Values[i]), x+30, y-5, 94, 35, fmt.Sprintf("seq-value:%d", i), i < int(s.Length))
	}
	a.text(dst, "Values are native 16-bit words. Repeat at the last step holds its final value.", 42, 646, 12, dim)
}
func (a *App) drawSamples(dst *ebiten.Image, e *replay.Engine) {
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, "SAMPLE BANK · signed 8-bit PCM", 42, 208, 16, fg)
	for i := 0; i < 8; i++ {
		a.btn(dst, fmt.Sprintf("%d", i+1), 42+i*148, 247, 136, 34, fmt.Sprintf("sample:%d", i), i == a.sample)
	}
	pcm := e.Project.Bank.Samples[a.sample].PCM
	a.text(dst, fmt.Sprintf("Sample %d · %d bytes", a.sample+1, len(pcm)), 42, 305, 14, accent)
	rect(dst, 42, 348, 1192, 190, bg)
	if len(pcm) > 1 {
		for x := 0; x < 1190; x++ {
			at := x * (len(pcm) - 1) / 1190
			next := (x + 1) * (len(pcm) - 1) / 1190
			y0 := float32(443) - float32(int8(pcm[at]))*.65
			y1 := float32(443) - float32(int8(pcm[next]))*.65
			vector.StrokeLine(dst, float32(43+x), y0, float32(44+x), y1, 1, purple, false)
		}
	}
	a.btn(dst, "Import PCM / WAV", 42, 570, 200, 38, "import-sample", false)
	a.btn(dst, "Clear sample", 258, 570, 176, 38, "clear-sample", false)
	a.text(dst, "STe has two independent sample voices; DigiDrums use the YM channel DAC.", 42, 636, 12, dim)
}
func (a *App) drawSettings(dst *ebiten.Image, e *replay.Engine) {
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, "PLAYBACK & EDITING", 42, 210, 16, fg)
	items := []struct{ label, value, action string }{{"Replay rate", fmt.Sprint(e.Project.Song.TickRate()), "rate"}, {"Ticks per row", fmt.Sprint(e.Speed), "speed"}, {"Keyboard octave", fmt.Sprint(a.octave), "octave"}, {"Edit step", fmt.Sprint(a.step), "step"}, {"Global volume", fmt.Sprint(e.MasterVolume), "master"}, {"Timer allocation", fmt.Sprintf("%X", e.TimerMask), "timers"}}
	for i, v := range items {
		y := 264 + i*57
		a.text(dst, v.label, 48, float64(y+12), 14, dim)
		a.btn(dst, v.value, 370, y, 220, 38, "setting:"+v.action, false)
	}
	a.text(dst, "Audio is rendered at 48 kHz. Sequencing follows the selected replay rate.", 670, 277, 13, fg)
	a.text(dst, "The YM noise generator and envelope are shared between all three voices.", 670, 321, 12, dim)
	a.btn(dst, "MIDI input", 670, 372, 180, 38, "midi", a.midiInput != nil)
}
func (a *App) drawHelp(dst *ebiten.Image) {
	rect(dst, 24, 192, 1232, 482, panel)
	lines := []string{"Space: play / stop song        Right Ctrl: play current patterns", "Enter: toggle editing          Tab / Shift+Tab: select channel", "Arrow keys: move cursor        Page Up / Down: move by 16 rows", "Z S X D C V G B H N J M: lower keyboard octave", "Q 2 W 3 E R 5 T 6 Y 7 U: upper keyboard octave", "Backspace: clear cell          Caps Lock: note off", "Ctrl+S: save native MYS + MYV   Ctrl+O: open a native project", "Instrument and sequence fields accept hexadecimal values.", "MaxYMiser original code/design: Gareth Morris / gwEm.", "Original design: Dma-Sc. This Go edition uses YM Player."}
	for i, s := range lines {
		a.text(dst, s, 46, float64(222+i*38), 14, fg)
	}
}
func (a *App) drawModal(dst *ebiten.Image) {
	rect(dst, 0, 0, width, height, color.RGBA{0, 0, 0, 180})
	rect(dst, 220, 224, 840, 290, panel)
	a.text(dst, a.modal, 250, 248, 18, fg)
	rect(dst, 248, 292, 784, 58, bg)
	a.text(dst, a.entry, 260, 312, 15, accent)
	a.text(dst, "Enter confirms · Escape cancels", 250, 376, 13, dim)
	a.btn(dst, "Cancel", 800, 437, 100, 40, "modal:cancel", false)
	a.btn(dst, "Apply", 918, 437, 114, 40, "modal:apply", true)
}
func (a *App) Update() error {
	for {
		select {
		case data := <-a.midiData:
			a.midiDecoder.Feed(data, func(message []byte) { a.synth.Edit(func(e *replay.Engine) { midi.Apply(e, message) }) })
		default:
			goto midiDone
		}
	}
midiDone:
	a.mouseX, a.mouseY = ebiten.CursorPosition()
	a.ctrl = ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if a.modal != "" {
		chars := ebiten.AppendInputChars(nil)
		a.entry += string(chars)
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(a.entry) > 0 {
			a.entry = a.entry[:len(a.entry)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			a.modal = ""
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			a.applyModal()
		}
	} else {
		a.keyboard()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i := len(a.buttons) - 1; i >= 0; i-- {
			b := a.buttons[i]
			if a.mouseX >= b.x && a.mouseX < b.x+b.w && a.mouseY >= b.y && a.mouseY < b.y+b.h {
				a.action(b.action)
				break
			}
		}
	}
	if files := ebiten.DroppedFiles(); files != nil {
		fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".mys") {
				a.status = "Use Open to choose the dropped native song path."
			}
			return nil
		})
	}
	return nil
}
func (a *App) keyboard() {
	if inpututil.IsKeyJustPressed(ebiten.KeyControlRight) {
		a.action("pattern")
		return
	}
	if a.ctrl {
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
			a.restore(false)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			a.restore(true)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyC) {
			a.synth.Edit(func(e *replay.Engine) { a.copied = e.Project.Song.Patterns[a.pattern]; a.hasCopy = true })
			a.status = "Pattern copied"
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyV) && a.hasCopy {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[a.pattern] = a.copied })
			a.dirty = true
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			a.action("save")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			a.action("open")
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		a.action("play")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyControlRight) {
		a.action("pattern")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		a.editing = !a.editing
	}
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		delta := 1
		if shift {
			delta = -1
		}
		a.selectChannel((a.channel + delta + 4) % 4)
	}
	for key, delta := range map[ebiten.Key]int{ebiten.KeyArrowUp: -1, ebiten.KeyArrowDown: 1, ebiten.KeyPageUp: -16, ebiten.KeyPageDown: 16} {
		if inpututil.IsKeyJustPressed(key) {
			a.row = max(0, min(63, a.row+delta))
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		a.field = max(0, a.field-1)
		a.nibble = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		a.field = min(6, a.field+1)
		a.nibble = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		a.editCell(func(c *model.Cell) { *c = model.Cell{} })
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyCapsLock) {
		a.enterNote(1)
	}
	keys := []ebiten.Key{ebiten.KeyZ, ebiten.KeyS, ebiten.KeyX, ebiten.KeyD, ebiten.KeyC, ebiten.KeyV, ebiten.KeyG, ebiten.KeyB, ebiten.KeyH, ebiten.KeyN, ebiten.KeyJ, ebiten.KeyM}
	upper := []ebiten.Key{ebiten.KeyQ, ebiten.KeyDigit2, ebiten.KeyW, ebiten.KeyDigit3, ebiten.KeyE, ebiten.KeyR, ebiten.KeyDigit5, ebiten.KeyT, ebiten.KeyDigit6, ebiten.KeyY, ebiten.KeyDigit7, ebiten.KeyU}
	if a.field == 0 || a.channel == 3 && a.field == 3 {
		for n, key := range keys {
			if inpututil.IsKeyJustPressed(key) {
				a.enterNote(byte(12 + a.octave*12 + n))
			}
		}
		for n, key := range upper {
			if inpututil.IsKeyJustPressed(key) {
				a.enterNote(byte(24 + a.octave*12 + n))
			}
		}
	}
	if a.field > 0 && a.editing {
		for _, r := range ebiten.AppendInputChars(nil) {
			a.enterField(r)
		}
	}
}
func (a *App) enterNote(note byte) {
	if a.editing {
		a.remember()
	}
	playing := false
	a.synth.Edit(func(e *replay.Engine) {
		if a.channel < 3 {
			e.Trigger(a.channel, note, byte(a.instrument+1))
		}
		playing = e.Playing
		if a.editing {
			pattern, row := a.pattern, a.row
			if e.Playing {
				pattern, row = int(e.Patterns[a.channel]), e.Row
			}
			if pattern < len(e.Project.Song.Patterns) {
				cell := &e.Project.Song.Patterns[pattern][row]
				if a.channel < 3 {
					cell.Note = note
					cell.Instrument = byte(a.instrument + 1)
				} else if a.field >= 3 {
					cell.Effect1 = note
					cell.Parameter1 = byte(a.sample + 1)
				} else {
					cell.Note = note
					cell.Instrument = byte(a.sample + 1)
				}
			}
		}
	})
	if a.editing {
		a.dirty = true
		if !playing {
			a.row = (a.row + a.step) % 64
		}
	}
}
func (a *App) editCell(fn func(*model.Cell)) {
	a.remember()
	a.synth.Edit(func(e *replay.Engine) {
		if a.pattern < len(e.Project.Song.Patterns) {
			fn(&e.Project.Song.Patterns[a.pattern][a.row])
		}
	})
	a.dirty = true
}
func (a *App) enterField(r rune) {
	if a.field == 3 || a.field == 5 {
		r = []rune(strings.ToUpper(string(r)))[0]
		if r >= '0' && r <= 'Z' {
			a.editCell(func(c *model.Cell) {
				if a.field == 3 {
					c.Effect1 = byte(r)
				} else {
					c.Effect2 = byte(r)
				}
			})
			a.field++
			return
		}
	}
	value, err := strconv.ParseUint(string(r), 16, 4)
	if err != nil {
		return
	}
	a.editCell(func(c *model.Cell) {
		var target *byte
		switch a.field {
		case 1:
			target = &c.Instrument
		case 2:
			target = &c.Volume
		case 4:
			target = &c.Parameter1
		case 6:
			target = &c.Parameter2
		}
		if target == nil {
			return
		}
		if a.nibble == 0 {
			*target = byte(value) << 4
		} else {
			*target = (*target & 240) | byte(value)
		}
		if a.field == 2 {
			*target = byte(value)
			if *target == 0 {
				*target = 16
			}
		}
	})
	a.nibble = (a.nibble + 1) % 2
	if a.nibble == 0 {
		a.row = (a.row + a.step) % 64
	}
}
func (a *App) action(name string) {
	if strings.HasPrefix(name, "tab:") {
		a.tab = strings.TrimPrefix(name, "tab:")
		return
	}
	if strings.HasPrefix(name, "modal:") {
		if name == "modal:apply" {
			a.applyModal()
		} else {
			a.modal = ""
		}
		return
	}
	if a.modal != "" {
		return
	}
	var x, y int
	if _, err := fmt.Sscanf(name, "dma-cell:%d", &x); err == nil {
		a.row = x
		if a.mouseX >= 500 {
			a.field = 3
		} else {
			a.field = 0
		}
		return
	}
	if _, err := fmt.Sscanf(name, "cell:%d:%d", &x, &y); err == nil {
		a.selectChannel(x)
		a.row = y
		character := int(float64(a.mouseX-(88+x*272)) / 7.8)
		switch {
		case character < 4:
			a.field = 0
		case character < 7:
			a.field = 1
		case character < 11:
			a.field = 2
		case character < 12:
			a.field = 3
		case character < 16:
			a.field = 4
		case character < 17:
			a.field = 5
		default:
			a.field = 6
		}
		return
	}
	if _, err := fmt.Sscanf(name, "instrument:%d", &x); err == nil {
		a.instrument = x
		return
	}
	if _, err := fmt.Sscanf(name, "sample:%d", &x); err == nil {
		a.sample = x
		return
	}
	if _, err := fmt.Sscanf(name, "seq-value:%d", &x); err == nil {
		e, _ := a.synth.Snapshot()
		a.modal = fmt.Sprintf("Sequence word %d", x)
		a.entry = fmt.Sprintf("%04X", e.Project.Bank.Sequences[a.sequence].Values[x])
		return
	}
	if _, err := fmt.Sscanf(name, "parameter:%d", &x); err == nil {
		e, _ := a.synth.Snapshot()
		a.modal = fmt.Sprintf("Instrument parameter %d", x)
		a.entry = fmt.Sprintf("%02X", e.Project.Bank.Instruments[a.instrument][x])
		return
	}
	if _, err := fmt.Sscanf(name, "mute:%d", &x); err == nil {
		a.synth.Edit(func(e *replay.Engine) { e.Mutes ^= 1 << x; e.Project.Song.State[37] = e.Mutes })
		return
	}
	if _, err := fmt.Sscanf(name, "order:%d:%d", &x, &y); err == nil {
		e, _ := a.synth.Snapshot()
		a.modal = fmt.Sprintf("Order %d channel %d", x, y)
		a.entry = fmt.Sprintf("%02X", e.Project.Song.Orders[x][y])
		return
	}
	switch name {
	case "ym:infer":
		data, err := os.ReadFile(a.ymPath)
		if err != nil {
			a.status = err.Error()
			return
		}
		trace, err := ymimport.Decode(data)
		if err != nil {
			a.status = err.Error()
			return
		}
		candidate, report, err := ymimport.Reconstruct(trace)
		if err != nil {
			a.status = err.Error()
			return
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project = candidate; e.Reset() })
		if a.corpus != nil {
			report.AuthorProfile = a.corpus.Author
			report.Evidence = a.corpus.Evidence(trace)
		}
		a.ymReport = &report
		a.pattern, a.row, a.channel = 0, 0, 0
		a.projectPath = ""
		a.dirty = true
		a.status = fmt.Sprintf("Reconstructed candidate: %d instruments, %d patterns. Compare with the original YM.", report.Instruments, report.Patterns)
	case "ym:profile":
		a.modal = "Load composer profile (.json)"
		a.entry = ""
	case "ym:reference":
		a.synth.SelectReference(true)
	case "ym:score":
		a.synth.SelectReference(false)
	case "midi":
		if a.midiInput != nil {
			a.midiInput.Close()
			a.midiInput = nil
			a.status = "MIDI input disconnected"
		} else {
			input, err := midi.Open(func(data []byte) {
				select {
				case a.midiData <- data:
				default:
				}
			})
			if err != nil {
				a.status = err.Error()
			} else {
				a.midiInput = input
				a.status = "MIDI input connected · channels 1–3 control YM A–C"
			}
		}
	case "mode:ym":
		a.selectChannel(0)
	case "mode:dma":
		a.selectChannel(3)
	case "bank:0":
		a.instrument = a.instrument % 16
	case "bank:1":
		a.instrument = 16 + a.instrument%16
	case "new":
		if _, ok := a.synth.Reference(); ok {
			a.synth.SelectReference(false)
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = model.New(); e.Reset() })
		a.projectPath = ""
		a.dirty = false
		a.status = "New project"
	case "open":
		a.modal = "Open music (.mys / .snd / .ym)"
		a.entry = a.projectPath
	case "save":
		if a.projectPath == "" {
			a.modal = "Save project (.mys + .myv)"
			a.entry = filepath.Join(a.directory, "untitled.mys")
		} else {
			a.save(a.projectPath)
		}
	case "play":
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.ToggleYM()
			return
		}
		a.synth.Edit(func(e *replay.Engine) {
			if e.Playing {
				e.Stop()
			} else {
				e.Play(false)
			}
		})
	case "pattern":
		a.synth.CloseYM()
		a.synth.Edit(func(e *replay.Engine) { e.Patterns[a.channel] = byte(a.pattern); e.Play(true) })
	case "stop":
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.StopYM()
			return
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop() })
	case "record":
		a.editing = !a.editing
	case "pat:+":
		a.synth.Edit(func(e *replay.Engine) {
			a.pattern++
			if a.pattern >= 240 {
				a.pattern = 239
			}
			for len(e.Project.Song.Patterns) <= a.pattern {
				e.Project.Song.Patterns = append(e.Project.Song.Patterns, model.Pattern{})
			}
		})
	case "pat:-":
		a.pattern = max(0, a.pattern-1)
	case "clear-pattern":
		a.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[a.pattern] = model.Pattern{} })
		a.dirty = true
	case "copy-pattern":
		a.synth.Edit(func(e *replay.Engine) {
			if len(e.Project.Song.Patterns) < 240 {
				e.Project.Song.Patterns = append(e.Project.Song.Patterns, e.Project.Song.Patterns[a.pattern])
				a.pattern = len(e.Project.Song.Patterns) - 1
			}
		})
		a.dirty = true
	case "seq:+":
		a.sequence = min(255, a.sequence+1)
	case "seq:-":
		a.sequence = max(0, a.sequence-1)
	case "seq-length":
		e, _ := a.synth.Snapshot()
		a.modal = "Sequence length"
		a.entry = fmt.Sprintf("%02X", e.Project.Bank.Sequences[a.sequence].Length)
	case "seq-repeat":
		e, _ := a.synth.Snapshot()
		a.modal = "Sequence repeat"
		a.entry = fmt.Sprintf("%02X", e.Project.Bank.Sequences[a.sequence].Repeat)
	case "rename-instrument":
		e, _ := a.synth.Snapshot()
		a.modal = "Instrument name"
		a.entry = e.Project.Bank.Instruments[a.instrument].Name()
	case "import-sample":
		a.modal = "Import raw PCM or WAV sample"
		a.entry = ""
	case "clear-sample":
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[a.sample] = model.Sample{} })
		a.dirty = true
	case "order:add":
		a.synth.Edit(func(e *replay.Engine) {
			if e.Project.Song.Length < 255 {
				at := int(e.Project.Song.Length)
				e.Project.Song.Orders[at] = e.Project.Song.Orders[at-1]
				e.Project.Song.Length++
			}
		})
		a.dirty = true
	case "export":
		a.modal = "Export WAV"
		a.entry = filepath.Join(a.directory, "maxymiser.wav")
	default:
		if strings.HasPrefix(name, "setting:") {
			a.modal = "Setting " + strings.TrimPrefix(name, "setting:")
			a.entry = ""
		}
	}
}
func (a *App) save(path string) {
	var err error
	a.synth.Edit(func(e *replay.Engine) { err = project.Save(e.Project, path) })
	if err != nil {
		a.status = err.Error()
		return
	}
	a.projectPath = path
	a.directory = filepath.Dir(path)
	a.dirty = false
	a.lastSave = time.Now()
	a.status = "Saved native song and voice bank"
}
func (a *App) applyModal() {
	modal, entry := a.modal, strings.TrimSpace(a.entry)
	a.modal = ""
	var x, y int
	switch {
	case strings.HasPrefix(modal, "Open music"):
		if strings.EqualFold(filepath.Ext(entry), ".ym") {
			if err := a.LoadYM(entry); err != nil {
				a.status = err.Error()
			}
			return
		}
		a.synth.CloseYM()
		p, err := project.Load(entry, "")
		if err != nil {
			a.status = err.Error()
			return
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = p; e.Reset() })
		a.projectPath = entry
		a.directory = filepath.Dir(entry)
		a.pattern, a.row = 0, 0
		a.dirty = false
		a.status = "Loaded " + filepath.Base(entry)
	case strings.HasPrefix(modal, "Save project"):
		a.save(entry)
	case modal == "Load composer profile (.json)":
		corpus, err := ymimport.LoadCorpus(entry)
		if err != nil {
			a.status = err.Error()
			return
		}
		a.corpus = &corpus
		a.status = fmt.Sprintf("Loaded %s corpus: %d recordings, %d recurring timbres", corpus.Author, corpus.Unique, len(corpus.Instruments))
	case modal == "Instrument name":
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Instruments[a.instrument].SetName(entry) })
		a.dirty = true
	case strings.HasPrefix(modal, "Import raw"):
		var err error
		a.synth.Edit(func(e *replay.Engine) { err = project.ImportSample(&e.Project.Bank, a.sample, entry) })
		if err != nil {
			a.status = err.Error()
		} else {
			a.dirty = true
			a.status = "Sample imported"
		}
	case modal == "Export WAV":
		var err error
		a.synth.Edit(func(e *replay.Engine) { err = export.WAV(e.Project, entry, 30*time.Second) })
		if err != nil {
			a.status = err.Error()
		} else {
			a.status = "Exported 30 seconds of WAV audio"
		}
	case strings.HasPrefix(modal, "Setting "):
		n, err := strconv.Atoi(entry)
		if err != nil {
			a.status = "Enter a decimal number"
			return
		}
		key := strings.TrimPrefix(modal, "Setting ")
		a.synth.Edit(func(e *replay.Engine) {
			switch key {
			case "rate":
				e.Project.Song.SetTickRate(n)
			case "speed":
				e.Project.Song.SetSpeed(n)
				e.Speed = e.Project.Song.Speed()
			case "octave":
				a.octave = max(0, min(8, n))
			case "step":
				a.step = max(0, min(64, n))
			case "master":
				e.MasterVolume = max(0, min(127, n))
			case "timers":
				e.TimerMask = byte(n) & 7
			}
		})
	default:
		n, err := strconv.ParseUint(entry, 16, 16)
		if err != nil {
			a.status = "Enter a hexadecimal number"
			return
		}
		a.synth.Edit(func(e *replay.Engine) {
			switch {
			case modal == "Sequence length":
				e.Project.Bank.Sequences[a.sequence].Length = byte(max(1, min(63, n)))
			case modal == "Sequence repeat":
				e.Project.Bank.Sequences[a.sequence].Repeat = byte(min(62, n))
			case strings.HasPrefix(modal, "Sequence word "):
				fmt.Sscanf(modal, "Sequence word %d", &x)
				e.Project.Bank.Sequences[a.sequence].Values[x] = uint16(n)
			case strings.HasPrefix(modal, "Instrument parameter "):
				fmt.Sscanf(modal, "Instrument parameter %d", &x)
				e.Project.Bank.Instruments[a.instrument][x] = byte(n)
			case strings.HasPrefix(modal, "Order "):
				fmt.Sscanf(modal, "Order %d channel %d", &x, &y)
				e.Project.Song.Orders[x][y] = byte(n)
			}
			e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, a.sequence+1)
		})
		a.dirty = true
	}
}
func (a *App) CaptureState() string {
	e, _ := a.synth.Snapshot()
	return fmt.Sprintf("position=%d row=%d tick=%d", e.Position, e.Row, e.Ticks)
}

// SetTab selects the initial workspace, also used by the capture command.
func (a *App) SetTab(name string) { a.tab = name }

func (a *App) selectChannel(channel int) {
	a.channel = channel
	e, _ := a.synth.Snapshot()
	pattern := int(e.Project.Song.Orders[e.Position][channel])
	if pattern < len(e.Project.Song.Patterns) {
		a.pattern = pattern
	}
}
func (a *App) remember() {
	a.synth.Edit(func(e *replay.Engine) { a.undo = append(a.undo, e.Project.Clone()) })
	if len(a.undo) > 32 {
		a.undo = a.undo[len(a.undo)-32:]
	}
	a.redo = nil
}
func (a *App) restore(redo bool) {
	source, dest := &a.undo, &a.redo
	if redo {
		source, dest = &a.redo, &a.undo
	}
	if len(*source) == 0 {
		return
	}
	value := (*source)[len(*source)-1]
	*source = (*source)[:len(*source)-1]
	a.synth.Edit(func(e *replay.Engine) {
		*dest = append(*dest, e.Project.Clone())
		e.Stop()
		e.Project = value
		e.Reset()
	})
	a.dirty = true
	a.status = "Edit restored"
}

func (a *App) drawDMAPattern(dst *ebiten.Image, e *replay.Engine) {
	p := e.Project
	pattern := a.pattern
	if e.Playing {
		pattern = int(e.Patterns[3])
	}
	a.text(dst, "STe A     NOTE SAMPLE VOL", 94, 248, 13, accent)
	a.text(dst, "STe B     NOTE SAMPLE VOL", 516, 248, 13, purple)
	start := max(0, min(44, a.row-10))
	if e.Playing && !a.editing {
		start = max(0, min(44, e.Row-10))
	}
	for visible := 0; visible < 20; visible++ {
		row := start + visible
		y := 280 + visible*19
		if row%4 == 0 {
			rect(dst, 36, float32(y), 876, 19, color.RGBA{27, 35, 49, 255})
		}
		if row == a.row {
			rect(dst, 36, float32(y), 876, 19, color.RGBA{57, 43, 80, 255})
		}
		a.text(dst, fmt.Sprintf("%02X", row), 42, float64(y+2), 12, dim)
		var c model.Cell
		if pattern < len(p.Song.Patterns) {
			c = p.Song.Patterns[pattern][row]
		}
		a.text(dst, fmt.Sprintf("%s    %s     %s", noteName(c.Note), hexOrDash(c.Instrument), hexOrDash(c.Volume)), 94, float64(y+1), 14, fg)
		a.text(dst, fmt.Sprintf("%s    %s     %s", noteName(c.Effect1), hexOrDash(c.Parameter1), hexOrDash(c.Effect2)), 516, float64(y+1), 14, fg)
		a.buttons = append(a.buttons, button{88, y, 806, 19, fmt.Sprintf("dma-cell:%d", row)})
	}
	a.text(dst, "SAMPLE BANK", 954, 206, 12, dim)
	for i := 0; i < 8; i++ {
		a.btn(dst, fmt.Sprintf("%d   %d bytes", i+1, len(p.Bank.Samples[i].PCM)), 954, 245+i*40, 286, 34, fmt.Sprintf("sample:%d", i), i == a.sample)
	}
	a.text(dst, "Select a sample and enter notes.", 954, 600, 11, dim)
}

func (a *App) LoadYM(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = a.synth.LoadYM(data); err != nil {
		return err
	}
	a.tab = "YM"
	a.ymPath = path
	a.ymReport = nil
	a.status = "YM reference loaded · register stream, not tracker patterns"
	return nil
}
func (a *App) drawYM(dst *ebiten.Image) {
	rect(dst, 24, 192, 1232, 482, panel)
	r, ok := a.synth.Reference()
	if !ok {
		a.text(dst, "Open a .ym file to listen and inspect the YM2149 registers.", 42, 222, 16, fg)
		return
	}
	a.text(dst, r.Name, 42, 211, 22, fg)
	a.text(dst, r.Author+" · "+r.Format, 42, 248, 13, dim)
	a.btn(dst, "Composer profile", 520, 246, 180, 34, "ym:profile", a.corpus != nil)
	a.btn(dst, "Reconstruct", 710, 246, 156, 34, "ym:infer", false)
	a.btn(dst, "Listen YM", 876, 246, 152, 34, "ym:reference", r.Active)
	a.btn(dst, "Listen score", 1038, 246, 194, 34, "ym:score", !r.Active)
	a.text(dst, fmt.Sprintf("%d:%02d / %d:%02d", r.Position/60000, (r.Position/1000)%60, r.Duration/60000, (r.Duration/1000)%60), 968, 217, 18, accent)
	labels := []string{"Tone A low", "Tone A high", "Tone B low", "Tone B high", "Tone C low", "Tone C high", "Noise period", "Mixer", "Volume A", "Volume B", "Volume C", "Envelope low", "Envelope high", "Envelope shape"}
	for reg, label := range labels {
		x := 42 + (reg%2)*590
		y := 298 + (reg/2)*43
		a.text(dst, fmt.Sprintf("R%02d  %s", reg, label), float64(x), float64(y+8), 14, dim)
		a.text(dst, fmt.Sprintf("%02X", r.Registers[reg]), float64(x+262), float64(y+8), 18, accent)
	}
	if a.ymReport != nil {
		label := fmt.Sprintf("Candidate: %d instruments · %d patterns · %d positions", a.ymReport.Instruments, a.ymReport.Patterns, a.ymReport.Positions)
		if a.corpus != nil {
			label += fmt.Sprintf(" · %d corpus matches", len(a.ymReport.Evidence))
		}
		a.text(dst, label, 42, 626, 13, purple)
	}
	a.text(dst, "Reconstruction infers a candidate score; original instrument definitions and pattern boundaries are not stored in YM.", 42, 652, 11, dim)
}
