// Command replaycheck verifies a native song against recorded register ticks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func main() {
	song := flag.String("song", "", "native MYS file")
	bank := flag.String("bank", "", "native MYV file")
	trace := flag.String("trace", "", "native register tick JSON")
	flag.Parse()
	if *song == "" || *trace == "" {
		log.Fatal("song and trace are required")
	}
	p, err := project.Load(*song, *bank)
	if err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(*trace)
	if err != nil {
		log.Fatal(err)
	}
	var ticks []replay.RegisterTick
	if err = json.Unmarshal(raw, &ticks); err != nil {
		log.Fatal(err)
	}
	if err = replay.VerifyRegisterTrace(p, ticks); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Verified %d native replay calls: R0–R13 and envelope writes match.\n", len(ticks))
}
