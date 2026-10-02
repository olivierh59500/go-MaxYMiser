// Command maxymiser inspects and renders native projects without a GUI.
package main

import (
	"flag"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	song := flag.String("song", "", "native MYS file")
	bank := flag.String("bank", "", "native MYV bank")
	wav := flag.String("wav", "", "WAV output path; existing regular files are replaced after complete rendering")
	sndh := flag.String("sndh", "", "native SNDH output path")
	template := flag.String("template", "", "existing MaxYMiser SNDH replay template")
	ice := flag.Bool("ice", false, "ICE-compress native SNDH output")
	duration := flag.Duration("duration", 30*time.Second, "render duration")
	subtune := flag.Int("subtune", 1, "one-based native SNDH subtune")
	songDuration := flag.Bool("song-duration", false, "derive export duration from one arranged traversal")
	flag.Parse()
	if *song == "" && flag.NArg() > 0 {
		*song = flag.Arg(0)
	}
	p := model.Demo()
	var err error
	isYM := *song != "" && strings.EqualFold(filepath.Ext(*song), ".ym")
	if !isYM && (*song != "" || *bank != "") {
		if strings.EqualFold(filepath.Ext(*song), ".snd") || strings.EqualFold(filepath.Ext(*song), ".sndh") {
			p, err = project.LoadSubtune(*song, *subtune)
		} else {
			p, err = project.Load(*song, *bank)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	var ymData []byte
	if *template != "" {
		p.ReplaySource, err = os.ReadFile(*template)
		if err != nil {
			log.Fatal(err)
		}
	}
	if isYM {
		ymData, err = os.ReadFile(*song)
		if err != nil {
			log.Fatal(err)
		}
		synth := replay.NewSynth(replay.New(model.New()), 48000)
		if err = synth.LoadYM(ymData); err != nil {
			log.Fatal(err)
		}
		r, _ := synth.Reference()
		synth.CloseYM()
		fmt.Printf("%s: %s, %s, %.2f seconds; original register recording\n", filepath.Base(*song), r.Name, r.Format, float64(r.Duration)/1000)
	} else {
		fmt.Printf("%s: %d positions, %d patterns, %d sequences, %d Hz, speed %d\n", p.Title, p.Song.Length, len(p.Song.Patterns), p.Bank.SequenceCount, p.Song.TickRate(), p.Song.Speed())
	}
	if *songDuration {
		if isYM {
			log.Fatal("song-duration requires an editable native arrangement")
		}
		measured, err := replay.MeasureSongDuration(p)
		if err != nil {
			log.Fatal(err)
		}
		*duration = measured.Duration
		fmt.Printf("Measured arrangement traversal: %.3f seconds\n", duration.Seconds())
	}
	if *wav != "" {
		if isYM {
			err = export.YM(ymData, *wav, *duration)
		} else {
			err = export.WAV(p, *wav, *duration)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	if *sndh != "" {
		if isYM {
			log.Fatal("reconstruct the YM into an editable native project before SNDH export")
		}
		if err = project.SaveSNDHPacked(p, *sndh, *duration, *ice); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Native SNDH exported:", *sndh)
	}
}
