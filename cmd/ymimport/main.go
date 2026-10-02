// Command ymimport proposes an editable native score from a YM register recording.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
	"log"
	"os"
)

func main() {
	input := flag.String("input", "", "YM recording")
	output := flag.String("output", "candidate.mys", "native output song")
	profile := flag.String("profile", "", "composer corpus JSON")
	flag.Parse()
	raw, err := os.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}
	trace, err := ymimport.Decode(raw)
	if err != nil {
		log.Fatal(err)
	}
	candidate, report, err := ymimport.Reconstruct(trace)
	if err != nil {
		log.Fatal(err)
	}
	if *profile != "" {
		corpus, err := ymimport.LoadCorpus(*profile)
		if err != nil {
			log.Fatal(err)
		}
		report.AuthorProfile = corpus.Author
		report.Evidence = corpus.Evidence(trace)
	}
	if err = project.Save(candidate, *output); err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*output+".analysis.json", append(data, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d candidate instruments, %d patterns, %d positions, %d corpus matches\n", report.Instruments, report.Patterns, report.Positions, len(report.Evidence))
}
