package ymimport

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// PhraseMatch identifies musical evidence independently of a filename match.
// Times refer to the original recordings, before any tempo normalization.
type PhraseMatch struct {
	LeftChannel    int     `json:"left_channel"`
	RightChannel   int     `json:"right_channel"`
	LeftSeconds    float64 `json:"left_seconds"`
	RightSeconds   float64 `json:"right_seconds"`
	Transpose      int     `json:"transpose_semitones"`
	TempoRatio     float64 `json:"right_duration_over_left"`
	RhythmError    float64 `json:"rhythm_error"`
	TimbreDistance float64 `json:"timbre_distance"`
	Intervals      []int   `json:"intervals"`
	ContextNotes   int     `json:"matching_context_notes"`
}

type RecordingComparison struct {
	Left                    string        `json:"left"`
	Right                   string        `json:"right"`
	TitleScore              float64       `json:"title_score"`
	SharedPhrases           int           `json:"distinct_shared_phrases"`
	RhythmMatches           int           `json:"rhythm_matches"`
	LongPhraseMatches       int           `json:"sixteen_note_matches"`
	LeftCoverage            float64       `json:"left_phrase_coverage"`
	RightCoverage           float64       `json:"right_phrase_coverage"`
	MedianTempoRatio        float64       `json:"median_duration_ratio"`
	MedianTimbreDistance    float64       `json:"median_timbre_distance"`
	IdenticalRegisterStream bool          `json:"identical_register_stream"`
	Examples                []PhraseMatch `json:"examples"`
}

type ComparisonReport struct {
	LeftDirectory  string                `json:"left_directory"`
	RightDirectory string                `json:"right_directory"`
	LeftFiles      int                   `json:"left_files"`
	RightFiles     int                   `json:"right_files"`
	Comparisons    []RecordingComparison `json:"comparisons"`
	Errors         []string              `json:"errors,omitempty"`
	Method         string                `json:"method"`
}

type melodicNote struct {
	start, end, note int
	features         []int16
}

type melodyPhrase struct {
	channel, start int
	notes          [8]melodicNote
	context        []melodicNote
}

// melodicPhrases removes consecutive equal pitches and excludes noise-only
// events. Arpeggios and slides remain ambiguous and are not labelled as notes
// from the source score. Generic scales and repeated intervals are omitted.
func melodicPhrases(trace Trace) map[string][]melodyPhrase {
	phrases := map[string][]melodyPhrase{}
	for channel := 0; channel < 3; channel++ {
		var notes []melodicNote
		for _, event := range ExtractEvents(trace, channel) {
			if event.Note < 0 || trace.Frames[event.Start][7]&(1<<channel) != 0 {
				continue
			}
			if len(notes) > 0 {
				last := &notes[len(notes)-1]
				if last.note == event.Note && event.Start-last.end <= trace.Rate/2 {
					last.end = event.End
					continue
				}
			}
			notes = append(notes, melodicNote{event.Start, event.End, event.Note, event.Features})
		}
		for at := 0; at+8 <= len(notes); at++ {
			var phrase melodyPhrase
			copy(phrase.notes[:], notes[at:at+8])
			phrase.channel, phrase.start = channel, notes[at].start
			phrase.context = notes[at:min(at+32, len(notes))]
			var intervals [7]int
			variety := map[int]bool{}
			valid := true
			for i := range intervals {
				intervals[i] = phrase.notes[i+1].note - phrase.notes[i].note
				variety[intervals[i]] = true
				if phrase.notes[i+1].start-phrase.notes[i].end > trace.Rate*2 {
					valid = false
				}
			}
			if !valid || len(variety) < 3 {
				continue
			}
			key := fmt.Sprint(intervals)
			// Bound work on recordings that repeat the same phrase for minutes.
			if len(phrases[key]) < 12 {
				phrases[key] = append(phrases[key], phrase)
			}
		}
	}
	return phrases
}

