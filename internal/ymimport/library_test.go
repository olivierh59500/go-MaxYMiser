package ymimport

import "testing"

func TestYMLibraryKeepsVersionsSeparateAndMatchesSNDHTitle(t *testing.T) {
	library := YMLibrary{Files: []YMAlternative{{Path: "/music/Warhawk remix.ym", Title: "Warhawk remix"}, {Path: "/music/Warhawk.ym", Title: "Warhawk"}, {Path: "/music/Another song.ym", Title: "Another song"}}}
	data := append([]byte("SNDHTITLWarhawk"), 0)
	results := library.Alternatives("WAR_HAWK.sndh", data)
	if len(results) != 2 || results[0].Title != "Warhawk" || results[0].Score != 1 || results[1].Title != "Warhawk remix" || results[1].Score >= 1 {
		t.Fatalf("title hint selected or merged versions: %+v", results)
	}
}
