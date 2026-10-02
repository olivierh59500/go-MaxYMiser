// Command ympair extracts native music labels and verifies a candidate YM pair.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func main() {
	source := flag.String("sndh", "", "SNDH containing a supported native music player")
	ym := flag.String("ym", "", "candidate equivalent YM; verifies alignment and trains a labelled profile")
	subtune := flag.Int("subtune", 0, "zero-based source subtune")
	frames := flag.Int("frames", 6000, "source timeline frame limit")
	output := flag.String("output", "source-score.json", "source labels JSON")
	bankPath := flag.String("bank", "", "export translated native MYV square/envelope/arpeggio instruments")
	flag.Parse()
	raw, err := os.ReadFile(*source)
	if err != nil {
		log.Fatal(err)
	}
	score, err := ymimport.DecodeSource(raw, *subtune, *frames)
	if err != nil {
		log.Fatal(err)
	}
	var result any = score
	if *ym != "" {
		raw, err := os.ReadFile(*ym)
		if err != nil {
			log.Fatal(err)
		}
		trace, err := ymimport.Decode(raw)
		if err != nil {
			log.Fatal(err)
		}
		profile, err := ymimport.LearnPair(score, trace, *ym)
		if err != nil {
			log.Fatal(err)
		}
		result = profile
		fmt.Printf("Verified alignment: offset %d frames, transpose %d, %d/%d tonal events; held-out labels %d/%d correct (%d abstentions)\n", profile.Alignment.Offset, profile.Alignment.Transpose, profile.Alignment.Matched, profile.Alignment.Checked, profile.Validation.Correct, profile.Validation.Known, profile.Validation.Abstained)
		fmt.Printf("Pattern validation: %d/%d known source passages recognized; %d/%d candidate passages include the correct source pattern\n", profile.PatternValidation.Recognized, profile.PatternValidation.Known, profile.PatternValidation.Correct, profile.PatternValidation.Hits)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
	if *bankPath != "" {
		bank, report, err := ymimport.SourceVoiceBank(score)
		if err != nil {
			log.Fatal(err)
		}
		data, err := native.EncodeVoiceBank(bank)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*bankPath, data, 0644); err != nil {
			log.Fatal(err)
		}
		data, err = json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*bankPath+".analysis.json", append(data, '\n'), 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Translated %d source instruments; %d unsupported definitions retained in the source report\n", len(report.Converted), len(report.Unsupported))
	}
	notes := 0
	for _, event := range score.Events {
		if !event.Rest {
			notes++
		}
	}
	fmt.Printf("%s: %d original patterns, %d instrument definitions, %d labelled notes and %d rests\n", score.Player, len(score.Patterns), len(score.Instruments), notes, len(score.Events)-notes)
}