func comparePhrase(left, right melodyPhrase, leftRate, rightRate int) PhraseMatch {
	lspan := float64(left.notes[7].start - left.notes[0].start)
	rspan := float64(right.notes[7].start - right.notes[0].start)
	match := PhraseMatch{
		LeftChannel: left.channel, RightChannel: right.channel,
		LeftSeconds:  float64(left.start) / float64(leftRate),
		RightSeconds: float64(right.start) / float64(rightRate),
		Transpose:    right.notes[0].note - left.notes[0].note,
		TempoRatio:   rspan * float64(leftRate) / (lspan * float64(rightRate)),
	}
	for i := 0; i < 7; i++ {
		match.Intervals = append(match.Intervals, left.notes[i+1].note-left.notes[i].note)
		x := float64(left.notes[i+1].start-left.notes[i].start) / lspan
		y := float64(right.notes[i+1].start-right.notes[i].start) / rspan
		match.RhythmError += math.Abs(x - y)
	}
	// Pitch was already compared. Describe envelope/mixer/noise differences
	// separately; their distance is neither an author attribution nor a score.
	for i := range left.notes {
		a, b := left.notes[i].features, right.notes[i].features
		for k := 0; k < min(len(a), len(b)); k++ {
			if k%5 != 0 {
				match.TimbreDistance += math.Abs(float64(a[k] - b[k]))
			}
		}
	}
	match.TimbreDistance /= 8 * 16 * 4
	match.ContextNotes = 8
	for i := 8; i < min(len(left.context), len(right.context)); i++ {
		if right.context[i].note-left.context[i].note != match.Transpose {
			break
		}
		x := float64(left.context[i].start-left.start) / lspan
		y := float64(right.context[i].start-right.start) / rspan
		if math.Abs(x-y) > 0.12 {
			break
		}
		match.ContextNotes++
	}
	return match
}

// CompareRecordings uses exact eight-pitch interval matches, then tests relative
// rhythm. It permits transposition, different tempos and reassigned channels.
// Filenames nominate candidates but never count as musical confirmation.
func CompareRecordings(left, right Trace, leftName, rightName string) RecordingComparison {
	return compareIndexed(left, right, melodicPhrases(left), melodicPhrases(right), leftName, rightName)
}

func compareIndexed(left, right Trace, lphrases, rphrases map[string][]melodyPhrase, leftName, rightName string) RecordingComparison {
	result := RecordingComparison{Left: leftName, Right: rightName, TitleScore: titleSimilarity(leftName, rightName)}
	result.IdenticalRegisterStream = left.Rate == right.Rate && left.Clock == right.Clock && len(left.Frames) == len(right.Frames)
	if result.IdenticalRegisterStream {
		for i := range left.Frames {
			if left.Frames[i] != right.Frames[i] {
				result.IdenticalRegisterStream = false
				break
			}
		}
	}
	var keys []string
	for key := range lphrases {
		if len(rphrases[key]) > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result.SharedPhrases = len(keys)
	var matches []PhraseMatch
	for _, key := range keys {
		best := PhraseMatch{RhythmError: math.Inf(1)}
		for _, l := range lphrases[key] {
			for _, r := range rphrases[key] {
				m := comparePhrase(l, r, left.Rate, right.Rate)
				if m.RhythmError <= 0.2 && (best.RhythmError > 0.2 || m.ContextNotes > best.ContextNotes || m.ContextNotes == best.ContextNotes && m.RhythmError < best.RhythmError) {
					best = m
				}
			}
		}
		// 0.2 is a tolerance for duration quantization and onset uncertainty.
		if best.RhythmError <= 0.2 {
			matches = append(matches, best)
		}
	}
	result.RhythmMatches = len(matches)
	for _, match := range matches {
		if match.ContextNotes >= 16 {
			result.LongPhraseMatches++
		}
	}
	if len(lphrases) > 0 {
		result.LeftCoverage = float64(len(matches)) / float64(len(lphrases))
	}
	if len(rphrases) > 0 {
		result.RightCoverage = float64(len(matches)) / float64(len(rphrases))
	}
	if len(matches) > 0 {
		var ratios, distances []float64
		for _, match := range matches {
			ratios = append(ratios, match.TempoRatio)
			distances = append(distances, match.TimbreDistance)
		}
		sort.Float64s(ratios)
		sort.Float64s(distances)
		result.MedianTempoRatio = ratios[len(ratios)/2]
		result.MedianTimbreDistance = distances[len(distances)/2]
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].ContextNotes != matches[j].ContextNotes {
				return matches[i].ContextNotes > matches[j].ContextNotes
			}
			if matches[i].RhythmError != matches[j].RhythmError {
				return matches[i].RhythmError < matches[j].RhythmError
			}
			return matches[i].LeftSeconds < matches[j].LeftSeconds
		})
		result.Examples = matches[:min(12, len(matches))]
	}
	return result
}

