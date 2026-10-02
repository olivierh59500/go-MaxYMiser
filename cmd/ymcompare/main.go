// Command ymcompare compares musical phrases across two YM collections.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

func main() {
	left := flag.String("left", "", "first YM directory")
	right := flag.String("right", "", "second YM directory")
	output := flag.String("output", "comparison.json", "comparison JSON path")
	flag.Parse()
	if *left == "" || *right == "" {
		log.Fatal("left and right directories are required")
	}
	report, err := ymimport.CompareDirectories(*left, *right, func(index int, file string) {
		if index%50 == 0 {
			fmt.Printf("%d %s\n", index, file)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = ymimport.SaveComparison(report, *output); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d × %d files; %d candidate comparisons; %d decoding errors\n", report.LeftFiles, report.RightFiles, len(report.Comparisons), len(report.Errors))
	for _, comparison := range report.Comparisons {
		fmt.Printf("%s / %s: %d phrases (%d with 16+ notes), %.1f%% / %.1f%% coverage, duration ratio %.3f, timbre distance %.2f\n",
			comparison.Left, comparison.Right, comparison.RhythmMatches, comparison.LongPhraseMatches, comparison.LeftCoverage*100,
			comparison.RightCoverage*100, comparison.MedianTempoRatio, comparison.MedianTimbreDistance)
	}
}
