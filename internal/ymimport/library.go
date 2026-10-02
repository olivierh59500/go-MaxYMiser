package ymimport

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type YMAlternative struct {
	Path, Title string
	Score       float64
}

type YMLibrary struct{ Files []YMAlternative }

func IndexLibrary(directory string) (YMLibrary, error) {
	var library YMLibrary
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".ym") {
			library.Files = append(library.Files, YMAlternative{Path: path, Title: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))})
		}
		return nil
	})
	return library, err
}

func titleKey(title string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// Alternatives uses title evidence only. It does not claim equivalence between
// a native SNDH arrangement and an independently recorded YM version.
func (library YMLibrary) Alternatives(path string, data []byte) []YMAlternative {
	titles := []string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	if at := bytes.Index(data[:min(512, len(data))], []byte("TITL")); at >= 0 {
		if end := bytes.IndexByte(data[at+4:], 0); end >= 0 {
			titles = append(titles, string(data[at+4:at+4+end]))
		}
	}
	var result []YMAlternative
	for _, file := range library.Files {
		score := 0.0
		for _, title := range titles {
			if titleKey(title) != "" && titleKey(title) == titleKey(file.Title) {
				score = 1
			} else {
				score = max(score, titleSimilarity(title, file.Title)*0.8)
			}
		}
		if score >= 0.55 {
			file.Score = score
			result = append(result, file)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Path < result[j].Path
	})
	return result[:min(8, len(result))]
}
