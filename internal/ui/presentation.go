package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

const PresentationWidth, PresentationHeight, PresentationFPS = 1280, 960, 30

type PresentationConfig struct {
	Native, YM, Workspace, Poster string
	Video, Audio                  io.Writer
	DurationSeconds               int
}

type PresentationChapter struct {
	At    int    `json:"start_seconds"`
	Title string `json:"title"`
	Text  string `json:"caption"`
}

type presentationStep struct {
	PresentationChapter
	action func(*App) error
}

// Presentation records the real tracker controls and its synthesizer on one
// frame clock. Captions occupy a separate panel, leaving every control visible.
// Supplied music and generated media remain external runtime files.
type Presentation struct {
	app           *App
	config        PresentationConfig
	steps         []presentationStep
	frame, step   int
	pixels, pcm   []byte
	caption       PresentationChapter
	failure       error
	posterWritten bool
}

func NewPresentation(c PresentationConfig) (*Presentation, error) {
	if c.Video == nil || c.Audio == nil || c.Workspace == "" || c.Native == "" || c.YM == "" {
		return nil, fmt.Errorf("presentation: music, workspace and output streams are required")
	}
	if c.DurationSeconds == 0 {
		c.DurationSeconds = 180
	}
	if c.DurationSeconds < 1 || c.DurationSeconds > 180 {
		return nil, fmt.Errorf("presentation: duration must be 1–180 seconds")
	}
	if err := os.MkdirAll(c.Workspace, 0755); err != nil {
		return nil, err
	}
	a, err := New(model.Demo(), "", true)
	if err != nil {
		return nil, err
	}
	p := &Presentation{app: a, config: c, pixels: make([]byte, PresentationWidth*PresentationHeight*4), pcm: make([]byte, 48000/PresentationFPS*4)}
	p.steps = presentationSteps(c)
	return p, nil
}

