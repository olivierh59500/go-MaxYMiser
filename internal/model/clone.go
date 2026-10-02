package model

// Clone creates an independent project for undo snapshots and offline exports.
func (p *Project) Clone() *Project {
	clone := *p
	clone.Song.Patterns = append([]Pattern(nil), p.Song.Patterns...)
	for i, s := range p.Bank.Samples {
		clone.Bank.Samples[i].PCM = append([]byte(nil), s.PCM...)
		clone.Bank.Samples[i].Trailer = append([]byte(nil), s.Trailer...)
	}
	return &clone
}
