package ymimport

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Fingerprint struct {
	ID                 string
	Kind               string
	Occurrences, Files int
	Examples           []string
	Features           []int16
}
type Corpus struct {
	Author                      string `json:"author"`
	Files, Decoded, Unsupported int
	Unique                      int
	Frames                      int64
	Instruments                 []Fingerprint
	Motifs                      []Fingerprint
	Errors                      []string
}
type occurrence struct {
	kind     string
	features []int16
	count    int
	files    map[string]bool
}
type Event struct {
	Start, End int
	Note       int
	Features   []int16
}

// ExtractEvents uses onset and pitch evidence without claiming access to the
// source instruments. Feature vectors normalize pitch and volume for matching.
func ExtractEvents(trace Trace, channel int) []Event {
	var out []Event
	start := -1
	lastNote := -1
	active := func(frame int) bool {
		r := trace.Frames[frame]
		return r[8+channel]&31 != 0 && (r[7]&(1<<channel) == 0 || r[7]&(1<<(channel+3)) == 0 || r[8+channel]&16 != 0)
	}
	noteAt := func(frame int) int {
		r := trace.Frames[frame]
		period := int(r[channel*2]) | int(r[channel*2+1]&15)<<8
		if period == 0 {
			return -1
		}
		return int(math.Round(69 + 12*math.Log2(float64(trace.Clock)/16/float64(period)/440)))
	}
	emit := func(end int) {
		if start < 0 || end-start < 3 {
			return
		}
		values := eventFeatures(trace, channel, start, end)
		out = append(out, Event{start, end, noteAt(start), values})
	}
	for i := range trace.Frames {
		if !active(i) {
			emit(i)
			start = -1
			lastNote = -1
			continue
		}
		note := noteAt(i)
		onset := start < 0
		if i > 0 && start >= 0 {
			before, now := trace.Frames[i-1], trace.Frames[i]
			oldVolume, newVolume := int(before[8+channel]&15), int(now[8+channel]&15)
			onset = (newVolume-oldVolume >= 3 && i-start >= 3) || (note != lastNote && note >= 0 && lastNote >= 0 && i-start >= 6)
		}
		if onset {
			emit(i)
			start = i
		}
		lastNote = note
	}
	emit(len(trace.Frames))
	return out
}
func eventFeatures(trace Trace, channel, start, end int) []int16 {
	size := 16
	features := make([]int16, size*5)
	first := trace.Frames[start]
	base := int(first[channel*2]) | int(first[channel*2+1]&15)<<8
	maxVolume := 1
	for at := start; at < end; at++ {
		maxVolume = max(maxVolume, int(trace.Frames[at][8+channel]&15))
	}
	for n := 0; n < size; n++ {
		at := start + n*(end-start)/size
		if at >= end {
			at = end - 1
		}
		r := trace.Frames[at]
		period := int(r[channel*2]) | int(r[channel*2+1]&15)<<8
		pitch := 0
		if period > 0 && base > 0 {
			pitch = int(math.Round(24 * math.Log2(float64(base)/float64(period))))
		}
		mix := 0
		if r[7]&(1<<channel) == 0 {
			mix |= 1
		}
		if r[7]&(1<<(channel+3)) == 0 {
			mix |= 2
		}
		if r[8+channel]&16 != 0 {
			mix |= 4
		}
		if mix&1 != 0 {
			features[n*5] = int16(max(-127, min(127, pitch)))
		}
		features[n*5+1] = int16(int(r[8+channel]&15) * 15 / maxVolume)
		features[n*5+2] = int16(mix)
		if mix&2 != 0 {
			features[n*5+3] = int16(r[6] & 31)
		}
		if mix&4 != 0 {
			features[n*5+4] = int16(r[13] & 15)
		}
	}
	return features
}
func signature(kind string, features []int16) string {
	bytes := make([]byte, len(features)*2+len(kind))
	copy(bytes, kind)
	for i, v := range features {
		binary.LittleEndian.PutUint16(bytes[len(kind)+i*2:], uint16(v))
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:12])
}
func Learn(directory, author string, progress func(int, string)) (Corpus, error) {
	corpus := Corpus{Author: author}
	instrumentSet, motifSet := map[string]*occurrence{}, map[string]*occurrence{}
	var paths []string
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".ym") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return corpus, err
	}
	sort.Strings(paths)
	corpus.Files = len(paths)
	add := func(set map[string]*occurrence, kind string, features []int16, file string) {
		key := signature(kind, features)
		value := set[key]
		if value == nil {
			value = &occurrence{kind: kind, features: append([]int16(nil), features...), files: map[string]bool{}}
			set[key] = value
		}
		value.count++
		value.files[file] = true
	}
	seenTraces := map[[32]byte]bool{}
	for index, path := range paths {
		if progress != nil {
			progress(index, filepath.Base(path))
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			corpus.Errors = append(corpus.Errors, err.Error())
			continue
		}
		trace, err := Decode(raw)
		if err != nil {
			corpus.Unsupported++
			corpus.Errors = append(corpus.Errors, filepath.Base(path)+": "+err.Error())
			continue
		}
		corpus.Decoded++
		hash := sha256.New()
		for _, frame := range trace.Frames {
			hash.Write(frame[:])
		}
		var identity [32]byte
		copy(identity[:], hash.Sum(nil))
		if seenTraces[identity] {
			continue
		}
		seenTraces[identity] = true
		corpus.Unique++
		corpus.Frames += int64(len(trace.Frames))
		for channel := 0; channel < 3; channel++ {
			events := ExtractEvents(trace, channel)
			for _, event := range events {
				add(instrumentSet, "timbre", event.Features, filepath.Base(path))
			}
			for at := 0; at+8 <= len(events); at++ {
				features := make([]int16, 0, 16)
				base := events[at].Note
				unit := max(1, events[at].End-events[at].Start)
				for _, event := range events[at : at+8] {
					features = append(features, int16(event.Note-base), int16((event.End-event.Start)*8/unit))
				}
				add(motifSet, "phrase", features, filepath.Base(path))
			}
		}
	}
	collect := func(set map[string]*occurrence) []Fingerprint {
		var results []Fingerprint
		for id, value := range set {
			if len(value.files) < 2 {
				continue
			}
			files := make([]string, 0, len(value.files))
			for file := range value.files {
				files = append(files, file)
			}
			sort.Strings(files)
			results = append(results, Fingerprint{ID: id, Kind: value.kind, Occurrences: value.count, Files: len(files), Examples: files[:min(6, len(files))], Features: value.features})
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Files != results[j].Files {
				return results[i].Files > results[j].Files
			}
			return results[i].Occurrences > results[j].Occurrences
		})
		return results
	}
	corpus.Instruments = collect(instrumentSet)
	corpus.Motifs = collect(motifSet)
	return corpus, nil
}
func SaveCorpus(corpus Corpus, path string) error {
	b, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func LoadCorpus(path string) (Corpus, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, err
	}
	var corpus Corpus
	err = json.Unmarshal(b, &corpus)
	if err == nil && len(corpus.Instruments) == 0 {
		err = fmt.Errorf("ymimport: profile contains no recurring timbral evidence")
	}
	return corpus, err
}

// Match ranks recurring evidence by normalized feature distance. The score is
// similarity support, not a probability of recovering the original instrument.
func (c Corpus) Match(features []int16) (Fingerprint, float64, bool) {
	best := Fingerprint{}
	distance := math.Inf(1)
	for _, candidate := range c.Instruments {
		if len(candidate.Features) > 2 && len(features) > 2 && candidate.Features[2] != features[2] {
			continue
		}
		if len(candidate.Features) != len(features) {
			continue
		}
		sum := 0.0
		for i, v := range features {
			delta := float64(v - candidate.Features[i])
			weight := 1.0
			if i%5 == 2 {
				weight = 16
			}
			sum += delta * delta * weight
		}
		d := math.Sqrt(sum / float64(len(features)))
		if d < distance {
			distance = d
			best = candidate
		}
	}
	if math.IsInf(distance, 1) {
		return best, 0, false
	}
	score := 100 / (1 + distance)
	dynamic := false
	for at := 5; at < len(features); at++ {
		if features[at] != features[at%5] {
			dynamic = true
			break
		}
	}
	if !dynamic {
		score = min(score, 40)
	}
	return best, score, true
}