func presentationSteps(c PresentationConfig) []presentationStep {
	var out []presentationStep
	add := func(at int, title, caption string, fn func(*App) error) {
		out = append(out, presentationStep{PresentationChapter{at, title, caption}, fn})
	}
	act := func(name string) func(*App) error { return func(a *App) error { a.action(name); return nil } }
	tab := func(name string) func(*App) error { return act("tab:" + name) }
	modal := func(value string) func(*App) error {
		return func(a *App) error { a.entry = value; a.applyModal(); return nil }
	}
	open := func(path string) func(*App) error {
		return func(a *App) error {
			a.directory = filepath.Dir(path)
			a.action("open")
			a.entry = filepath.Base(path)
			return nil
		}
	}
	add(0, "Go MaxYMiser", "A native-format Atari ST tracker, rebuilt in Go and Ebitengine.", act("play"))
	add(8, "Open a native SNDH", "Load original MaxYMiser notes, instruments, sequences and PCM samples.", open(c.Native))
	add(11, "Native music, editable data", "The same file browser opens native songs and YM recordings.", func(a *App) error {
		a.applyModal()
		if a.modal != "" {
			return fmt.Errorf("native opening failed: %s", a.status)
		}
		return nil
	})
	add(12, "Replay the original score", "Three YM2149 voices and the native PCM track play together.", act("play"))
	add(28, "Build the arrangement", "Four independent pattern lists, song positions and repeat points.", tab("Song"))
	add(36, "Explore the original instruments", "Select a sound to see its masks, parameters and linked sequences.", func(a *App) error { a.tab = "Instruments"; a.SelectInstrument(2); return nil })
	add(42, "Edit a sound while it plays", "Change the bass attenuation; the arrangement keeps its position.", act("parameter:38"))
	add(44, "A quieter bass", "Instrument fields use hexadecimal values. The change is immediately audible.", modal("03"))
	add(48, "Shared volume envelopes", "Instrument links open the actual shared sequence used by the sound bank.", act("instrument-sequence:1"))
	add(53, "Edit individual sequence words", "Shape envelopes, arpeggios, vibrato, noise and waveform control.", act("seq-value:0"))
	add(55, "Hear the changed attack", "The linked instruments use the edited sequence without restarting the song.", modal("000F"))
	add(59, "Undo an edit", "Restore the previous definition with the same undo history as manual editing.", func(a *App) error { a.restore(false); a.action("play"); return nil })
	add(62, "Generate editable sequences", "Ramps, triangles, sine waves and squares share the same sequence editor.", func(a *App) error {
		a.sequenceTools = true
		a.generatorLow = "000F"
		a.generatorHigh = "0002"
		a.action("gen-shape:ramp")
		return nil
	})
	add(66, "Apply a generated envelope", "The generated words replace the selected shared sequence.", act("gen-apply"))
	add(70, "Return to the original envelope", "Undo preserves the original bank while exploring different sounds.", func(a *App) error { a.restore(false); a.action("play"); a.sequenceTools = false; return nil })
	add(73, "Real native PCM samples", "The sample banks come from the loaded SNDH, including their signed waveforms.", func(a *App) error { a.tab = "Samples"; a.sample = 0; return nil })
	add(77, "Preview a sample", "Audition the selected sample independently of song playback.", act("sample-preview"))
	add(80, "Tune the sample", "Adjust its pitch with interpolation; trim and gain tools are available too.", act("sample-tune"))
	add(82, "Transpose the sample by three semitones", "The waveform is edited through the same controls used in the tracker.", modal("3"))
	add(84, "Hear the tuned sample", "Sample editing supports undo and native bank saving.", act("sample-preview"))
	add(87, "Compose a new piece", "Start a blank project with a square-wave instrument and three editable voices.", func(a *App) error {
		a.action("new")
		a.pattern, a.row, a.channel, a.instrument = 0, 0, 0, 0
		a.tab = "Patterns"
		a.editing = true
		a.status = "Enter notes with the two-octave keyboard"
		return nil
	})
	add(89, "Name the composition", "The arrangement and sound bank will be saved as a native MYS/MYV pair.", act("song-info"))
	add(91, "Small beginnings", "An original melody entered into the tracker, one note at a time.", modal("Small beginnings / Malakh Software"))
	for i, note := range []byte{60, 64, 67, 72, 67, 64, 62, 59} {
		i, note := i, note
		add(93+i, "Enter the melody", "Notes, instruments, volume and two effect columns remain directly editable.", func(a *App) error {
			a.channel, a.pattern, a.row, a.instrument = 0, 0, i*4, 0
			a.enterNote(note)
			return nil
		})
	}
	add(103, "Add a second voice", "Independent voices make it easy to build a bass line beside the melody.", func(a *App) error {
		a.channel, a.pattern, a.row = 1, 1, 0
		a.enterNote(48)
		a.row = 16
		a.enterNote(55)
		a.channel, a.pattern, a.row = 0, 0, 0
		a.editing = false
		return nil
	})
	add(106, "Replay the composition", "The sound you hear is rendered from the notes and instruments shown above.", act("play"))
	add(113, "Edit a block of notes", "Copy, transpose, insert rows and remap sounds with selectable column masks.", func(a *App) error { a.tab = "Edit"; a.blockFirst, a.blockLast = 0, 31; return nil })
	add(118, "Save the native song and bank", "Save a working copy while leaving the original reference music untouched.", func(a *App) error {
		a.directory = c.Workspace
		a.action("save-as")
		a.entry = "small-beginnings.mys"
		return nil
	})
	add(121, "Native project saved", "The MYS partition and MYV sound bank can be reopened and edited again.", func(a *App) error {
		a.applyModal()
		if a.dirty {
			return fmt.Errorf("save failed: %s", a.status)
		}
		return nil
	})
	add(124, "Export a WAV", "Offline stereo rendering uses the same score and synthesizer as live replay.", func(a *App) error {
		a.exportDuration = 5 * time.Second
		a.action("export")
		a.entry = "small-beginnings.wav"
		return nil
	})
	add(126, "Rendering the composition", "Editing and playback remain independent of the background WAV export.", func(a *App) error { a.applyModal(); return nil })
	add(130, "Open a YM recording", "Listen to the original soundchip register stream before proposing an editable score.", open(c.YM))
	add(133, "The original YM reference", "Live tone, mixer, volume, noise and envelope registers are visible.", func(a *App) error {
		a.applyModal()
		if _, ok := a.synth.Reference(); !ok {
			return fmt.Errorf("YM opening failed: %s", a.status)
		}
		return nil
	})
	add(142, "Choose a reconstruction range", "A one-frame grid preserves editable tone-period curves when native capacity permits.", act("ym:range"))
	add(144, "Select the first 1,200 frames", "The reference remains separate from the proposed tracker composition.", modal("0,1200,1"))
	add(147, "YM to an editable candidate", "Reconstruction is experimental: original instruments and pattern boundaries are not guaranteed.", func(a *App) error {
		a.action("ym:infer")
		if a.ymReport == nil {
			return fmt.Errorf("reconstruction failed: %s", a.status)
		}
		return nil
	})
	add(154, "Listen to the reconstructed score", "Compare the candidate with the unchanged YM recording.", func(a *App) error { a.action("ym:score"); a.tab = "Patterns"; return nil })
	add(163, "Find recurring melodic passages", "Local phrase IDs describe this analysis, not recovered original pattern numbers.", func(a *App) error { a.tab = "YM"; a.ymPatternView = true; return nil })
	add(168, "Hear a detected phrase in the reference", "Selecting a passage seeks the original YM without rewriting the composition.", func(a *App) error {
		if len(a.ymReport.SourcePatterns) > 0 {
			a.action("ym:pattern-hit:0")
		}
		return nil
	})
	add(173, "Keyboard, MIDI and export settings", "Native configuration, audio controls and macOS MIDI complete the editing workflow.", func(a *App) error { a.ymPatternView = false; a.tab = "Settings"; return nil })
	add(177, "Go MaxYMiser · Development release 0.1.0", "Explore the tracker and its documented compatibility limits on GitHub.", func(a *App) error {
		a.synth.CloseYM()
		a.tab = "Patterns"
		a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.Project = model.Demo(); e.Reset(); e.Play(false) })
		return nil
	})
	return out
}

