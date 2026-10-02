package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"
)

// sndhDurationRate reads the outer wrapper's call rate. This can differ from
// each native song's rate: a TC200 selector can run a song at 50 Hz internally.
func sndhDurationRate(prefix []byte) (int, error) {
	end := bytes.Index(prefix, []byte("HDNS"))
	if end < 16 || end > 512 {
		return 0, fmt.Errorf("native: unsupported SNDH duration header")
	}
	for _, tag := range []string{"TA", "TB", "TC", "TD", "!V"} {
		at := sndhHeaderTag(prefix, tag, end)
		if at < 0 {
			continue
		}
		start := at + len(tag)
		stop := start
		for stop < end && prefix[stop] >= '0' && prefix[stop] <= '9' {
			stop++
		}
		if stop == start || stop >= end || prefix[stop] != 0 {
			return 0, fmt.Errorf("native: invalid SNDH duration call rate")
		}
		rate, err := strconv.Atoi(string(prefix[start:stop]))
		if err != nil || rate < 1 || rate > 2000 {
			return 0, fmt.Errorf("native: unsupported SNDH duration call rate")
		}
		return rate, nil
	}
	return 50, nil
}

// setSNDHDurations updates every existing TIME and FRMS entry together. FRMS
// counts calls to the outer SNDH play entry, with zero denoting indefinite play.
// Fractional durations round up to avoid ending before the requested instant.
func setSNDHDurations(prefix []byte, durations []time.Duration, rate int) error {
	end := bytes.Index(prefix, []byte("HDNS"))
	if end < 16 || end > 512 || len(durations) < 1 || rate < 1 || rate > 2000 {
		return fmt.Errorf("native: invalid SNDH duration fields")
	}
	timeAt, framesAt, err := sndhDurationTags(prefix, end, len(durations))
	if err != nil {
		return err
	}
	var frames []uint32
	for _, duration := range durations {
		if duration < 0 {
			return fmt.Errorf("native: negative SNDH duration")
		}
		seconds := int64(duration / time.Second)
		count := seconds*int64(rate) + (int64(duration%time.Second)*int64(rate)+int64(time.Second)-1)/int64(time.Second)
		if timeAt >= 0 && seconds > 65535 || framesAt >= 0 && count > int64(^uint32(0)) {
			return fmt.Errorf("native: SNDH duration exceeds its metadata range")
		}
		if timeAt < 0 && framesAt < 0 && duration != 0 {
			return fmt.Errorf("native: SNDH template has no duration field")
		}
		frames = append(frames, uint32(count))
	}
	for slot, duration := range durations {
		if timeAt >= 0 {
			binary.BigEndian.PutUint16(prefix[timeAt+4+slot*2:], uint16(duration/time.Second))
		}
		if framesAt >= 0 {
			binary.BigEndian.PutUint32(prefix[framesAt+4+slot*4:], frames[slot])
		}
	}
	return nil
}

// Binary duration arrays do not need a preceding zero byte. Skip complete
// fields so a duration word or a title containing "TIME" cannot become a tag.
func sndhDurationTags(prefix []byte, end, songs int) (int, int, error) {
	timeAt, framesAt := -1, -1
	for at := 16; at+4 <= end; {
		tag := string(prefix[at : at+4])
		size := 0
		switch tag {
		case "TIME":
			size = 4 + songs*2
			if timeAt >= 0 {
				return 0, 0, fmt.Errorf("native: duplicate TIME array")
			}
			timeAt = at
		case "FRMS":
			size = 4 + songs*4
			if framesAt >= 0 {
				return 0, 0, fmt.Errorf("native: duplicate FRMS array")
			}
			framesAt = at
		case "FLAG":
			size = 4 + songs*2
		case "TITL", "COMM", "RIPP", "CONV", "YEAR":
			zero := bytes.IndexByte(prefix[at+4:end], 0)
			if zero < 0 {
				return 0, 0, fmt.Errorf("native: unterminated SNDH text field")
			}
			size = 5 + zero
		}
		if size == 0 {
			at++
			continue
		}
		if at+size > end {
			return 0, 0, fmt.Errorf("native: SNDH template lacks its complete duration array")
		}
		at += size
	}
	return timeAt, framesAt, nil
}
