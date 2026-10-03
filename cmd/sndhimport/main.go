// Command sndhimport creates an editable excerpt from an executable SNDH.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func main() {
	input := flag.String("input", "", "SNDH executable, plain or ICE-packed")
	output := flag.String("output", "candidate.mys", "editable native output song")
	subtune := flag.Int("subtune", 0, "one-based song selection; zero uses the header default")
	frames := flag.Int("frames", 400, "exclusive end frame; default is about eight seconds")
	start := flag.Int("start-frame", 0, "first reference frame of the editable excerpt")
	grid := flag.Int("row-frames", 1, "reference frames per row; zero estimates the grid")
	flag.Parse()
	if *input == "" {
		log.Fatal("-input is required")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	_, candidate, report, err := ymimport.ImportSNDHSelection(ctx, raw, *subtune, ymimport.ReconstructionOptions{StartFrame: *start, EndFrame: *frames, FramesPerRow: *grid})
	if err != nil {
		log.Fatal(err)
	}
	if err := project.Save(candidate, *output); err != nil {
		log.Fatal(err)
	}
	details := struct {
		SourceSHA256     string
		RequestedSubtune int
		Analysis         ymimport.Report
	}{fmt.Sprintf("%x", sha256.Sum256(raw)), *subtune, report}
	data, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := project.WriteFileAtomically(*output+".analysis.json", func(w io.Writer) error { _, err := w.Write(append(data, '\n')); return err }); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d patterns, %d positions; source instruments and pattern identities are not claimed\n", report.SourcePlayer, report.Patterns, report.Positions)
}