func titleSimilarity(left, right string) float64 {
	tokens := func(name string) map[string]bool {
		name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
		values := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		result := map[string]bool{}
		for _, value := range values {
			switch value {
			case "big", "remix", "demo", "union", "1", "2":
				continue
			}
			result[value] = true
		}
		return result
	}
	a, b := tokens(left), tokens(right)
	if len(a)+len(b) == 0 {
		return 0
	}
	shared := 0
	for token := range a {
		if b[token] {
			shared++
		}
	}
	return float64(shared*2) / float64(len(a)+len(b))
}

// CompareDirectories compares all recordings, including differently named
// recordings. Each trace is decoded once and released after feature indexing.
func CompareDirectories(leftDirectory, rightDirectory string, progress func(int, string)) (ComparisonReport, error) {
	report := ComparisonReport{LeftDirectory: leftDirectory, RightDirectory: rightDirectory,
		Method: "Eight-pitch interval phrases, extended to 32-note contexts; tempo-normalized onset rhythm; all channel pairs; filename evidence kept separate. Generic phrases excluded. Register data cannot uniquely identify original instruments or authorship."}
	paths := func(directory string) ([]string, error) {
		var files []string
		err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".ym") {
				files = append(files, path)
			}
			return nil
		})
		sort.Strings(files)
		return files, err
	}
	leftPaths, err := paths(leftDirectory)
	if err != nil {
		return report, err
	}
	rightPaths, err := paths(rightDirectory)
	if err != nil {
		return report, err
	}
	report.LeftFiles, report.RightFiles = len(leftPaths), len(rightPaths)
	type indexed struct {
		name    string
		trace   Trace
		phrases map[string][]melodyPhrase
	}
	load := func(path string) (indexed, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return indexed{}, err
		}
		trace, err := Decode(raw)
		if err != nil {
			return indexed{}, err
		}
		return indexed{filepath.Base(path), trace, melodicPhrases(trace)}, nil
	}
	var left []indexed
	for _, path := range leftPaths {
		recording, err := load(path)
		if err != nil {
			report.Errors = append(report.Errors, path+": "+err.Error())
			continue
		}
		left = append(left, recording)
	}
	for i, path := range rightPaths {
		if progress != nil {
			progress(i, filepath.Base(path))
		}
		right, err := load(path)
		if err != nil {
			report.Errors = append(report.Errors, path+": "+err.Error())
			continue
		}
		for _, l := range left {
			result := compareIndexed(l.trace, right.trace, l.phrases, right.phrases, l.name, right.name)
			if result.TitleScore >= 0.5 || result.RhythmMatches >= 4 || result.IdenticalRegisterStream {
				report.Comparisons = append(report.Comparisons, result)
			}
		}
	}
	sort.Slice(report.Comparisons, func(i, j int) bool {
		a, b := report.Comparisons[i], report.Comparisons[j]
		if a.Left != b.Left {
			return a.Left < b.Left
		}
		if a.RhythmMatches != b.RhythmMatches {
			return a.RhythmMatches > b.RhythmMatches
		}
		if a.TitleScore != b.TitleScore {
			return a.TitleScore > b.TitleScore
		}
		return a.Right < b.Right
	})
	return report, nil
}

func SaveComparison(report ComparisonReport, path string) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}