func (p *Presentation) Update() error {
	if p.failure != nil {
		return p.failure
	}
	if p.frame >= p.config.DurationSeconds*PresentationFPS {
		return ebiten.Termination
	}
	return nil
}

func (p *Presentation) Draw(dst *ebiten.Image) {
	if p.failure != nil || p.frame >= p.config.DurationSeconds*PresentationFPS {
		return
	}
	for p.step < len(p.steps) && p.steps[p.step].At*PresentationFPS <= p.frame {
		s := p.steps[p.step]
		p.caption = s.PresentationChapter
		if s.action != nil {
			if err := s.action(p.app); err != nil {
				p.failure = err
				return
			}
		}
		fmt.Printf("%3ds: %s\n", s.At, s.Title)
		p.step++
	}
	p.app.pollExportResult()
	if _, err := io.ReadFull(p.app.synth, p.pcm); err != nil {
		p.failure = err
		return
	}
	if _, err := p.config.Audio.Write(p.pcm); err != nil {
		p.failure = err
		return
	}
	p.app.Draw(dst)
	rect(dst, 0, 800, 1280, 160, color.RGBA{9, 14, 22, 255})
	rect(dst, 32, 825, 5, 89, accent)
	p.app.text(dst, p.caption.Title, 56, 822, 26, fg)
	p.app.text(dst, p.caption.Text, 56, 864, 15, dim)
	p.app.text(dst, "MALAKH SOFTWARE  ·  REAL TRACKER PLAYBACK", 56, 921, 11, accent)
	p.app.text(dst, fmt.Sprintf("%02d:%02d / 03:00", p.frame/PresentationFPS/60, p.frame/PresentationFPS%60), 1084, 921, 13, dim)
	vector.FillRect(dst, 0, 956, float32(p.frame)*1280/float32(180*PresentationFPS), 4, accent, false)
	dst.ReadPixels(p.pixels)
	if _, err := p.config.Video.Write(p.pixels); err != nil {
		p.failure = err
		return
	}
	if !p.posterWritten && p.frame >= 18*PresentationFPS && p.config.Poster != "" {
		f, err := os.Create(p.config.Poster)
		if err != nil {
			p.failure = err
			return
		}
		img := image.NewRGBA(image.Rect(0, 0, PresentationWidth, PresentationHeight))
		copy(img.Pix, p.pixels)
		err = png.Encode(f, img)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			p.failure = err
			return
		}
		p.posterWritten = true
	}
	p.frame++
}

func (p *Presentation) Layout(_, _ int) (int, int) { return PresentationWidth, PresentationHeight }
func (p *Presentation) Close()                     { p.app.Close() }
func (p *Presentation) Report(path string) error {
	if p.frame != p.config.DurationSeconds*PresentationFPS {
		return fmt.Errorf("presentation: recording ended at frame %d before completion", p.frame)
	}
	chapters := make([]PresentationChapter, 0, len(p.steps))
	for _, s := range p.steps {
		chapters = append(chapters, s.PresentationChapter)
	}
	b, err := json.MarshalIndent(struct {
		Frames, FPS int
		Complete    bool
		Chapters    []PresentationChapter
	}{p.frame, PresentationFPS, p.frame == p.config.DurationSeconds*PresentationFPS, chapters}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
