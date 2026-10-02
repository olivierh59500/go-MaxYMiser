// Package export renders tracker songs without relying on a running audio device.
package export

import (
	"encoding/binary"
	"fmt"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
	"io"
	"os"
	"time"
)

func WAV(p *model.Project, path string, duration time.Duration) error {
	if p == nil {
		return fmt.Errorf("export: no tracker project to render")
	}
	if p.Song.State[31]&1 != 0 {
		return fmt.Errorf("export: select internal clock before rendering a tracker WAV; external pulses are unavailable offline")
	}
	engine := replay.New(p)
	engine.Play(false)
	return writeAudio(replay.NewSynth(engine, 48000), path, duration)
}
func YM(data []byte, path string, duration time.Duration) error {
	synth := replay.NewSynth(replay.New(model.New()), 48000)
	if err := synth.LoadYM(data); err != nil {
		return err
	}
	defer synth.CloseYM()
	return writeAudio(synth, path, duration)
}
func writeAudio(reader io.Reader, path string, duration time.Duration) error {
	if duration <= 0 || duration > time.Hour {
		return fmt.Errorf("export: duration must be positive and at most one hour")
	}
	rate := 48000
	frames := int64(duration) * int64(rate) / int64(time.Second)
	length := frames * 4
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(path)
		}
	}()
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(length+36))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], uint32(rate))
	binary.LittleEndian.PutUint32(header[28:], uint32(rate*4))
	binary.LittleEndian.PutUint16(header[32:], 4)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(length))
	if _, err = f.Write(header); err != nil {
		return err
	}
	buffer := make([]byte, 4096*4)
	for left := frames; left > 0; {
		count := min(left, 4096)
		chunk := buffer[:count*4]
		if _, err = io.ReadFull(reader, chunk); err != nil {
			return err
		}
		if _, err = f.Write(chunk); err != nil {
			return err
		}
		left -= count
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	success = true
	return nil
}
