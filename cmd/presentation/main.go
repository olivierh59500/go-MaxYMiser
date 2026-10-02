// Command presentation records the tracker workflow with synchronized audio.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/go-MaxYMiser/internal/ui"
)

func main() {
	native := flag.String("sndh", "", "editable native SNDH example")
	ym := flag.String("ym", "", "YM recording for the reconstruction example")
	output := flag.String("output", "", "new MP4 output path")
	seconds := flag.Int("seconds", 180, "recording duration, up to 180 seconds")
	flag.Parse()
	if err := run(*native, *ym, *output, *seconds); err != nil {
		log.Fatal(err)
	}
}

func run(native, ym, output string, seconds int) error {
	if output == "" {
		return fmt.Errorf("output is required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output must not already exist")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(output), ".maxymiser-video-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	pcm, err := os.Create(filepath.Join(work, "audio.s16"))
	if err != nil {
		return err
	}
	defer pcm.Close()
	video := filepath.Join(work, "video.mp4")
	encoder := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-f", "rawvideo", "-pixel_format", "rgba", "-video_size", "1280x960", "-framerate", "30", "-i", "pipe:0", "-an", "-c:v", "libx264", "-preset", "fast", "-crf", "19", "-threads", "4", "-pix_fmt", "yuv420p", "-y", video)
	encoder.Stderr = os.Stderr
	pipe, err := encoder.StdinPipe()
	if err != nil {
		return err
	}
	if err = encoder.Start(); err != nil {
		return err
	}
	base := output[:len(output)-len(filepath.Ext(output))]
	p, err := ui.NewPresentation(ui.PresentationConfig{Native: native, YM: ym, Workspace: base + "-workspace", Poster: base + ".png", Video: pipe, Audio: pcm, DurationSeconds: seconds})
	if err != nil {
		pipe.Close()
		encoder.Wait()
		return err
	}
	defer p.Close()
	ebiten.SetWindowSize(960, 720)
	ebiten.SetWindowTitle("Go MaxYMiser — guided presentation recording")
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetVsyncEnabled(false)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	err = ebiten.RunGame(p)
	pipe.Close()
	encodeErr := encoder.Wait()
	pcm.Close()
	if err != nil {
		return err
	}
	if encodeErr != nil {
		return encodeErr
	}
	if err = p.Report(base + ".json"); err != nil {
		return err
	}
	final := filepath.Join(work, "final.mp4")
	mux := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-i", video, "-f", "s16le", "-ar", "48000", "-ac", "2", "-i", pcm.Name(), "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-af", "volume=-2dB", "-c:a", "aac", "-b:a", "160k", "-metadata", "title=Go MaxYMiser — Guided presentation", "-metadata", "artist=Malakh Software / Olivier Houte", "-movflags", "+faststart", "-shortest", "-y", final)
	mux.Stderr = os.Stderr
	if err = mux.Run(); err != nil {
		return err
	}
	return os.Link(final, output)
}
