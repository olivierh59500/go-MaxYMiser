// Command sndhcheck audits bounded executable SNDH playback without a GUI.
// Metadata acceptance, successful execution and nonzero samples are separate
// observations; none establish recovery of an original editable tracker score.
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/olivierh59500/go-MaxYMiser/internal/sndh"
)

const sampleRate = 48000

type settings struct {
	directory, input, output string
	seconds                  float64
	allSubtunes              bool
	workers                  int
}

type report struct {
	Input       string       `json:"input"`
	Seconds     float64      `json:"requested_seconds_per_subtune"`
	SampleRate  int          `json:"sample_rate"`
	AllSubtunes bool         `json:"all_subtunes"`
	Workers     int          `json:"workers"`
	Interrupted bool         `json:"interrupted"`
	Elapsed     float64      `json:"elapsed_seconds"`
	Counts      counts       `json:"counts"`
	Files       []fileResult `json:"files"`
}

type counts struct {
	FilesDiscovered       int `json:"files_discovered"`
	FilesAttempted        int `json:"files_attempted"`
	FilesRead             int `json:"files_read"`
	FilesMetadataParsed   int `json:"files_metadata_parsed"`
	DeclaredSubtunes      int `json:"declared_subtunes"`
	FilesInitPassed       int `json:"files_all_selected_init_passed"`
	FilesReplayPassed     int `json:"files_all_selected_replay_passed"`
	FilesNonzeroAudio     int `json:"files_with_nonzero_audio"`
	FilesVariableAudio    int `json:"files_with_varying_pcm"`
	FilesEffects          int `json:"files_with_timer_or_dma_effects"`
	FilesErrors           int `json:"files_with_errors"`
	SubtunesAttempted     int `json:"subtunes_attempted"`
	SubtunesInitPassed    int `json:"subtunes_init_passed"`
	SubtunesReplayPassed  int `json:"subtunes_replay_passed"`
	SubtunesNonzeroAudio  int `json:"subtunes_with_nonzero_audio"`
	SubtunesVariableAudio int `json:"subtunes_with_varying_pcm"`
	SubtunesEffects       int `json:"subtunes_with_timer_or_dma_effects"`
	SubtunesErrors        int `json:"subtunes_with_errors"`
	SubtunesClosePassed   int `json:"subtunes_close_passed"`
}

