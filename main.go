// Command go-MaxYMiser starts the tracker or renders a native song to WAV.
package main

import (
	"flag"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/ui"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	song := flag.String("song", "", "native MYS file")
	bank := flag.String("bank", "", "native MYV voice bank")
	wav := flag.String("wav", "", "render to a new WAV file")
	duration := flag.Duration("duration", 30*time.Second, "WAV duration")
	mute := flag.Bool("mute", false, "disable audio device")
	info := flag.Bool("info", false, "print native project information")
	flag.Parse()
	if *song == "" && flag.NArg() > 0 {
		*song = flag.Arg(0)
	}
	p := model.Demo()
	var err error
	if *song != "" && strings.EqualFold(filepath.Ext(*song), ".ym") {
	} else if *song != "" || *bank != "" {
		p, err = project.Load(*song, *bank)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *info {
		fmt.Printf("%s: %d positions, %d patterns, %d sequences, %d Hz, speed %d\n", p.Title, p.Song.Length, len(p.Song.Patterns), p.Bank.SequenceCount, p.Song.TickRate(), p.Song.Speed())
		return
	}
	if *wav != "" {
		if err = export.WAV(p, *wav, *duration); err != nil {
			log.Fatal(err)
		}
		return
	}
	app, err := ui.New(p, *song, *mute)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer app.Close()
	if *song != "" && strings.EqualFold(filepath.Ext(*song), ".ym") {
		if err = app.LoadYM(*song); err != nil {
			log.Fatal(err)
		}
	}
	ebiten.SetWindowTitle("MaxYMiser Go — YM2149 tracker")
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	if err = ebiten.RunGame(app); err != nil {
		log.Fatal(err)
	}
}
