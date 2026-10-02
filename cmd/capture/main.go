// Command capture saves a tracker view for visual regression checks.
package main

import (
	"flag"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/ui"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
	"image"
	"image/png"
	"log"
	"os"
)

type capture struct {
	app     *ui.App
	output  string
	written bool
	ticks   int
}

func (c *capture) Update() error {
	c.ticks++
	if c.written && c.ticks > 8 {
		return ebiten.Termination
	}
	return c.app.Update()
}
func (c *capture) Draw(screen *ebiten.Image) {
	c.app.Draw(screen)
	if c.written {
		return
	}
	b := screen.Bounds()
	rgba := image.NewRGBA(b)
	screen.ReadPixels(rgba.Pix)
	f, e := os.Create(c.output)
	if e != nil {
		log.Fatal(e)
	}
	if e = png.Encode(f, rgba); e != nil {
		log.Fatal(e)
	}
	f.Close()
	c.written = true
}
func (c *capture) Layout(w, h int) (int, int) { return c.app.Layout(w, h) }
func main() {
	output := flag.String("output", "captures/tracker.png", "PNG output")
	tab := flag.String("tab", "Patterns", "tracker view")
	instrument := flag.Int("instrument", 1, "selected instrument, 1 through 32")
	tools := flag.Bool("sequence-tools", false, "show the sequence generator")
	browser := flag.Bool("file-browser", false, "show the file browser")
	song := flag.String("song", "", "open a music file through the GUI workflow before capture")
	paired := flag.String("paired-profile", "", "source-labelled profile for the YM workspace")
	reconstruct := flag.Bool("reconstruct", false, "apply the YM reconstruction workflow before capture")
	end := flag.Int("end-frame", 0, "exclusive reconstruction end frame")
	flag.Parse()
	app, e := ui.New(model.Demo(), "", true)
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	app.SetTab(*tab)
	app.SelectInstrument(*instrument - 1)
	app.SetSequenceTools(*tools)
	if *song != "" {
		if e = app.OpenMusic(*song); e != nil {
			log.Fatal(e)
		}
	}
	if *paired != "" {
		if e = app.LoadPairedProfile(*paired); e != nil {
			log.Fatal(e)
		}
	}
	if *reconstruct {
		app.ReconstructYM(ymimport.ReconstructionOptions{EndFrame: *end, FramesPerRow: 1})
	}
	app.SetTab(*tab)
	if *browser {
		app.ShowFileBrowser()
	}
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowTitle("MaxYMiser Go — interface capture")
	if e = ebiten.RunGame(&capture{app: app, output: *output}); e != nil {
		log.Fatal(e)
	}
}
