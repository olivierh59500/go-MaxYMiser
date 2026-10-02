package ymimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// PairedSong groups all recordings and arrangements of one composition.
// Group is a curated composition identifier, not a filename-derived label.
type PairedSong struct {
	Group, Name, YMFile string
	Score               SourceScore
	Trace               Trace
}

type PairedCorpusSource struct {
	Group      string        `json:"composition_group"`
	Name       string        `json:"name"`
	SourceHash string        `json:"source_sha256"`
	YMIdentity string        `json:"ym_register_identity"`
	Alignment  PairAlignment `json:"alignment"`
	Examples   int           `json:"labelled_examples"`
}

// Corpus instrument IDs identify complete definitions shared by a player
// family. File offsets, bank-local IDs and resolved arpeggio IDs are excluded.
type PairedCorpusInfo struct {
	Clock          uint32               `json:"chip_clock_hz"`
	Groups         []string             `json:"training_composition_groups"`
	Sources        []PairedCorpusSource `json:"training_sources"`
	InstrumentKeys []string             `json:"instrument_definition_sha256"`
}

type CrossSongCounts struct {
	Events          int `json:"eligible_source_events"`
	Known           int `json:"events_with_training_definition"`
	Correct         int `json:"correct_labels"`
	Wrong           int `json:"wrong_known_labels"`
	Abstained       int `json:"abstained_known_events"`
	Unknown         int `json:"events_without_training_definition"`
	UnknownAccepted int `json:"incorrectly_accepted_unknown_events"`
	Detected        int `json:"independently_detected_ym_events"`
	Matched         int `json:"matched_source_onsets"`
	ExtraAccepted   int `json:"accepted_unmatched_ym_events"`
}

type CrossSongFold struct {
	HeldOutGroup string          `json:"held_out_composition_group"`
	Training     []string        `json:"training_composition_groups"`
	SourceBounds CrossSongCounts `json:"classification_at_source_boundaries"`
	YMOnsets     CrossSongCounts `json:"independent_ym_onsets"`
}

type CrossSongReport struct {
	Version      int                  `json:"version"`
	Pairs        []PairedCorpusSource `json:"verified_pairs"`
	Folds        []CrossSongFold      `json:"leave_one_composition_out"`
	SourceBounds CrossSongCounts      `json:"total_source_boundaries"`
	YMOnsets     CrossSongCounts      `json:"total_independent_ym_onsets"`
	Warnings     []string             `json:"warnings"`
}

type preparedPair struct {
	song      PairedSong
	alignment PairAlignment
	features  []labelledFeatures
	keys      []string
	info      PairedCorpusSource
}

