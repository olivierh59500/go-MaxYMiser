// Command maxymiser inspects and renders native projects without a GUI.
package main

import (
	"flag"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/export"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"log"
	"time"
)

func main() {
	song := flag.String("song", "", "native MYS file")
	bank := flag.String("bank", "", "native MYV bank")
	wav := flag.String("wav", "", "new WAV output path")
	duration := flag.Duration("duration", 30*time.Second, "render duration")
	flag.Parse()
	p := model.Demo()
	var err error
	if *song != "" || *bank != "" {
		p, err = project.Load(*song, *bank)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("%s: %d positions, %d patterns, %d sequences, %d Hz, speed %d\n", p.Title, p.Song.Length, len(p.Song.Patterns), p.Bank.SequenceCount, p.Song.TickRate(), p.Song.Speed())
	if *wav != "" {
		if err = export.WAV(p, *wav, *duration); err != nil {
			log.Fatal(err)
		}
	}
}
