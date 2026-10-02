package ymimport

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// registerIdentity includes dimensions, chip clock, rate and every decoded
// register frame. Titles, paths and composer names never establish equality.
// Timer/sample recordings are excluded because registers alone omit payloads.
func registerIdentity(trace Trace) string {
	hash := sha256.New()
	var metadata [16]byte
	binary.BigEndian.PutUint32(metadata[0:4], trace.Clock)
	binary.BigEndian.PutUint32(metadata[4:8], uint32(trace.Rate))
	binary.BigEndian.PutUint64(metadata[8:16], uint64(len(trace.Frames)))
	hash.Write(metadata[:])
	for _, frame := range trace.Frames {
		hash.Write(frame[:])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (p PairedProfile) KnownPatternEvidence(trace Trace, start, end int) ([]PatternEvidence, bool) {
	if trace.Effects || p.ReferenceIdentity == "" || registerIdentity(trace) != p.ReferenceIdentity {
		return nil, false
	}
	var out []PatternEvidence
	for _, hit := range p.ReferencePatterns {
		if hit.Start >= start && hit.End <= end {
			copy := hit
			copy.Patterns = append([]int(nil), hit.Patterns...)
			copy.Known = true
			out = append(out, copy)
		}
	}
	return out, true
}
