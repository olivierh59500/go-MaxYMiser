// Command go-MaxYMiser starts the tracker or renders a native song to WAV.
package main

import (
	"flag"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
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
	config := flag.String("config", "", "native MYM.CNF configuration")
	ymLibrary := flag.String("ym-library", "", "directory containing YM recordings for SNDH alternatives")
	pairedProfile := flag.String("paired-profile", "", "verified source-labelled SNDH/YM profile JSON")
	flag.Parse()
	if *song == "" && flag.NArg() > 0 {
		*song = flag.Arg(0)
	}
	p := model.Demo()
	var err error
	inspectSourceAtStartup := false
	isYM := *song != "" && strings.EqualFold(filepath.Ext(*song), ".ym")
	if !isYM && (*song != "" || *bank != "") {
		p, err = project.Load(*song, *bank)
		if err != nil {
			if !*info && *wav == "" && *bank == "" && (strings.EqualFold(filepath.Ext(*song), ".sndh") || strings.EqualFold(filepath.Ext(*song), ".snd")) {
				p, inspectSourceAtStartup = model.Demo(), true
			} else {
				log.Fatal(err)
			}
		}
	}
	if *info {
		if isYM {
			raw, e := os.ReadFile(*song)
			if e != nil {
				log.Fatal(e)
			}
			synth := replay.NewSynth(replay.New(model.New()), 48000)
			if e = synth.LoadYM(raw); e != nil {
				log.Fatal(e)
			}
			r, _ := synth.Reference()
			synth.CloseYM()
			fmt.Printf("%s: %s, %s, %.2f seconds; original register recording\n", filepath.Base(*song), r.Name, r.Format, float64(r.Duration)/1000)
		} else {
			fmt.Printf("%s: %d positions, %d patterns, %d sequences, %d Hz, speed %d\n", p.Title, p.Song.Length, len(p.Song.Patterns), p.Bank.SequenceCount, p.Song.TickRate(), p.Song.Speed())
		}
		return
	}
	if *wav != "" {
		if isYM {
			raw, e := os.ReadFile(*song)
			if e != nil {
				log.Fatal(e)
			}
			err = export.YM(raw, *wav, *duration)
		} else {
			err = export.WAV(p, *wav, *duration)
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	projectPath := *song
	if inspectSourceAtStartup {
		projectPath = ""
	}
	app, err := ui.New(p, projectPath, *mute)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer app.Close()
	if inspectSourceAtStartup {
		if err := app.OpenMusic(*song); err != nil {
			log.Fatal(err)
		}
	}
	if *ymLibrary != "" {
		if err = app.SetYMLibrary(*ymLibrary); err != nil {
			log.Fatal(err)
		}
	}
	if *config != "" {
		if err = app.LoadConfiguration(*config); err != nil {
			log.Fatal(err)
		}
	}
	if *pairedProfile != "" {
		if err = app.LoadPairedProfile(*pairedProfile); err != nil {
			log.Fatal(err)
		}
	}
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
