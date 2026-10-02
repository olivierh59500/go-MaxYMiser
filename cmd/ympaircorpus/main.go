// Command ympaircorpus trains and evaluates explicitly paired SNDH/YM recordings.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/olivierh59500/go-MaxYMiser/internal/project"
	"github.com/olivierh59500/go-MaxYMiser/internal/ymimport"
)

type manifest struct {
	Version int `json:"version"`
	Pairs   []struct {
		Group   string `json:"composition_group"`
		Name    string `json:"name"`
		SNDH    string `json:"sndh"`
		YM      string `json:"ym"`
		Subtune int    `json:"subtune"`
	} `json:"pairs"`
}

func loadSongs(path string, frames int) ([]ymimport.PairedSong, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 4<<20))
	d.DisallowUnknownFields()
	var m manifest
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("manifest: expected one JSON object")
	}
	if m.Version != 1 || len(m.Pairs) < 2 || len(m.Pairs) > 1024 {
		return nil, fmt.Errorf("manifest: version 1 and 2–1024 pairs are required")
	}
	resolve := func(name string) string {
		if filepath.IsAbs(name) {
			return name
		}
		return filepath.Join(filepath.Dir(path), name)
	}
	var songs []ymimport.PairedSong
	for _, entry := range m.Pairs {
		if entry.SNDH == "" || entry.YM == "" || entry.Group == "" {
			return nil, fmt.Errorf("manifest: SNDH, YM and composition_group are required")
		}
		raw, err := os.ReadFile(resolve(entry.SNDH))
		if err != nil {
			return nil, err
		}
		score, err := ymimport.DecodeSource(raw, entry.Subtune, frames)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.SNDH, err)
		}
		raw, err = os.ReadFile(resolve(entry.YM))
		if err != nil {
			return nil, err
		}
		trace, err := ymimport.Decode(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.YM, err)
		}
		name := entry.Name
		if name == "" {
			name = filepath.Base(entry.SNDH)
		}
		songs = append(songs, ymimport.PairedSong{Group: entry.Group, Name: name, YMFile: resolve(entry.YM), Score: score, Trace: trace})
		fmt.Printf("Decoded %s: %d source events\n", name, len(score.Events))
	}
	return songs, nil
}

func writeJSON(path string, value any) error {
	return project.WriteFileAtomically(path, func(w io.Writer) error {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(value)
	})
}

func main() {
	manifestPath := flag.String("manifest", "", "JSON manifest of SNDH/YM pairs grouped by composition")
	frames := flag.Int("frames", 6000, "decoded source frame limit for each recording")
	output := flag.String("output", "cross-song-report.json", "whole-composition validation report")
	modelPath := flag.String("model", "", "optional corpus profile for YM reconstruction in the tracker")
	minimum := flag.Int("min-compositions", 1, "independent training compositions required for a sound label")
	flag.Parse()
	if *manifestPath == "" {
		log.Fatal("manifest is required")
	}
	songs, err := loadSongs(*manifestPath, *frames)
	if err != nil {
		log.Fatal(err)
	}
	report, err := ymimport.ValidatePairedCorpusWithSupport(songs, *minimum, func(group string) { fmt.Printf("Held out: %s\n", group) })
	if err != nil {
		log.Fatal(err)
	}
	var model ymimport.PairedProfile
	if *modelPath != "" {
		model, err = ymimport.LearnPairedCorpus(songs)
		if err != nil {
			log.Fatal(err)
		}
		model.Corpus.MinimumGroups = *minimum
	}
	if err := writeJSON(*output, report); err != nil {
		log.Fatal(err)
	}
	if *modelPath != "" {
		if err := writeJSON(*modelPath, model); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Saved %d-definition model trained on %d compositions\n", len(model.Source.Instruments), len(model.Corpus.Groups))
	}
	s, y := report.SourceBounds, report.YMOnsets
	fmt.Printf("Source boundaries: %d/%d known events correct, %d wrong, %d abstained; %d unknown definitions accepted\n", s.Correct, s.Known, s.Wrong, s.Abstained, s.UnknownAccepted)
	fmt.Printf("Independent YM onsets: %d/%d known events correct, %d/%d source onsets matched; %d accepted unmatched events\n", y.Correct, y.Known, y.Matched, y.Events, y.ExtraAccepted)
}