type fileResult struct {
	Path     string          `json:"path"`
	Read     bool            `json:"read"`
	Metadata *sndh.Metadata  `json:"metadata,omitempty"`
	Selected []int           `json:"selected_subtunes,omitempty"`
	Subtunes []subtuneResult `json:"subtunes,omitempty"`
	Stage    string          `json:"error_stage,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type subtuneResult struct {
	Subtune       int      `json:"subtune"`
	InitPassed    bool     `json:"init_passed"`
	ReplayPassed  bool     `json:"replay_passed"`
	ClosePassed   bool     `json:"close_passed"`
	Samples       uint64   `json:"samples_rendered"`
	NonzeroAudio  bool     `json:"nonzero_audio"`
	NonzeroFrames uint64   `json:"nonzero_sample_frames"`
	Peak          int      `json:"peak_pcm_level"`
	Minimum       int      `json:"minimum_pcm_level"`
	Maximum       int      `json:"maximum_pcm_level"`
	RMS           float64  `json:"rms_pcm_level"`
	VariableAudio bool     `json:"varying_pcm"`
	Effects       bool     `json:"timer_or_dma_effects"`
	Registers     [14]byte `json:"final_ym_registers"`
	Stage         string   `json:"error_stage,omitempty"`
	Error         string   `json:"error,omitempty"`
	CloseError    string   `json:"close_error,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sndhcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	var options settings
	flags := flag.NewFlagSet("sndhcheck", flag.ContinueOnError)
	flags.StringVar(&options.directory, "directory", "", "recursively audit SNDH/SND files in this directory")
	flags.StringVar(&options.input, "input", "", "audit one executable SNDH file")
	flags.Float64Var(&options.seconds, "seconds", 0.10, "seconds to execute per selected subtune (greater than 0, at most 5)")
	flags.BoolVar(&options.allSubtunes, "all-subtunes", false, "audit every declared subtune, instead of the default")
	flags.IntVar(&options.workers, "workers", 2, "concurrent file workers (1–8; larger values use 8)")
	flags.StringVar(&options.output, "output", "", "JSON report path; omitted writes JSON to standard output")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || (options.directory == "") == (options.input == "") {
		return fmt.Errorf("provide exactly one of -directory or -input")
	}
	if math.IsNaN(options.seconds) || math.IsInf(options.seconds, 0) || options.seconds <= 0 || options.seconds > 5 {
		return fmt.Errorf("-seconds must be greater than 0 and at most 5")
	}
	if options.workers < 1 {
		return fmt.Errorf("-workers must be positive")
	}
	options.workers = min(8, options.workers)
	paths, input, err := inputPaths(options)
	if err != nil {
		return err
	}
	if options.output != "" {
		output, err := filepath.Abs(options.output)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if path == output {
				return fmt.Errorf("report destination would replace an input music file")
			}
		}
		if err := prepareDestination(options.output); err != nil {
			return err
		}
	}
	started := time.Now()
	r := report{Input: input, Seconds: options.seconds, SampleRate: sampleRate, AllSubtunes: options.allSubtunes, Workers: options.workers}
	r.Counts.FilesDiscovered = len(paths)
	target := uint64(max(1, int(math.Round(options.seconds*sampleRate))))
	jobs := make(chan string)
	results := make(chan fileResult, options.workers)
	var workers sync.WaitGroup
	for i := 0; i < options.workers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for path := range jobs {
				results <- auditFile(ctx, path, input, options.allSubtunes, target)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, path := range paths {
			select {
			case <-ctx.Done():
				return
			case jobs <- path:
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	for result := range results {
		r.Files = append(r.Files, result)
		addCounts(&r.Counts, result)
		if r.Counts.FilesAttempted%100 == 0 {
			fmt.Fprintf(os.Stderr, "sndhcheck: %d/%d files; metadata=%d init=%d replay=%d nonzero=%d errors=%d\n", r.Counts.FilesAttempted, len(paths), r.Counts.FilesMetadataParsed, r.Counts.FilesInitPassed, r.Counts.FilesReplayPassed, r.Counts.FilesNonzeroAudio, r.Counts.FilesErrors)
		}
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	r.Interrupted = ctx.Err() != nil
	r.Elapsed = time.Since(started).Seconds()
	if err := writeReport(options.output, r); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "sndhcheck: checked %d/%d files in %.2fs; metadata=%d init=%d replay=%d nonzero=%d errors=%d\n", r.Counts.FilesAttempted, r.Counts.FilesDiscovered, r.Elapsed, r.Counts.FilesMetadataParsed, r.Counts.FilesInitPassed, r.Counts.FilesReplayPassed, r.Counts.FilesNonzeroAudio, r.Counts.FilesErrors)
	if r.Interrupted {
		return ctx.Err()
	}
	if r.Counts.FilesErrors != 0 {
		return fmt.Errorf("%d files reported execution or input errors; complete details are in the JSON report", r.Counts.FilesErrors)
	}
	return nil
}

func inputPaths(options settings) ([]string, string, error) {
	source := options.directory
	if options.input != "" {
		source = options.input
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, "", err
	}
	if options.input != "" {
		return []string{source}, source, nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("-directory is not a directory")
	}
	var paths []string
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".sndh" || extension == ".snd" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if len(paths) == 0 {
		return nil, "", fmt.Errorf("no SNDH/SND files in %s", source)
	}
	sort.Strings(paths)
	return paths, source, nil
}

func auditFile(ctx context.Context, path, root string, all bool, target uint64) (result fileResult) {
	result.Path = path
	if path != root {
		if relative, err := filepath.Rel(root, path); err == nil {
			result.Path = relative
		}
	}
	result.Stage = "read"
	defer func() {
		if value := recover(); value != nil {
			result.Error = fmt.Sprintf("backend panic: %v", value)
		}
	}()
	if err := ctx.Err(); err != nil {
		result.Stage, result.Error = "cancelled", err.Error()
		return result
	}
	input, err := os.Open(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	data, err := io.ReadAll(io.LimitReader(input, (16<<20)+1))
	closeErr := input.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Read = true
	result.Stage = "metadata"
	file, err := sndh.Parse(data)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Metadata = &file.Metadata
	if all {
		for subtune := 1; subtune <= file.Metadata.Subtunes; subtune++ {
			result.Selected = append(result.Selected, subtune)
		}
	} else {
		result.Selected = []int{file.Metadata.DefaultSubtune}
	}
	result.Stage = ""
	for _, subtune := range result.Selected {
		if err := ctx.Err(); err != nil {
			result.Stage, result.Error = "cancelled", err.Error()
			break
		}
		result.Subtunes = append(result.Subtunes, auditSubtune(ctx, file, subtune, target))
	}
	return result
}

func auditSubtune(ctx context.Context, file *sndh.File, subtune int, target uint64) (result subtuneResult) {
	result.Subtune, result.Stage = subtune, "init"
	defer func() {
		if value := recover(); value != nil {
			result.Error = fmt.Sprintf("backend panic: %v", value)
		}
	}()
	renderer, err := sndh.NewRenderer(file, subtune, sampleRate)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.InitPassed = true
	result.Stage = "replay"
	var measuredFrames uint64
	var sumSquares float64
	defer func() {
		result.Samples = renderer.PositionSamples()
		result.Registers = renderer.Registers()
		result.Effects = renderer.HasEffects()
		result.VariableAudio = result.Minimum != result.Maximum
		if measuredFrames != 0 {
			result.RMS = math.Sqrt(sumSquares / float64(measuredFrames*2))
		}
		if result.Error == "" {
			result.Stage = "close"
		}
		if err := renderer.Close(); err != nil {
			result.CloseError = err.Error()
		} else {
			result.ClosePassed = true
			if result.ReplayPassed {
				result.Stage = ""
			}
		}
	}()
	buffer := make([]byte, 1024*4)
	for renderer.PositionSamples() < target {
		if err := ctx.Err(); err != nil {
			result.Stage, result.Error = "cancelled", err.Error()
			return result
		}
		remaining := target - renderer.PositionSamples()
		chunk := buffer[:min(uint64(len(buffer)/4), remaining)*4]
		n, err := renderer.Read(chunk)
		for at := 0; at+4 <= n; at += 4 {
			left := int(int16(binary.LittleEndian.Uint16(chunk[at:])))
			right := int(int16(binary.LittleEndian.Uint16(chunk[at+2:])))
			if measuredFrames == 0 {
				result.Minimum, result.Maximum = min(left, right), max(left, right)
			} else {
				result.Minimum = min(result.Minimum, left, right)
				result.Maximum = max(result.Maximum, left, right)
			}
			measuredFrames++
			sumSquares += float64(left)*float64(left) + float64(right)*float64(right)
			if left != 0 || right != 0 {
				result.NonzeroAudio = true
				result.NonzeroFrames++
			}
			result.Peak = max(result.Peak, max(left, -left), max(right, -right))
		}
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if n == 0 || n%4 != 0 {
			result.Error = "renderer returned incomplete PCM without an error"
			return result
		}
	}
	result.ReplayPassed, result.Stage = true, ""
	return result
}

func addCounts(c *counts, file fileResult) {
	c.FilesAttempted++
	if file.Read {
		c.FilesRead++
	}
	if file.Metadata != nil {
		c.FilesMetadataParsed++
		c.DeclaredSubtunes += file.Metadata.Subtunes
	}
	initPassed, replayPassed := 0, 0
	nonzero, varying, effects, errors := false, false, false, file.Error != ""
	for _, song := range file.Subtunes {
		c.SubtunesAttempted++
		if song.InitPassed {
			c.SubtunesInitPassed++
			initPassed++
		}
		if song.ReplayPassed {
			c.SubtunesReplayPassed++
			replayPassed++
		}
		if song.ClosePassed {
			c.SubtunesClosePassed++
		}
		if song.NonzeroAudio {
			c.SubtunesNonzeroAudio++
			nonzero = true
		}
		if song.VariableAudio {
			c.SubtunesVariableAudio++
			varying = true
		}
		if song.Effects {
			c.SubtunesEffects++
			effects = true
		}
		if song.Error != "" || song.CloseError != "" {
			c.SubtunesErrors++
			errors = true
		}
	}
	if len(file.Selected) > 0 && initPassed == len(file.Selected) {
		c.FilesInitPassed++
	}
	if len(file.Selected) > 0 && replayPassed == len(file.Selected) {
		c.FilesReplayPassed++
	}
	if nonzero {
		c.FilesNonzeroAudio++
	}
	if varying {
		c.FilesVariableAudio++
	}
	if effects {
		c.FilesEffects++
	}
	if errors {
		c.FilesErrors++
	}
}

func prepareDestination(path string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("report destination is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeReport(path string, r report) error {
	if path == "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(r)
	}
	if err := prepareDestination(path); err != nil {
		return err
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".sndhcheck-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(r)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}
