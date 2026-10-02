// Command capture saves a tracker view for visual regression checks.
package main

import (
	"flag"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/ui"
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
	flag.Parse()
	app, e := ui.New(model.Demo(), "", true)
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	app.SetTab(*tab)
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowTitle("MaxYMiser Go — interface capture")
	if e = ebiten.RunGame(&capture{app: app, output: *output}); e != nil {
		log.Fatal(e)
	}
}
