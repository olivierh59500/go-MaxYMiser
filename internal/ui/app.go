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
	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/midi"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
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
	"unicode/utf8"
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
	ymData                                                             []byte
	exportResults                                                      chan error
	exportDuration                                                     time.Duration
	exporting                                                          bool
	icePacking                                                         bool
	helpTopic                                                          int
	nativeConfiguration                                                native.Configuration
	configurationLoaded                                                bool
	drumKeyboard                                                       bool
	corpus                                                             *ymimport.Corpus
	pairedProfile                                                      *ymimport.PairedProfile
	sourceScore                                                        *ymimport.SourceScore
	sourcePreview                                                      *model.Project
	sourceReport                                                       *ymimport.SourceProjectReport
	sourcePath                                                         string
	sourceConversionError                                              string
	sourcePage                                                         int
	ymPath                                                             string
	ymReport                                                           *ymimport.Report
	ymPatternView                                                      bool
	ymPatternPage                                                      int
	ymOptions                                                          ymimport.ReconstructionOptions
	ymLibrary                                                          ymimport.YMLibrary
	ymLibraryDirectory                                                 string
	ymAlternatives                                                     []ymimport.YMAlternative
	midiInput                                                          *midi.Input
	midiOutput                                                         *midi.Output
	midiDestinations                                                   []midi.Destination
	midiMessages                                                       [256]replay.MIDIMessage
	subtunes                                                           []native.EmbeddedProject
	subtuneIndex                                                       int
	subtuneWorkspaces                                                  []subtuneWorkspace
	collectionSource                                                   []byte
	midiData                                                           chan []byte
	midiDecoder                                                        midi.Decoder
	undo, redo                                                         []*model.Project
	copied                                                             model.Pattern
	hasCopy                                                            bool
	blockClipboard                                                     []model.Cell
	orderClipboard                                                     [][4]byte
	sequenceClipboard                                                  model.Sequence
	hasSequenceClipboard                                               bool
	blockFirst, blockLast                                              int
	pasteMode                                                          edit.PasteMode
	columnMask                                                         edit.ColumnMask
	synth                                                              *replay.Synth
	view                                                               model.Project
	generatorLow, generatorHigh, generatorCycles, morphDestination     string
	generatorShape                                                     edit.Shape
	generatorSigned, sequenceTools                                     bool
	player                                                             *audio.Player
	font                                                               *text.GoTextFaceSource
	projectPath, status, tab                                           string
	buttons                                                            []button
	row, channel, field, nibble, pattern, instrument, sequence, sample int
	octave, step, scroll                                               int
	editing, dirty                                                     bool
	modal, entry                                                       string
	errorDetails                                                       []string
	listed                                                             []fs.DirEntry
	directory                                                          string
	browser                                                            *fileBrowser
	mouseX, mouseY                                                     int
	ctrl                                                               bool
	lastSave                                                           time.Time
}

