package ymimport

import "fmt"

type Evidence struct {
	Family             string
	Similarity         float64
	Files, Occurrences int
	Examples           []string
}

func (c Corpus) Evidence(trace Trace) []Evidence {
	best := map[string]Evidence{}
	for channel := 0; channel < 3; channel++ {
		for _, event := range ExtractEvents(trace, channel) {
			family, score, ok := c.Match(event.Features)
			if !ok || score < 40 {
				continue
			}
			if previous, ok := best[family.ID]; !ok || score > previous.Similarity {
				best[family.ID] = Evidence{family.ID, score, family.Files, family.Occurrences, family.Examples}
			}
		}
	}
	var out []Evidence
	for _, e := range best {
		out = append(out, e)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Files > out[i].Files {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
func (e Evidence) Summary() string {
	return fmt.Sprintf("matching support %.0f/100; evidence in %d distinct recordings (%d events)", e.Similarity, e.Files, e.Occurrences)
}