func sourceInstrumentIdentity(player string, rate int, instrument SourceInstrument) string {
	i := cloneCorpusInstrument(instrument)
	i.ID, i.Offset, i.Arpeggio.Offset = 0, 0, 0
	if len(i.Arpeggio.Values) > 0 && len(i.Settings) > 1 && i.Settings[1] < 128 {
		i.Settings[1] = 0 // The resolved arpeggio, including timing, is retained.
	}
	b, _ := json.Marshal(struct {
		Player     string
		Rate       int
		Instrument SourceInstrument
	}{player, rate, i})
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func cloneCorpusInstrument(i SourceInstrument) SourceInstrument {
	i.Settings = append([]byte(nil), i.Settings...)
	i.VolumeSequence = append([]byte(nil), i.VolumeSequence...)
	i.Arpeggio.Values = append([]int(nil), i.Arpeggio.Values...)
	i.NoiseProgram = append([]SourceNoiseStep(nil), i.NoiseProgram...)
	return i
}

// Composition identity catches identical timelines with different wrappers,
// sound-bank IDs, filenames or a constant channel transpose. Other variants
// still need to be assigned to the same curated group by the caller.
func sourceCompositionIdentity(s SourceScore) string {
	type note struct {
		Channel, Frame, Pitch int
		Rest, Retrigger       bool
	}
	var notes []note
	var bases [3]int
	var hasBase [3]bool
	for _, e := range s.Events {
		if !e.Rest && !hasBase[e.Channel] {
			bases[e.Channel], hasBase[e.Channel] = e.Note, true
		}
		pitch := 0
		if !e.Rest {
			pitch = e.Note - bases[e.Channel]
		}
		notes = append(notes, note{e.Channel, e.Frame, pitch, e.Rest, e.Retrigger})
	}
	b, _ := json.Marshal(notes)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func preparePairs(songs []PairedSong) ([]preparedPair, error) {
	if len(songs) < 2 || len(songs) > 1024 {
		return nil, fmt.Errorf("corpus: provide 2–1024 verified candidate pairs")
	}
	groups := map[string]bool{}
	identities := map[string]string{}
	var out []preparedPair
	for _, song := range songs {
		if song.Group == "" || song.Score.Player == "" || len(song.Score.Instruments) == 0 || len(song.Score.Instruments) > 256 {
			return nil, fmt.Errorf("corpus: composition group, player and instrument definitions are required")
		}
		if song.Score.Player != songs[0].Score.Player || song.Score.Rate != songs[0].Score.Rate || song.Trace.Clock != songs[0].Trace.Clock {
			return nil, fmt.Errorf("corpus: train separate profiles for different player families, rates or chip clocks")
		}
		alignment, err := AlignPair(song.Score, song.Trace)
		if err != nil {
			return nil, fmt.Errorf("corpus: %s: %w", song.Name, err)
		}
		p := preparedPair{song: song, alignment: alignment, features: sourceFeatures(song.Score, song.Trace, alignment)}
		if len(p.features) == 0 {
			return nil, fmt.Errorf("corpus: %s has no labelled examples", song.Name)
		}
		for _, i := range song.Score.Instruments {
			p.keys = append(p.keys, sourceInstrumentIdentity(song.Score.Player, song.Score.Rate, i))
		}
		ymID := registerIdentity(song.Trace)
		ids := []string{"ym:" + ymID, "music:" + sourceCompositionIdentity(song.Score)}
		if song.Score.SHA256 != "" {
			ids = append(ids, fmt.Sprintf("sndh:%s:%d", song.Score.SHA256, song.Score.Subtune))
		}
		for _, id := range ids {
			if previous, exists := identities[id]; exists && previous != song.Group {
				return nil, fmt.Errorf("corpus: duplicate source or recording occurs in different composition groups %q and %q", previous, song.Group)
			}
			identities[id] = song.Group
		}
		groups[song.Group] = true
		p.info = PairedCorpusSource{song.Group, song.Name, song.Score.SHA256, ymID, alignment, len(p.features)}
		out = append(out, p)
	}
	if len(groups) < 2 || len(groups) > 256 {
		return nil, fmt.Errorf("corpus: provide 2–256 independent composition groups")
	}
	return out, nil
}

func trainPreparedPairs(pairs []preparedPair, recipes bool) (PairedProfile, error) {
	p := PairedProfile{Version: 1, Source: SourceScore{Player: pairs[0].song.Score.Player, Rate: pairs[0].song.Score.Rate}, Corpus: &PairedCorpusInfo{Clock: pairs[0].song.Trace.Clock}}
	groups, ids, prototypes := map[string]bool{}, map[string]int{}, map[string]int{}
	for sourceIndex, pair := range pairs {
		groups[pair.song.Group] = true
		p.Corpus.Sources = append(p.Corpus.Sources, pair.info)
		local := map[int]int{}
		for _, f := range pair.features {
			key := pair.keys[f.event.Instrument]
			id, exists := ids[key]
			if !exists {
				if len(ids) >= 256 {
					return p, fmt.Errorf("corpus: more than 256 distinct source definitions; split the training corpus")
				}
				id = len(ids)
				ids[key] = id
				i := cloneCorpusInstrument(pair.song.Score.Instruments[f.event.Instrument])
				i.ID, i.Offset, i.Arpeggio.Offset = id, 0, 0
				p.Source.Instruments = append(p.Source.Instruments, i)
				p.Corpus.InstrumentKeys = append(p.Corpus.InstrumentKeys, key)
			}
			local[f.event.Instrument] = id
			featureKey := fmt.Sprint(id) + ":" + signature("source", f.features)
			if at, exists := prototypes[featureKey]; exists {
				p.Prototypes[at].Examples++
			} else {
				if len(p.Prototypes) >= 100000 {
					return p, fmt.Errorf("corpus: too many distinct feature prototypes")
				}
				prototypes[featureKey] = len(p.Prototypes)
				p.Prototypes = append(p.Prototypes, PairedPrototype{id, append([]int16(nil), f.features...), 1})
			}
		}
		if recipes {
			for _, r := range learnRecipes(pair.song.Score, pair.song.Trace, pair.alignment, len(pair.song.Trace.Frames)-pair.alignment.Offset) {
				id, exists := local[r.SourceInstrument]
				if !exists {
					continue
				}
				r.SourceInstrument = id
				r.TrainingSource = fmt.Sprintf("%d:%s", sourceIndex, pair.info.YMIdentity)
				r.Instrument.SetName(fmt.Sprintf("Corpus %02X", id))
				p.Recipes = append(p.Recipes, r)
				if len(p.Recipes) > 100000 {
					return p, fmt.Errorf("corpus: too many editable instrument recipes")
				}
			}
		}
	}
	for group := range groups {
		p.Corpus.Groups = append(p.Corpus.Groups, group)
	}
	sort.Strings(p.Corpus.Groups)
	p.Warnings = []string{
		"Instrument labels identify complete definitions within this player family, not bank-local instrument numbers.",
		"This profile contains no source-score times or original pattern IDs for any target recording.",
		"Whole-composition validation is separate from this model trained on all listed groups; see the corpus evaluation report.",
	}
	return p, nil
}

// LearnPairedCorpus trains only from the explicitly supplied recordings. Source
// pattern IDs and reference timelines are omitted rather than merged across songs.
func LearnPairedCorpus(songs []PairedSong) (PairedProfile, error) {
	pairs, err := preparePairs(songs)
	if err != nil {
		return PairedProfile{}, err
	}
	return trainPreparedPairs(pairs, true)
}

func countCorpusLabel(counts *CrossSongCounts, truth, predicted int, accepted bool) {
	counts.Events++
	if truth < 0 {
		counts.Unknown++
		if accepted {
			counts.UnknownAccepted++
		}
		return
	}
	counts.Known++
	if !accepted {
		counts.Abstained++
	} else if predicted == truth {
		counts.Correct++
	} else {
		counts.Wrong++
	}
}

func addCrossSongCounts(a *CrossSongCounts, b CrossSongCounts) {
	a.Events += b.Events
	a.Known += b.Known
	a.Correct += b.Correct
	a.Wrong += b.Wrong
	a.Abstained += b.Abstained
	a.Unknown += b.Unknown
	a.UnknownAccepted += b.UnknownAccepted
	a.Detected += b.Detected
	a.Matched += b.Matched
	a.ExtraAccepted += b.ExtraAccepted
}

func evaluatePreparedPair(p PairedProfile, pair preparedPair) (CrossSongCounts, CrossSongCounts) {
	ids := map[string]int{}
	for i, key := range p.Corpus.InstrumentKeys {
		ids[key] = i
	}
	truth := func(f labelledFeatures) int {
		if id, known := ids[pair.keys[f.event.Instrument]]; known {
			return id
		}
		return -1
	}
	var sourceCounts, detectedCounts CrossSongCounts
	for _, f := range pair.features {
		id, _, _, accepted := p.MatchSource(f.features)
		countCorpusLabel(&sourceCounts, truth(f), id, accepted)
	}
	// Prediction consumes only independently detected YM events. Native source
	// times are used afterward to score one-to-one onset matches within two frames.
	for ch := 0; ch < 3; ch++ {
		events := ExtractEvents(pair.song.Trace, ch)
		var matched = make(map[int]bool)
		type prediction struct {
			id       int
			accepted bool
		}
		predictions := make([]prediction, len(events))
		for i, e := range events {
			id, _, _, accepted := p.MatchSource(e.Features)
			predictions[i] = prediction{id, accepted}
		}
		for _, f := range pair.features {
			if f.event.Channel != ch {
				continue
			}
			best, distance := -1, 3
			for i, e := range events {
				if !matched[i] && abs(e.Start-f.start) < distance {
					best, distance = i, abs(e.Start-f.start)
				}
			}
			if best < 0 {
				countCorpusLabel(&detectedCounts, truth(f), -1, false)
				continue
			}
			matched[best] = true
			detectedCounts.Matched++
			prediction := predictions[best]
			countCorpusLabel(&detectedCounts, truth(f), prediction.id, prediction.accepted)
		}
		// Only the decoded, overlapping source interval is eligible for evaluation.
		for i, e := range events {
			if e.Start < max(0, pair.alignment.Offset) || e.Start >= min(len(pair.song.Trace.Frames), pair.song.Score.Frames+pair.alignment.Offset) {
				continue
			}
			detectedCounts.Detected++
			if !matched[i] && predictions[i].accepted {
				detectedCounts.ExtraAccepted++
			}
		}
	}
	return sourceCounts, detectedCounts
}

// ValidatePairedCorpus excludes every recording in the target composition's
// group. Classification at known source boundaries is reported separately from
// end-to-end recognition using YM-only event detection.
func ValidatePairedCorpus(songs []PairedSong, progress func(string)) (CrossSongReport, error) {
	report := CrossSongReport{Version: 1}
	pairs, err := preparePairs(songs)
	if err != nil {
		return report, err
	}
	groups := map[string]bool{}
	for _, pair := range pairs {
		groups[pair.song.Group] = true
		report.Pairs = append(report.Pairs, pair.info)
	}
	var ordered []string
	for group := range groups {
		ordered = append(ordered, group)
	}
	sort.Strings(ordered)
	for _, held := range ordered {
		if progress != nil {
			progress(held)
		}
		var training []preparedPair
		for _, pair := range pairs {
			if pair.song.Group != held {
				training = append(training, pair)
			}
		}
		p, err := trainPreparedPairs(training, false)
		if err != nil {
			return report, err
		}
		fold := CrossSongFold{HeldOutGroup: held, Training: p.Corpus.Groups}
		for _, pair := range pairs {
			if pair.song.Group != held {
				continue
			}
			source, detected := evaluatePreparedPair(p, pair)
			addCrossSongCounts(&fold.SourceBounds, source)
			addCrossSongCounts(&fold.YMOnsets, detected)
		}
		report.Folds = append(report.Folds, fold)
		addCrossSongCounts(&report.SourceBounds, fold.SourceBounds)
		addCrossSongCounts(&report.YMOnsets, fold.YMOnsets)
	}
	report.Warnings = []string{
		"Source-boundary classification uses native segmentation; it is not end-to-end YM import accuracy.",
		"Independent YM onset evaluation requires a one-to-one match within two frames; accepted unmatched events are reported separately.",
		"Unknown definitions and missed onsets are retained in the counts; precision alone does not establish coverage.",
		"Grouping variants of the same composition is required; duplicate payload, register stream and normalized timeline checks catch only provable overlap.",
	}
	return report, nil
}
