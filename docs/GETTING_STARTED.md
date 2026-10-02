# Creating and editing a song

Go MaxYMiser 0.1.0 provides a native-format tracker with a modern interface. It
starts with the original composition First signal; select New to begin a blank
song with an ordinary square-wave instrument. Go 1.26 or newer is required.

```sh
go run .
```

## A first composition

1. Select New, then Patterns. Press Enter to enable note entry. The lower note
   keyboard begins with Z; the upper keyboard begins with Q. F1–F8 select the
   octave. Up/Down select rows and Tab selects another voice. Caps Lock inserts
   note-off. Undo and redo use Ctrl+Z and Ctrl+Y.
2. Select Instruments to choose or edit one of the 32 sounds. The component
   matrix selects square, buzzer and timer effects. Sequence links open their
   corresponding volume, arpeggio, vibrato, mixer, noise or waveform editor.
   Shared sequence edits affect every sound linked to that definition.
3. In Song, arrange the four independent pattern lists. The first three are YM
   voices; the fourth carries the two STe PCM lanes when enabled in Settings.
   Length and Repeat define the traversal. Clone track creates an independent
   occurrence before changing a reused pattern.
4. Press Space to listen. Pattern playback and Record provide focused editing
   and recording; Stop ends the transport. Jam defers position changes to the
   next pattern boundary. Instrument Preview auditions without changing notes.
5. Save as chooses a native MYS/MYV pair. Both files belong together. Open the
   MYS to reload the matching bank automatically. Individual sounds can be
   exchanged as MYI files, and personal settings as MYM.CNF.
6. Choose Export WAV and its duration. Rendering runs independently of editing
   and live playback. An existing complete output remains intact until its
   replacement succeeds. The resulting audio is stereo, 48 kHz, 16-bit PCM.

The complete create/edit/save/reopen/export path is covered by an interface
regression using entered notes and edited instrument parameters. It checks the
reloaded native payloads and a correctly sized, non-silent WAV.

## Opening existing music

```sh
go run . -song /path/to/song.mys -bank /path/to/bank.myv
go run . /path/to/native.sndh
go run . /path/to/recording.ym
```

Supported MaxYMiser SNDH files expose editable instruments and patterns directly.
Verified collections retain independent subtunes. A foreign SNDH can instead
open in the source inspector when its player is supported. The classic Mad Max
inspector offers song selection, original IDs and an explicit excerpt import.
An unrecognized format reports its limitation while retaining the current song.

A YM initially plays the original register recording. Select Range and a bounded
excerpt before Reconstruct; the native tracker has finite pattern and sequence
capacity. The one-frame grid retains editable tone-period curves when capacity
permits. Listen YM and Listen score compare the reference and candidate. Patterns
lists recurring melodic passages, optionally augmented by paired-source
evidence. Detected phrase IDs belong to the analysis, not the original tracker.

Reconstruction is experimental. It cannot reliably recover arbitrary original
instruments, patterns, hardware programs or an author's arrangement. Corpus
support, distances and validation describe evidence rather than certainty.
See [YM reconstruction](YM_RECONSTRUCTION.md) for measured limits and commands.

## Command-line rendering

The headless command does not need an audio device or a graphical display:

```sh
go run ./cmd/maxymiser -version
go run ./cmd/maxymiser -song /path/to/song.mys -bank /path/to/bank.myv \
  -wav /path/to/song.wav -song-duration
```

`-song-duration` follows one arranged traversal; it does not assert an exact audio
loop. External MIDI clock cannot drive an offline render. Native SNDH export
requires a verified replay template supplied at runtime; no original executable
or source music is bundled.
