# Guided tracker presentation

`cmd/presentation` records a three-minute tour of the actual tracker controls.
It opens a native MaxYMiser SNDH, edits instruments and linked sequences, auditions
PCM samples, enters an original melody, saves a native pair and exports WAV.
It then loads a YM, reconstructs a bounded excerpt and compares the original
recording with the editable candidate. English captions occupy a separate area
below the interface; the capture contains no desktop or unrelated windows.

```sh
go run ./cmd/presentation \
  -sndh /path/to/sequences-2005.sndh \
  -ym /path/to/warhawk.ym \
  -output recordings/go-maxymiser-presentation.mp4
```

FFmpeg and an available Ebitengine graphics session are required. The output path
must be new. A short preview can be recorded with `-seconds 20`; the complete tour
uses 180 seconds. Native source music is provided at runtime and is not bundled.

The current scenario uses the editable SEQUENCE SNDH example: instrument 03
links to volume sequence 01, and sample 01 contains real PCM data. The YM example
must contain at least 1,200 frames for the illustrated reconstruction range.
The example paths should therefore match these capabilities. All saved working
copies and the five-second exported WAV use the output's `-workspace` directory,
leaving the supplied reference files unchanged.

Video frames and audio share one simulation clock: 30 frames/s, 1,600 stereo
samples per frame at 48 kHz. Export speed does not change musical timing.
The MP4 contains H.264 video and AAC audio, with fast-start metadata. Its PNG
poster and JSON chapter/caption report use the same output basename. An incomplete
recording is rejected. The final soundtrack uses a uniform -2 dB output gain.

The WebM edition can be generated for the portfolio independently:

```sh
ffmpeg -i recordings/go-maxymiser-presentation.mp4 \
  -vf scale=960:720:flags=lanczos -c:v libvpx-vp9 -crf 32 -b:v 0 \
  -row-mt 1 -cpu-used 3 -c:a libopus -b:a 96k \
  recordings/go-maxymiser-presentation.webm
```

The captions distinguish native editable imports from experimental YM
reconstruction. They do not claim general SNDH support or recovery of arbitrary
original instruments and pattern boundaries.