func New(p *model.Project, projectPath string, mute bool) (*App, error) {
	face, err := text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
	if err != nil {
		return nil, err
	}
	a := &App{synth: replay.NewSynth(replay.New(p), 48000), font: face, projectPath: projectPath, status: "Ready · Space plays · Enter edits · Ctrl+S saves", tab: "Patterns", midiData: make(chan []byte, 64), exportResults: make(chan error, 1), octave: 4, step: 1, directory: "."}
	a.generatorLow, a.generatorHigh, a.generatorCycles = "0000", "000F", "1"
	a.generatorShape, a.morphDestination = edit.Ramp, "03"
	a.exportDuration = 30 * time.Second
	a.ymOptions.FramesPerRow = 1
	a.blockLast, a.pasteMode, a.columnMask = 63, edit.Overwrite, edit.AllColumns
	for channel, id := range p.Song.Orders[0] {
		if int(id) < len(p.Song.Patterns) {
			a.channel, a.pattern = channel, int(id)
			break
		}
	}
	if len(p.ReplaySource) > 0 {
		a.subtunes, _ = native.DecodeContainers(p.ReplaySource)
	}
	if projectPath != "" {
		a.directory = filepath.Dir(projectPath)
		a.status = "Loaded " + filepath.Base(projectPath)
	}
	a.initializeSubtuneWorkspaces()
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
	if a.midiOutput != nil {
		a.synth.EnableMIDIOutput(false)
		a.flushMIDIOutput()
		a.midiOutput.Close()
	}
	if a.midiInput != nil {
		a.midiInput.Close()
	}
	if a.player != nil {
		a.player.Close()
	}
	a.synth.CloseYM()
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
	e, wave := a.synth.SnapshotInto(&a.view)
	p := e.Project
	a.buttons = a.buttons[:0]
	dst.Fill(bg)
	a.text(dst, "MaxYMiser Go", 24, 18, 24, fg)
	a.text(dst, "YM2149 + STe sample tracker", 25, 51, 12, dim)
	a.btn(dst, "New", 510, 20, 64, 36, "new", false)
	a.btn(dst, "Open", 582, 20, 72, 36, "open", false)
	a.btn(dst, "Save", 662, 20, 56, 36, "save", a.dirty)
	a.btn(dst, "As", 726, 20, 32, 36, "save-as", false)
	a.btn(dst, "Song", 764, 20, 78, 36, "play", e.Playing && !e.PatternMode)
	a.btn(dst, "Pattern", 850, 20, 94, 36, "pattern", e.Playing && e.PatternMode)
	a.btn(dst, "Stop", 952, 20, 74, 36, "stop", false)
	a.btn(dst, "Record", 1034, 20, 90, 36, "record", a.editing)
	a.btn(dst, "Export", 1132, 20, 122, 36, "export", false)
	tabs := []string{"Patterns", "Song", "Instruments", "Sequences", "Samples", "Edit", "YM", "Settings", "Help"}
	for i, name := range tabs {
		a.btn(dst, name, 24+i*137, 88, 127, 36, "tab:"+name, a.tab == name)
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
	case "Edit":
		a.drawPatternTools(dst, &e)
	case "YM":
		a.drawYM(dst)
	case "Settings":
		a.drawSettings(dst, &e)
	case "Help":
		a.drawDetailedHelp(dst)
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
		if a.browser != nil {
			a.drawFileBrowser(dst)
		} else {
			a.drawModal(dst)
		}
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
	a.btn(dst, "Rec pattern", 516, 199, 128, 32, "record-pattern", a.editing && e.PatternMode)
	a.btn(dst, "YM", 658, 199, 70, 32, "mode:ym", a.channel < 3)
	a.btn(dst, "DMA", 738, 199, 80, 32, "mode:dma", a.channel == 3)
	a.btn(dst, "Unused", 832, 199, 76, 32, "pattern-unused", false)
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
	start := a.patternViewStart(e)
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
	a.text(dst, "SONG ORDER", 42, 208, 15, fg)
	a.btn(dst, fmt.Sprintf("Length %02X", p.Song.Length), 260, 204, 160, 34, "song-length", false)
	a.btn(dst, fmt.Sprintf("Repeat %02X", p.Song.Repeat), 434, 204, 160, 34, "song-repeat", false)
	a.btn(dst, "Title / artist", 612, 204, 190, 34, "song-info", false)
	a.btn(dst, "Add position", 820, 204, 172, 34, "order:add", false)
	a.btn(dst, "Remove last", 1008, 204, 174, 34, "order:remove", false)
	if e.PositionQueued {
		a.text(dst, fmt.Sprintf("Next %02X", e.NextPosition), 42, 614, 12, accent)
	}
	for i, button := range []struct{ label, action string }{{"Insert", "order:insert"}, {"Delete", "order:delete"}, {"Copy", "order:copy"}, {"Copy range", "order:copy-range"}, {"Paste", "order:paste"}, {"Clone track", "order:clone-pattern"}} {
		a.btn(dst, button.label, 42+i*147, 636, 137, 28, button.action, false)
	}
	start := min(a.scroll, max(0, int(p.Song.Length)-16))
	for pos := start; pos < int(p.Song.Length) && pos < start+16; pos++ {
		y := 252 + (pos-start)*22
		if pos == e.Position {
			rect(dst, 40, float32(y), 1200, 22, color.RGBA{31, 70, 67, 255})
		}
		a.btn(dst, fmt.Sprintf("%02X", pos), 42, y, 80, 22, fmt.Sprintf("position:%d", pos), pos == e.Position)
		for ch := 0; ch < 4; ch++ {
			a.btn(dst, fmt.Sprintf("%02X", p.Song.Orders[pos][ch]), 160+ch*248, y, 206, 22, fmt.Sprintf("order:%d:%d", pos, ch), false)
		}
	}
	if len(a.subtunes) > 1 {
		a.btn(dst, "Export collection", 942, 594, 284, 30, "subtune-export-all", false)
		a.btn(dst, fmt.Sprintf("Subtune %d / %d", a.subtuneIndex+1, len(a.subtunes)), 942, 636, 198, 28, "subtune-select", false)
		a.btn(dst, "Next", 1150, 636, 76, 28, "subtune-next", false)
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
	a.btn(dst, "I", 804, 202, 42, 32, "bank:0", a.instrument < 16)
	a.btn(dst, "II", 852, 202, 42, 32, "bank:1", a.instrument >= 16)
	a.btn(dst, "Load MYI", 902, 202, 104, 34, "instrument-load", false)
	a.btn(dst, "Save MYI", 1014, 202, 104, 34, "instrument-save", false)
	a.btn(dst, "Rename", 1126, 202, 104, 34, "rename-instrument", false)
	a.text(dst, "DIRECT SETTINGS", 320, 246, 10, dim)
	a.btn(dst, "Copy", 670, 240, 98, 26, "instrument-copy", false)
	a.btn(dst, "Preview", 546, 240, 110, 26, "instrument-preview", false)
	a.text(dst, "SOUND SEQUENCES · click to edit", 820, 246, 10, dim)
	maskNames := []string{"Portamento", "Arpeggio", "Vibrato", "Transpose", "Fixed period", "Fixed detune"}
	for bit, label := range []string{"Square", "Buzzer", "Timer"} {
		a.text(dst, label, float64(487+bit*99), 274, 10, dim)
	}
	for index, label := range maskNames {
		y := 299 + index*28
		a.text(dst, label, 320, float64(y+6), 11, dim)
		for component := 0; component < 3; component++ {
			bit := 2 - component
			a.btn(dst, map[bool]string{true: "On", false: "Off"}[inst[16+index]&(1<<bit) != 0], 482+component*99, y, 82, 24, fmt.Sprintf("mask:%d:%d", 16+index, bit), inst[16+index]&(1<<bit) != 0)
		}
	}
	labels := []string{"Sequence speed", "Pulse width", "Envelope shape", "Start sync", "Digi sample", "Digi rate", "Attenuation", "Detune coarse", "Detune fine", "Frequency resolution"}
	offsets := []int{32, 33, 34, 35, 36, 37, 38, 39, 40, 41}
	for i, label := range labels {
		x := 320 + (i%2)*240
		y := 478 + (i/2)*32
		a.text(dst, label, float64(x), float64(y+9), 10, dim)
		a.btn(dst, fmt.Sprintf("%02X", inst[offsets[i]]), x+156, y, 66, 32, fmt.Sprintf("parameter:%d", offsets[i]), false)
	}
	for i, sequence := range instrumentSequences(&p.Bank, a.instrument) {
		y := 270 + i*46
		a.text(dst, sequence.name, 820, float64(y+9), 11, dim)
		a.btn(dst, fmt.Sprintf("%02X", sequence.id), 922, y, 54, 28, fmt.Sprintf("parameter:%d", sequence.offset), false)
		a.btn(dst, sequence.values, 984, y, 246, 28, fmt.Sprintf("instrument-sequence:%d", sequence.id), false)
	}
	a.text(dst, "Sequence IDs are shared across the bank. Editing a sequence affects every linked instrument.", 320, 650, 11, dim)
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
	a.btn(dst, "Generate / morph", 820, 203, 198, 32, "seq-tools", a.sequenceTools)
	a.btn(dst, "Clear", 1032, 203, 96, 32, "seq-clear", false)
	a.btn(dst, "Unused", 1142, 203, 90, 32, "sequence-unused", false)
	for i, item := range []struct{ label, action string }{{"Cut", "sequence-cut"}, {"Copy", "sequence-copy"}, {"Paste", "sequence-paste"}} {
		a.btn(dst, item.label, 42+i*112, 636, 100, 28, item.action, false)
	}
	if a.sequenceTools {
		a.drawSequenceTools(dst, e)
		return
	}
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
	a.btn(dst, "+1.5 dB", 450, 570, 118, 38, "sample-gain:1.5", false)
	a.btn(dst, "−1.5 dB", 580, 570, 118, 38, "sample-gain:-1.5", false)
	a.btn(dst, "Tune", 710, 570, 116, 38, "sample-tune", false)
	a.btn(dst, "Trim", 838, 570, 116, 38, "sample-trim", false)
	a.btn(dst, "Sign / unsign", 966, 570, 160, 38, "sample-sign", false)
	a.btn(dst, "YMise", 1140, 570, 94, 38, "sample-ymise", false)
	a.btn(dst, "Save PCM", 42, 622, 150, 34, "sample-save", false)
	a.btn(dst, "Preview", 208, 622, 128, 34, "sample-preview", false)
	a.text(dst, "Tune: semitones (0.125 = fine step) · Trim: start,length in bytes · Ctrl+Z undoes edits.", 354, 636, 11, dim)
}
func (a *App) drawSettings(dst *ebiten.Image, e *replay.Engine) {
	rect(dst, 24, 192, 1232, 482, panel)
	a.text(dst, "PLAYBACK & EDITING", 42, 210, 16, fg)
	a.btn(dst, "Year", 370, 205, 104, 28, "song-year", false)
	if e.Project.Year != "" {
		a.text(dst, e.Project.Year, 489, 214, 12, accent)
	}
	items := []struct{ label, value, action string }{{"Replay rate", fmt.Sprint(e.Project.Song.TickRate()), "rate"}, {"Ticks per row", fmt.Sprint(e.Speed), "speed"}, {"Keyboard octave", fmt.Sprint(a.octave), "octave"}, {"Edit step", fmt.Sprint(a.step), "step"}, {"Global volume", fmt.Sprint(e.MasterVolume), "master"}, {"Timer allocation", fmt.Sprintf("%X", e.TimerMask), "timers"}}
	for i, v := range items {
		y := 264 + i*57
		a.text(dst, v.label, 48, float64(y+12), 14, dim)
		a.btn(dst, v.value, 370, y, 220, 38, "setting:"+v.action, false)
	}
	a.text(dst, "Audio is rendered at 48 kHz. Sequencing follows the selected replay rate.", 670, 277, 13, fg)
	a.text(dst, "The YM noise generator and envelope are shared between all three voices.", 670, 321, 12, dim)
	a.btn(dst, "Jam mode", 860, 372, 156, 38, "jam", e.Jam)
	clockLabel := "MIDI clock"
	if e.Project.Song.State[31]&3 == 3 {
		clockLabel = "Sync24 unavailable"
	}
	a.btn(dst, clockLabel, 1030, 372, 194, 38, "midi-clock", e.ExternalClock)
	a.btn(dst, "MIDI input", 670, 372, 180, 38, "midi", a.midiInput != nil)
	a.btn(dst, "MIDI output", 670, 236, 180, 30, "midi-output", a.midiOutput != nil)
	a.btn(dst, "Load CNF", 870, 236, 160, 30, "config-load", false)
	a.btn(dst, "Save CNF", 1044, 236, 160, 30, "config-save", false)
	a.btn(dst, "Reload CNF", 1060, 205, 144, 28, "config-reload", a.nativeConfiguration[10] != 0)
	a.btn(dst, "Follow rows", 670, 205, 180, 28, "pattern-scroll", e.Project.Song.State[11] != 0)
	a.btn(dst, "Controllers", 870, 205, 174, 28, "midi-controllers", e.Project.Song.State[31]&4 != 0)
	modes := []string{"Disabled", "One voice", "Two voices", "Native STe rate", "MIDI output"}
	mode := int(e.Project.Song.State[49])
	if mode >= len(modes) {
		mode = 0
	}
	a.text(dst, "PCM mode", 670, 438, 13, dim)
	a.btn(dst, modes[mode], 850, 426, 200, 38, "pcm-mode", false)
	a.btn(dst, fmt.Sprintf("Latency: %d", e.Project.Song.State[57]), 1064, 426, 160, 38, "midi-latency", false)
	a.text(dst, "PCM attenuation limit", 670, 492, 13, dim)
	a.btn(dst, fmt.Sprint(e.Project.Song.State[56]), 950, 480, 180, 38, "setting:pcm-limit", false)
	a.btn(dst, "ICE", 1144, 480, 72, 38, "ice-packing", a.icePacking)
	a.text(dst, "WAV export seconds", 670, 546, 13, dim)
	a.btn(dst, strconv.FormatFloat(a.exportDuration.Seconds(), 'f', -1, 64), 950, 534, 180, 38, "setting:export-duration", false)
	a.btn(dst, "Song", 1144, 534, 72, 38, "export-song-duration", false)
	a.btn(dst, "Load SNDH replay", 670, 588, 250, 36, "sndh-template", len(e.Project.ReplaySource) > 0)
	a.btn(dst, "Export SNDH", 936, 588, 232, 36, "sndh-export", false)
	for track, offset := range []int{40, 41, 42, 43, 51} {
		x := 670 + track*106
		a.btn(dst, fmt.Sprintf("%s %02X", []string{"A", "B", "C", "D", "E"}[track], e.Project.Song.State[offset]&15), x, 636, 94, 28, fmt.Sprintf("midi-channel:%d", track), false)
	}
	for track, offset := range []int{32, 33, 34, 35, 50} {
		if track == 0 {
			a.text(dst, "MIDI sounds · 00 off · DD percussion", 42, 612, 12, dim)
		}
		a.btn(dst, fmt.Sprintf("%s %02X", []string{"A", "B", "C", "D", "E"}[track], e.Project.Song.State[offset]), 42+track*108, 636, 98, 28, fmt.Sprintf("midi-sound:%d", track), false)
	}
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
	if a.modal == "Unable to open this music" {
		for i, message := range a.errorDetails {
			if i < 4 {
				a.text(dst, message, 250, float64(295+i*30), 11, fg)
			}
		}
		if len(a.ymAlternatives) > 0 {
			for i, item := range a.ymAlternatives {
				if i >= 3 {
					break
				}
				a.btn(dst, "YM: "+item.Title, 250, 428+i*32, 600, 28, fmt.Sprintf("ym-alternative:%d", i), false)
			}
		}
		a.btn(dst, "Close", 918, 437, 114, 40, "modal:cancel", true)
		return
	}
	rect(dst, 248, 292, 784, 58, bg)
	a.text(dst, a.entry, 260, 312, 15, accent)
	a.text(dst, "Enter confirms · Escape cancels", 250, 376, 13, dim)
	if a.modal == "MIDI output destination (number)" {
		for i, item := range a.midiDestinations {
			if i >= 4 {
				break
			}
			a.text(dst, fmt.Sprintf("%d  %s", i+1, item.Name), 250, float64(390+i*22), 11, fg)
		}
	}
	a.btn(dst, "Cancel", 800, 437, 100, 40, "modal:cancel", false)
	a.btn(dst, "Apply", 918, 437, 114, 40, "modal:apply", true)
}
func (a *App) Update() error {
	a.flushMIDIOutput()
	select {
	case result := <-a.exportResults:
		a.exporting = false
		if result != nil {
			a.status = result.Error()
		} else {
			a.status = "WAV export complete"
		}
	default:
	}
	_, wheel := ebiten.Wheel()
	if a.browser != nil {
		a.browser.scroll = max(0, min(max(0, len(a.browser.entries)-10), a.browser.scroll-int(wheel)*3))
	}
	if a.tab == "Song" {
		a.scroll = max(0, a.scroll-int(wheel)*3)
	}

	for {
		select {
		case data := <-a.midiData:
			a.midiDecoder.Feed(data, a.receiveMIDI)
		default:
			goto midiDone
		}
	}
midiDone:
	a.mouseX, a.mouseY = ebiten.CursorPosition()
	a.ctrl = ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if a.modal != "" {
		if a.modal == "Unable to open this music" {
			if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				a.modal = ""
			}
		} else {
			chars := ebiten.AppendInputChars(nil)
			a.entry += string(chars)
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(a.entry) > 0 {
				_, size := utf8.DecodeLastRuneInString(a.entry)
				a.entry = a.entry[:len(a.entry)-size]
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				a.modal = ""
				a.browser = nil
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				a.applyModal()
			}
			if a.browser != nil {
				if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
					a.browser.move(1)
					if a.browser.selected >= 0 {
						a.entry = a.browser.entries[a.browser.selected].Name()
					}
				}
				if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
					a.browser.move(-1)
					if a.browser.selected >= 0 {
						a.entry = a.browser.entries[a.browser.selected].Name()
					}
				}
			}
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
	if a.modal == "" && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		for i := len(a.buttons) - 1; i >= 0; i-- {
			b := a.buttons[i]
			if a.mouseX >= b.x && a.mouseX < b.x+b.w && a.mouseY >= b.y && a.mouseY < b.y+b.h {
				a.rightClickAction(b.action)
				break
			}
		}
	}
	if files := ebiten.DroppedFiles(); files != nil {
		if err := a.OpenDroppedMusic(files); err != nil {
			a.ymAlternatives = nil
			a.errorDetails = []string{err.Error(), "The current composition and playback were retained."}
			a.modal, a.entry = "Unable to open this music", ""
			a.status = "Unable to open dropped music"
		}
	}
	return nil
}
func (a *App) keyboard() {
	if (a.tab == "Patterns" || a.tab == "Song") && inpututil.IsKeyJustPressed(ebiten.KeyShiftRight) {
		a.togglePatternRecord()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyControlRight) {
		a.action("pattern")
		return
	}
	if a.ctrl {
		if inpututil.IsKeyJustPressed(ebiten.KeyN) {
			if a.tab == "Sequences" {
				a.action("sequence-unused")
				return
			}
			if a.tab == "Patterns" || a.tab == "Edit" {
				a.action("pattern-unused")
				return
			}
		}
		if a.tab == "Sequences" {
			for key, action := range map[ebiten.Key]string{ebiten.KeyC: "sequence-copy", ebiten.KeyX: "sequence-cut", ebiten.KeyV: "sequence-paste"} {
				if inpututil.IsKeyJustPressed(key) {
					a.sequenceClipboardAction(action)
					return
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			a.moveSongPosition(-1)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			a.moveSongPosition(1)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
			a.restore(false)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			a.restore(true)
			return
		}
		if (a.tab == "Patterns" || a.tab == "Edit") && inpututil.IsKeyJustPressed(ebiten.KeyC) {
			a.patternAction("block-copy")
			return
		}
		if (a.tab == "Patterns" || a.tab == "Edit") && inpututil.IsKeyJustPressed(ebiten.KeyV) {
			a.patternAction("block-paste")
			return
		}
		if (a.tab == "Patterns" || a.tab == "Edit") && inpututil.IsKeyJustPressed(ebiten.KeyX) {
			a.patternAction("block-cut")
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				a.action("save-as")
			} else {
				a.action("save")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			a.action("open")
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			a.synth.Edit(func(e *replay.Engine) { e.Jam = false; e.Project.Song.State[39] = 0 })
			a.dirty = true
		} else {
			a.action("jam")
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		a.action("play")
	}
	if a.tab == "Sequences" {
		for key, action := range map[ebiten.Key]string{ebiten.KeyF3: "sequence-cut", ebiten.KeyF4: "sequence-copy", ebiten.KeyF5: "sequence-paste"} {
			if inpututil.IsKeyJustPressed(key) {
				a.sequenceClipboardAction(action)
				return
			}
		}
	}
	if a.tab != "Patterns" {
		if a.tab == "Instruments" || a.tab == "Sequences" {
			keys := []ebiten.Key{ebiten.KeyZ, ebiten.KeyS, ebiten.KeyX, ebiten.KeyD, ebiten.KeyC, ebiten.KeyV, ebiten.KeyG, ebiten.KeyB, ebiten.KeyH, ebiten.KeyN, ebiten.KeyJ, ebiten.KeyM}
			upper := []ebiten.Key{ebiten.KeyQ, ebiten.KeyDigit2, ebiten.KeyW, ebiten.KeyDigit3, ebiten.KeyE, ebiten.KeyR, ebiten.KeyDigit5, ebiten.KeyT, ebiten.KeyDigit6, ebiten.KeyY, ebiten.KeyDigit7, ebiten.KeyU}
			for index, key := range keys {
				if inpututil.IsKeyJustPressed(key) {
					a.AuditionInstrument(byte(12 + a.octave*12 + index))
				}
			}
			for index, key := range upper {
				if inpututil.IsKeyJustPressed(key) {
					a.AuditionInstrument(byte(24 + a.octave*12 + index))
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyCapsLock) {
				a.AuditionInstrument(1)
			}
		}
		return
	}
	for octave, key := range []ebiten.Key{ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8} {
		if inpututil.IsKeyJustPressed(key) {
			a.octave = octave
			a.drumKeyboard = false
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		a.drumKeyboard = !a.drumKeyboard
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyInsert) {
		a.patternAction("row-insert")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		a.patternAction("row-delete")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		a.editing = !a.editing
	}
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	if shift && inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		a.moveLivePattern(-1)
		return
	}
	if shift && inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		a.moveLivePattern(1)
		return
	}
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
	if a.field > 0 && a.editing && !(a.channel == 3 && a.field == 3) {
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
			if a.drumKeyboard && note > 1 {
				a.instrument = int(note-12) % 32
				note = 60
			}
			e.Trigger(a.channel, note, byte(a.instrument+1))
		} else {
			dmaChannel := 0
			if a.field >= 3 {
				dmaChannel = 1
			}
			e.TriggerSample(dmaChannel, note, byte(a.sample+1))
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
	if a.channel < 3 && (a.field == 3 || a.field == 5) {
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
		case 5:
			if a.channel == 3 {
				target = &c.Effect2
			}
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
		if a.field == 2 || a.channel == 3 && a.field == 5 {
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
	if a.modal == "" && a.unusedAction(name) {
		return
	}
	if a.modal == "" && a.transportStartAction(name) {
		return
	}
	if a.modal == "" && a.sourceAction(name) {
		return
	}
	if a.modal == "" && a.midiClockAction(name) {
		return
	}
	if a.modal == "" && a.songDurationAction(name) {
		return
	}
	if a.modal == "" && a.yearAction(name) {
		return
	}
	if a.modal == "" && a.sequenceClipboardAction(name) {
		return
	}
	if a.modal == "" && a.arrangementAction(name) {
		return
	}
	if a.modal == "" && a.subtuneAction(name) {
		return
	}
	if a.modal == "" && a.midiAssignmentAction(name) {
		return
	}
	if a.ymPatternAction(name) {
		return
	}
	if a.alternativeAction(name) {
		return
	}
	if a.modal == "" && a.configurationAction(name) {
		return
	}
	if a.modal == "" && a.helpAction(name) {
		return
	}
	if a.modal == "" && a.midiOutputAction(name) {
		return
	}
	if a.fileAction(name) {
		return
	}
	if a.patternAction(name) {
		return
	}
	if a.soundAction(name) {
		return
	}
	if strings.HasPrefix(name, "tab:") {
		a.tab = strings.TrimPrefix(name, "tab:")
		return
	}
	if strings.HasPrefix(name, "modal:") {
		if name == "modal:apply" {
			a.applyModal()
		} else {
			a.modal = ""
			a.browser = nil
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
		a.SelectInstrument(x)
		return
	}
	if _, err := fmt.Sscanf(name, "instrument-sequence:%d", &x); err == nil {
		a.sequence = max(0, min(255, x))
		a.tab = "Sequences"
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
	if _, err := fmt.Sscanf(name, "mask:%d:%d", &x, &y); err == nil {
		if x >= 16 && x <= 21 && y >= 0 && y <= 2 {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) {
				e.Project.Bank.Instruments[a.instrument][x] ^= 1 << y
				e.RefreshInstrumentParameter(a.instrument, x)
			})
			a.dirty = true
		}
		return
	}
	if _, err := fmt.Sscanf(name, "mute:%d", &x); err == nil {
		a.synth.Edit(func(e *replay.Engine) { e.Mutes ^= 1 << x; e.Project.Song.State[37] = e.Mutes })
		return
	}
	if _, err := fmt.Sscanf(name, "midi-channel:%d", &x); err == nil {
		if x >= 0 && x < 5 {
			e, _ := a.synth.Snapshot()
			a.modal, a.entry = fmt.Sprintf("MIDI track %d channel", x), fmt.Sprintf("%X", e.Project.Song.State[[]int{40, 41, 42, 43, 51}[x]])
		}
		return
	}
	if _, err := fmt.Sscanf(name, "order:%d:%d", &x, &y); err == nil {
		e, _ := a.synth.Snapshot()
		a.modal = fmt.Sprintf("Order %d channel %d", x, y)
		a.entry = fmt.Sprintf("%02X", e.Project.Song.Orders[x][y])
		return
	}
	if _, err := fmt.Sscanf(name, "position:%d", &x); err == nil {
		queued := false
		a.synth.Edit(func(e *replay.Engine) {
			e.SelectPosition(x)
			queued = e.PositionQueued
		})
		if queued {
			a.status = fmt.Sprintf("Jam: position %02X queued for the next pattern boundary", x)
		} else {
			a.selectChannel(a.channel)
		}
		return
	}
	switch name {
	case "ym:infer":
		data, err := a.ymData, error(nil)
		if err != nil {
			a.status = err.Error()
			return
		}
		trace, err := ymimport.Decode(data)
		if err != nil {
			a.status = err.Error()
			return
		}
		candidate, report, err := ymimport.ReconstructSelection(trace, a.ymOptions)
		if err != nil {
			a.status = err.Error()
			return
		}
		if a.corpus != nil {
			report.AuthorProfile = a.corpus.Author
			report.Evidence = a.corpus.Evidence(trace)
		}
		if a.pairedProfile != nil {
			if err := ymimport.ApplyPairedRecipes(candidate, &report, trace, *a.pairedProfile); err != nil {
				a.status = err.Error()
				return
			}
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project = candidate; e.Reset() })
		a.ymReport = &report
		a.pattern, a.row, a.channel = 0, 0, 0
		a.projectPath = ""
		a.dirty = true
		a.status = fmt.Sprintf("Reconstructed candidate: %d instruments, %d patterns. Compare with the original YM.", report.Instruments, report.Patterns)
	case "ym:profile":
		a.beginFileBrowser("Load composer profile (.json)", "", false)
	case "ym:paired-profile":
		a.beginFileBrowser("Load paired source profile (.json)", "", false)
	case "ym-library":
		a.modal, a.entry = "YM library directory", a.ymLibraryDirectory
	case "ym:range":
		a.modal, a.entry = "YM range (first,last,row frames)", fmt.Sprintf("%d,%d,%d", a.ymOptions.StartFrame, a.ymOptions.EndFrame, a.ymOptions.FramesPerRow)
	case "ym:reference":
		a.synth.SelectReference(true)
	case "ym:score":
		a.synth.SelectReference(false)
	case "jam":
		a.synth.Edit(func(e *replay.Engine) {
			e.Jam = !e.Jam
			e.Project.Song.State[39] = 0
			if e.Jam {
				e.Project.Song.State[39] = 255
			}
		})
		a.dirty = true
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
	case "pcm-mode":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Song.State[49] = (e.Project.Song.State[49] + 1) % 5 })
		a.dirty = true
	case "midi-clock":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			e.Stop()
			e.Project.Song.SetSpeed(6)
			e.Speed = 6
			e.ExternalClock = e.Project.Song.State[31]&3 != 1
			if e.ExternalClock {
				e.Project.Song.State[31] |= 1
				e.Project.Song.State[31] &^= 2
			} else {
				e.Project.Song.State[31] &^= 1
			}
		})
		a.dirty = true
	case "midi-controllers":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Song.State[31] ^= 4 })
		a.dirty = true
	case "bank:0":
		a.instrument = a.instrument % 16
	case "bank:1":
		a.instrument = 16 + a.instrument%16
	case "new":
		a.collectionSource = nil
		a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
		a.sourcePath = ""
		a.sourceConversionError = ""
		a.subtunes = nil
		a.subtuneIndex = 0
		if _, ok := a.synth.Reference(); ok {
			a.synth.SelectReference(false)
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = model.New(); e.Reset() })
		a.projectPath = ""
		a.dirty = false
		a.status = "New project"
	case "instrument-preview":
		a.AuditionInstrument(48)
	case "open":
		a.beginFileBrowser("Open music (.mys / .myv / .snd / .ym)", "", false)
	case "subtune-next":
		if len(a.subtunes) > 1 {
			if err := a.SelectSubtune((a.subtuneIndex + 1) % len(a.subtunes)); err != nil {
				a.status = err.Error()
			}
		}
	case "save":
		if a.projectPath == "" {
			name := "untitled.mys"
			if len(a.subtunes) > 1 {
				name = fmt.Sprintf("subtune-%02d.mys", a.subtuneIndex+1)
			}
			a.beginFileBrowser("Save project (.mys + .myv)", name, true)
		} else {
			a.save(a.projectPath)
		}
	case "save-as":
		name := "untitled.mys"
		if a.projectPath != "" {
			name = filepath.Base(a.projectPath)
		} else if len(a.subtunes) > 1 {
			name = fmt.Sprintf("subtune-%02d.mys", a.subtuneIndex+1)
		}
		a.beginFileBrowser("Save project as (.mys + .myv)", name, true)
	case "play":
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.ToggleYM()
			return
		}
		a.synth.Edit(func(e *replay.Engine) {
			if e.Playing {
				if e.PatternMode {
					e.PatternMode = false
				} else {
					e.Stop()
				}
			} else {
				e.Play(false)
			}
		})
	case "pattern":
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.SelectReference(false)
		}
		a.synth.Edit(func(e *replay.Engine) {
			if e.Playing {
				e.PatternMode = true
			} else {
				e.Patterns[a.channel] = byte(a.pattern)
				e.Play(true)
			}
		})
	case "stop":
		a.editing = false
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.StopYM()
			return
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop() })
	case "record":
		a.editing = true
		if r, ok := a.synth.Reference(); ok && r.Active {
			a.synth.SelectReference(false)
		}
		a.synth.Edit(func(e *replay.Engine) {
			if e.Playing {
				e.PatternMode = false
			} else {
				e.Play(false)
			}
		})
		a.status = "Recording notes into the playing native pattern"
	case "record-pattern":
		a.recordPattern()
	case "pattern-scroll":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			if e.Project.Song.State[11] == 0 {
				e.Project.Song.State[11] = 255
			} else {
				e.Project.Song.State[11] = 0
			}
		})
		a.dirty = true
	case "pat:+":
		a.nextEditablePattern(1)
	case "pat:-":
		a.nextEditablePattern(-1)
	case "clear-pattern":
		e, _ := a.synth.Snapshot()
		if a.pattern < 0 || a.pattern >= len(e.Project.Song.Patterns) {
			a.status = "Select an ordinary stored pattern first"
			return
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Song.Patterns[a.pattern] = model.Pattern{} })
		a.dirty = true
	case "copy-pattern":
		e, _ := a.synth.Snapshot()
		if a.pattern < 0 || a.pattern >= len(e.Project.Song.Patterns) || len(e.Project.Song.Patterns) >= model.MaxPatterns {
			a.status = "Select a stored pattern and leave room for its copy"
			return
		}
		a.remember()
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
		a.beginFileBrowser("Import raw PCM or WAV sample", "", false)
	case "clear-sample":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[a.sample] = model.Sample{} })
		a.dirty = true
	case "song-length", "song-repeat":
		e, _ := a.synth.Snapshot()
		if name == "song-length" {
			a.modal, a.entry = "Song length", fmt.Sprintf("%02X", e.Project.Song.Length)
		} else {
			a.modal, a.entry = "Song repeat", fmt.Sprintf("%02X", e.Project.Song.Repeat)
		}
	case "song-info":
		e, _ := a.synth.Snapshot()
		a.modal, a.entry = "Song title / artist", e.Project.Title+" / "+e.Project.Author
	case "export":
		a.beginFileBrowser("Export WAV", "maxymiser.wav", true)
	case "sndh-template":
		a.beginFileBrowser("Load SNDH replay template", "", false)
	case "sndh-export":
		a.beginFileBrowser("Export native SNDH", "maxymiser.snd", true)
	case "ice-packing":
		a.icePacking = !a.icePacking
	default:
		if strings.HasPrefix(name, "setting:") {
			a.modal = "Setting " + strings.TrimPrefix(name, "setting:")
			a.entry = ""
		}
	}
}
func (a *App) save(path string) {
	var err error
	var snapshot *model.Project
	a.synth.Edit(func(e *replay.Engine) { snapshot = e.Project.Clone() })
	err = project.SavePacked(snapshot, path, a.icePacking)
	if err != nil {
		a.status = err.Error()
		return
	}
	a.projectPath = path
	a.directory = filepath.Dir(path)
	a.dirty = false
	a.lastSave = time.Now()
	a.status = "Saved native song and voice bank"
	if strings.EqualFold(filepath.Ext(path), ".snd") || strings.EqualFold(filepath.Ext(path), ".sndh") {
		a.status = "Saved native SNDH song"
	}
}
func (a *App) applyModal() {
	if a.browser != nil {
		if !a.browserConfirm() {
			return
		}
	}
	modal, entry := a.modal, strings.TrimSpace(a.entry)
	a.modal = ""
	if a.sourceModal(modal, entry) {
		return
	}
	if a.midiClockModal(modal, entry) {
		return
	}
	if a.yearModal(modal, entry) {
		return
	}
	if a.arrangementModal(modal, entry) {
		return
	}
	if a.subtuneModal(modal, entry) {
		return
	}
	if a.midiAssignmentModal(modal, entry) {
		return
	}
	if a.libraryModal(modal, entry) {
		return
	}
	if a.configurationModal(modal, entry) {
		return
	}
	if a.midiOutputModal(modal, entry) {
		return
	}
	if a.patternModal(modal, entry) {
		return
	}
	if a.soundModal(modal, entry) {
		return
	}
	var x, y int
	switch {
	case modal == "YM range (first,last,row frames)":
		parts := strings.Split(entry, ",")
		if len(parts) != 3 {
			a.status = "Enter decimal first,last,row frames (last 0 = end; row 0 = estimate)"
			return
		}
		first, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		last, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		grid, e3 := strconv.Atoi(strings.TrimSpace(parts[2]))
		if e1 != nil || e2 != nil || e3 != nil || first < 0 || last < 0 || grid < 0 || grid > 16 {
			a.status = "Invalid YM range/grid values"
			return
		}
		a.ymOptions = ymimport.ReconstructionOptions{StartFrame: first, EndFrame: last, FramesPerRow: grid}
		a.status = "YM reconstruction selection updated; reference retained"
	case modal == "Load SNDH replay template":
		raw, err := os.ReadFile(entry)
		if err == nil {
			_, err = native.ParseSNDHTemplate(raw)
		}
		if err != nil {
			a.status = err.Error()
		} else {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) { e.Project.ReplaySource = append([]byte(nil), raw...) })
			a.status = "Native SNDH replay loaded; current composition retained"
		}
	case modal == "Export native SNDH":
		var snapshot *model.Project
		a.synth.Edit(func(e *replay.Engine) { snapshot = e.Project.Clone() })
		if err := project.SaveSNDHPacked(snapshot, entry, a.exportDuration, a.icePacking); err != nil {
			a.status = err.Error()
		} else {
			a.status = "Native SNDH exported with track duration"
		}
	case strings.HasPrefix(modal, "Open music"):
		if err := a.OpenMusic(entry); err != nil {
			a.showOpenError(entry, err)
		}
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
	case modal == "Load paired source profile (.json)":
		if err := a.LoadPairedProfile(entry); err != nil {
			a.status = err.Error()
		}
	case modal == "Instrument name":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Instruments[a.instrument].SetName(entry) })
		a.dirty = true
	case modal == "Song title / artist":
		a.remember()
		parts := strings.SplitN(entry, "/", 2)
		a.synth.Edit(func(e *replay.Engine) {
			e.Project.Title = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				e.Project.Author = strings.TrimSpace(parts[1])
			}
		})
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
		if a.exporting {
			a.status = "A WAV export is already running"
			return
		}
		var snapshot *model.Project
		a.synth.Edit(func(e *replay.Engine) { snapshot = e.Project.Clone() })
		a.status = "Rendering WAV audio…"
		reference, active := a.synth.Reference()
		raw := append([]byte(nil), a.ymData...)
		duration := a.exportDuration
		a.exporting = true
		go func() {
			if active && reference.Active {
				a.exportResults <- export.YM(raw, entry, duration)
			} else {
				a.exportResults <- export.WAV(snapshot, entry, duration)
			}
		}()
	case strings.HasPrefix(modal, "Setting "):
		if modal == "Setting export-duration" {
			seconds, err := strconv.ParseFloat(entry, 64)
			if err != nil || seconds <= 0 || seconds > 3600 {
				a.status = "Enter an export duration from greater than 0 to 3600 seconds"
			} else {
				a.exportDuration = time.Duration(seconds * float64(time.Second))
			}
			return
		}
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
				if e.ExternalClock {
					a.status = "Select internal clock to edit row speed; external selection uses six pulses"
					return
				}
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
				e.Project.Song.State[36] = e.TimerMask
			case "pcm-limit":
				e.Project.Song.State[56] = byte(max(0, min(7, n)))
			}
		})
	default:
		n, err := strconv.ParseUint(entry, 16, 16)
		if err != nil {
			a.status = "Enter a hexadecimal number"
			return
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			switch {
			case strings.HasPrefix(modal, "MIDI track "):
				fmt.Sscanf(modal, "MIDI track %d channel", &x)
				if x >= 0 && x < 5 {
					e.Project.Song.State[[]int{40, 41, 42, 43, 51}[x]] = byte(n) & 15
				}
			case modal == "Song length":
				length := max(1, min(255, int(n)))
				for at := int(e.Project.Song.Length); at < length; at++ {
					e.Project.Song.Orders[at] = e.Project.Song.Orders[at-1]
				}
				e.Project.Song.Length = byte(length)
				e.Project.Song.Repeat = min(e.Project.Song.Repeat, byte(length-1))
				e.Position = min(e.Position, length-1)
			case modal == "Song repeat":
				e.Project.Song.Repeat = byte(min(int(n), int(e.Project.Song.Length)-1))
			case modal == "Sequence length":
				e.Project.Bank.Sequences[a.sequence].Length = byte(max(1, min(63, n)))
				e.Project.Bank.Sequences[a.sequence].Repeat = min(e.Project.Bank.Sequences[a.sequence].Repeat, e.Project.Bank.Sequences[a.sequence].Length-1)
				e.RefreshSequence(a.sequence)
			case modal == "Sequence repeat":
				e.Project.Bank.Sequences[a.sequence].Repeat = byte(min(int(n), max(0, int(e.Project.Bank.Sequences[a.sequence].Length)-1)))
				e.RefreshSequence(a.sequence)
			case strings.HasPrefix(modal, "Sequence word "):
				fmt.Sscanf(modal, "Sequence word %d", &x)
				e.Project.Bank.Sequences[a.sequence].Values[x] = uint16(n)
				e.RefreshSequence(a.sequence)
			case strings.HasPrefix(modal, "Instrument parameter "):
				fmt.Sscanf(modal, "Instrument parameter %d", &x)
				e.Project.Bank.Instruments[a.instrument][x] = byte(n)
				e.RefreshInstrumentParameter(a.instrument, x)
			case strings.HasPrefix(modal, "Order "):
				fmt.Sscanf(modal, "Order %d channel %d", &x, &y)
				if n >= 240 && n != 253 && n != 254 && n != 255 {
					a.status = "Use 00–EF for patterns, FD for a jam loop, FE for note-off, or FF for empty"
					return
				}
				for n < 240 && len(e.Project.Song.Patterns) <= int(n) {
					e.Project.Song.Patterns = append(e.Project.Song.Patterns, model.Pattern{})
				}
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

// SetSequenceTools selects the generation workspace for interface captures.
func (a *App) SetSequenceTools(enabled bool) { a.sequenceTools = enabled }

// ShowFileBrowser opens the native file chooser for captures and initial views.
func (a *App) ShowFileBrowser() {
	a.beginFileBrowser("Open music (.mys / .myv / .snd / .ym)", "", false)
}

// SelectInstrument selects the editable definition, not a channel's cached
// playback parameters. Linked sequences are read from the same voice bank.
func (a *App) SelectInstrument(index int) {
	a.instrument = max(0, min(model.MaxInstruments-1, index))
	e, _ := a.synth.Snapshot()
	a.status = fmt.Sprintf("Instrument %02X · %s · click a sound sequence to edit it", a.instrument+1, e.Project.Bank.Instruments[a.instrument].Name())
}

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
	start := a.patternViewStart(e)
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
	if err = a.loadYMBytes(data); err != nil {
		return err
	}
	a.ymPath = path
	return nil
}
func (a *App) loadYMBytes(data []byte) error {
	if err := a.synth.LoadYM(data); err != nil {
		return err
	}
	a.sourceScore, a.sourcePreview, a.sourceReport = nil, nil, nil
	a.sourcePath = ""
	a.sourceConversionError = ""
	a.ymData = append([]byte(nil), data...)
	if strings.EqualFold(filepath.Ext(a.projectPath), ".ym") {
		a.projectPath = ""
	}
	a.tab = "YM"
	a.ymReport = nil
	a.ymOptions = ymimport.ReconstructionOptions{FramesPerRow: 1}
	a.status = "YM reference loaded · register stream, not tracker patterns"
	return nil
}
func (a *App) drawYM(dst *ebiten.Image) {
	rect(dst, 24, 192, 1232, 482, panel)
	if a.sourceScore != nil && a.sourceReport != nil {
		a.drawSource(dst)
		return
	}
	r, ok := a.synth.Reference()
	if !ok {
		a.text(dst, "Open a .ym file to listen and inspect the YM2149 registers.", 42, 222, 16, fg)
		a.btn(dst, "YM library", 42, 268, 154, 34, "ym-library", len(a.ymLibrary.Files) > 0)
		a.btn(dst, "Paired source profile", 214, 268, 218, 34, "ym:paired-profile", a.pairedProfile != nil)
		return
	}
	a.text(dst, r.Name, 42, 211, 22, fg)
	a.text(dst, r.Author+" · "+r.Format, 42, 248, 13, dim)
	a.btn(dst, "Composer profile", 520, 246, 180, 34, "ym:profile", a.corpus != nil)
	a.btn(dst, "YM library", 350, 246, 154, 34, "ym-library", len(a.ymLibrary.Files) > 0)
	a.btn(dst, "Reconstruct", 710, 246, 156, 34, "ym:infer", false)
	a.btn(dst, "Listen YM", 876, 246, 152, 34, "ym:reference", r.Active)
	a.btn(dst, "Listen score", 1038, 246, 194, 34, "ym:score", !r.Active)
	a.btn(dst, fmt.Sprintf("Range %d:%d · grid %d", a.ymOptions.StartFrame, a.ymOptions.EndFrame, a.ymOptions.FramesPerRow), 520, 205, 344, 30, "ym:range", false)
	a.btn(dst, "Paired profile", 350, 205, 154, 30, "ym:paired-profile", a.pairedProfile != nil)
	a.btn(dst, "Patterns", 214, 205, 120, 30, "ym:source-patterns", a.ymPatternView)
	a.text(dst, fmt.Sprintf("%d:%02d / %d:%02d", r.Position/60000, (r.Position/1000)%60, r.Duration/60000, (r.Duration/1000)%60), 968, 217, 18, accent)
	if a.ymPatternView {
		a.drawYMPatterns(dst)
		return
	}
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
		if a.pairedProfile != nil {
			label += fmt.Sprintf(" · %d source labels · %d improved passages", len(a.ymReport.SourceLabels), len(a.ymReport.RecipeApplications))
		}
		a.text(dst, label, 42, 626, 13, purple)
	}
	if a.pairedProfile != nil && a.ymReport != nil {
		frame := int(r.Position) * a.ymReport.SourceLabelRate / 1000
		labels := [3]string{"?", "?", "?"}
		for _, evidence := range a.ymReport.SourceLabels {
			if evidence.Start <= frame && frame < evidence.End && evidence.Channel >= 0 && evidence.Channel < 3 && r.Active && r.Registers[8+evidence.Channel]&31 != 0 {
				labels[evidence.Channel] = fmt.Sprintf("%02X", evidence.Instrument)
			}
		}
		a.text(dst, fmt.Sprintf("Paired source IDs: A %s · B %s · C %s · ? = unresolved", labels[0], labels[1], labels[2]), 42, 599, 12, accent)
	}
	a.text(dst, "Reconstruction infers a candidate score; original instrument definitions and pattern boundaries are not stored in YM.", 42, 652, 11, dim)
}
