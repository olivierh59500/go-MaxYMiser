// Command ymlearn builds evidence profiles from a composer's YM recordings.
package main

import (
	"flag"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
	"log"
)

func main() {
	directory := flag.String("directory", "", "YM corpus directory")
	author := flag.String("author", "", "profile author")
	output := flag.String("output", "profile.json", "profile JSON path")
	flag.Parse()
	if *directory == "" {
		log.Fatal("directory is required")
	}
	profile, err := ymimport.Learn(*directory, *author, func(index int, file string) {
		if index%25 == 0 {
			fmt.Printf("%d %s\n", index, file)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = ymimport.SaveCorpus(profile, *output); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d/%d files decoded; %d recurring timbre signatures; %d recurring phrases\n", profile.Decoded, profile.Files, len(profile.Instruments), len(profile.Motifs))
}
