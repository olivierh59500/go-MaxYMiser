// Command sndhaudit checks editable native payloads in a local SNDH collection.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
)

type audit struct {
	Files, NativeCandidates, Imported, Subtunes, ReplayTemplates, RoundTrips int
	Issues                                                                   []issue
}
type issue struct{ File, Stage, Error string }

func main() {
	directory := flag.String("directory", "", "local SNDH directory")
	output := flag.String("output", "", "optional JSON report")
	flag.Parse()
	if *directory == "" {
		log.Fatal("directory is required")
	}
	var report audit
	err := filepath.WalkDir(*directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".sndh") {
			return nil
		}
		report.Files++
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		plain, err := native.UnpackICE(raw)
		if err != nil {
			return err
		}
		if !bytes.Contains(plain, []byte("MYM0INST")) && !bytes.Contains(plain, []byte("MYM1INST")) {
			return nil
		}
		report.NativeCandidates++
		name, _ := filepath.Rel(*directory, path)
		problem := func(stage string, err error) { report.Issues = append(report.Issues, issue{name, stage, err.Error()}) }
		projects, err := native.DecodeContainers(plain)
		if err != nil {
			problem("import", err)
			return nil
		}
		report.Imported++
		report.Subtunes += len(projects)
		if declared := native.DeclaredSubtunes(plain); declared > len(projects) {
			problem("subtunes", fmt.Errorf("container declares %d songs; %d editable payloads were decoded", declared, len(projects)))
		}
		template, err := native.ParseSNDHTemplate(plain)
		if err != nil {
			problem("replay template", err)
			return nil
		}
		report.ReplayTemplates++
		p := projects[0]
		project := &model.Project{Title: p.Title, Author: p.Author, Year: p.Year, Song: p.Song, Bank: p.Bank}
		encoded, err := native.EncodeSNDH(template, project, 0)
		if err != nil {
			problem("export", err)
			return nil
		}
		got, err := native.DecodeContainer(encoded)
		if err != nil {
			problem("reload", err)
			return nil
		}
		song, _ := native.EncodeSong(p.Song)
		newSong, _ := native.EncodeSong(got.Song)
		bank, _ := native.EncodeVoiceBank(p.Bank)
		newBank, _ := native.EncodeVoiceBank(got.Bank)
		if !bytes.Equal(song, newSong) || !bytes.Equal(bank, newBank) || got.Title != p.Title || got.Author != p.Author || got.Year != p.Year {
			problem("round trip", fmt.Errorf("serialized editable data or metadata changed"))
			return nil
		}
		report.RoundTrips++
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	if *output != "" {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(*output, append(raw, '\n'), 0600); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("%d SNDH files; %d MaxYMiser candidates; %d imports (%d subtunes); %d templates; %d unchanged round trips; %d issues\n", report.Files, report.NativeCandidates, report.Imported, report.Subtunes, report.ReplayTemplates, report.RoundTrips, len(report.Issues))
}
