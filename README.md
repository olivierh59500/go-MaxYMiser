# Go MaxYMiser

A modern Go/Ebitengine tracker inspired by **MaxYMiser FM 1.67**, with native
MaxYMiser song and voice-bank editing and YM2149 synthesis supplied by
[YM Player](https://github.com/olivierh59500/ym-player).

## Run

Go 1.26 or newer is required.

```sh
 go run .
 go run . -song /path/to/song.mys -bank /path/to/voices.myv
 go run . /path/to/music.ym
```

The initial project, **First signal**, is an original composition included for
trying the editor. External MaxYMiser files are not needed to launch it.

## Workspaces

- **Patterns**: three YM voices, or the two STe sample voices carried by the
  fourth native pattern stream; note, instrument, volume and two effect columns.
- **Song**: the four independent pattern lists making up the arrangement,
  selectable positions, length/repeat and title/artist editing. Assigning a new
  ordinary pattern ID creates its editable pattern automatically.
- **Instruments**: both banks of 16 instruments, detune masks, sequence links,
  envelope, timer, sample and PWM parameters. Linked sequence values are shown
  beside the direct settings; click the values to open that sequence. Sequence
  definitions are shared by every instrument referring to the same ID.
  **Load MYI / Save MYI** exchange individual instruments with their sequences
  and optional sample. New exports use the original editor's MYI3 layout.
  **Copy** duplicates a sound while retaining its shared sequence/sample links.
- **Sequences**: up to 256 native 16-bit sequences, with length and repeat,
  ramp/triangle/sine/square generation, signed words, copying and morphing.
  **Modify range** adds or scales selected words without changing loop metadata.
- **Samples**: eight banks of signed PCM, with raw PCM and 8/16-bit WAV import,
  gain, tuning with interpolation, trimming, sign conversion, PCM save and
  independent preview.
  **YMise** quantizes a sample using the original editor's YM DAC tables.
- **Edit**: row ranges, masked block copy/cut/paste with overwrite/overlay/underlay,
  row insertion/deletion, expand/shrink, transposition, attenuation and sound remap.
  Sound remapping can cover the current block or every arrangement pattern
  belonging to the selected YM/PCM track type.
  **Pack project** removes duplicate pattern/sequence definitions and remaps their
  references, including sequence-selection commands in both effect columns.
- **YM**: original YM playback, live register inspection and proposed tracker
  reconstruction, with optional composer-corpus evidence.
- **Settings**: playback rate, speed, edit step, octave, volume, timer mask,
  saved Jam mode, one/two-voice/native-rate PCM modes and macOS MIDI input.

The window can be resized. The waveform at the bottom displays the actual
synthesizer output.
Open, new-path saves, individual instrument exchange, sample import/export and
composer profile loading use the built-in file browser. Navigate folders, scroll
the list, select a file and confirm; a full path can also be typed.

## Keyboard

| Action | Key |
| --- | --- |
| Song play / stop | Space |
| Pattern play | Right Ctrl |
| Edit / preview | Enter |
| Change channel | Tab / Shift+Tab |
| Move row | Up / Down / Page Up / Page Down |
| Move field | Left / Right |
| Note off | Caps Lock |
| Clear cell | Backspace |
| Lower note keyboard | Z S X D C V G B H N J M |
| Upper note keyboard | Q 2 W 3 E R 5 T 6 Y 7 U |
| Open / save | Ctrl+O / Ctrl+S |
| Undo / redo | Ctrl+Z / Ctrl+Y |
| Jam mode | F10 |
| Copy / paste selected row range | Ctrl+C / Ctrl+V |
| Cut selected row range | Ctrl+X |
| Insert / delete row | Insert / Delete |

Instrument and sequence values use hexadecimal notation. Playback settings use
decimal notation. Notes entered during playback and recording target the
currently playing row.

In **Sequences**, choose **Generate / morph** to create envelopes or oscillations.
The selected sequence's length defines the generated length. Ramps hold their
last value; oscillations loop. Morphing fills IDs between two endpoints with the
same length and repeat. **Samples** supports 1.5 dB gain steps and decimal tuning
in semitones, including 0.125-semitone fine steps. These operations support undo.
Sample preview uses the Go PCM voice and native note-rate table: 8287 Hz at C3
and 16574 Hz at C4. Notes above the native range wrap down by octaves.
Title and artist are runtime/export metadata; the separate native MYS/MYV format
does not contain SNDH title/artist tags.
The **Edit** workspace applies operations to the pattern selected in **Patterns**.
Full-track copy/paste starts at row zero; smaller blocks paste at the cursor.
Expand/shrink keep displaced rows in the block clipboard. PCM transposition and
remapping operate on both sample voices and preserve their volume columns.

## Native files and audio

`.MYS` and `.MYV` files are decoded and saved using the original binary layout,
including reserved state, RLE pattern rows, instrument parameters, sequences,
sample offsets and sample tails. Save writes the corresponding native pair.
The three supplied example song/bank pairs round-trip byte for byte.

Unpacked `.SND`/`.SNDH` files created by MaxYMiser can be opened as editable
projects by extracting their native song and voice-bank payloads. This import
is specific to MaxYMiser exports; it is not a general 68000 SNDH player.
Native formats also accept ICE-compressed wrappers. The Go decoder is checked
against streams produced by the original editor's compressor. **Settings → ICE**
enables compression for native project, MYI and SNDH saves. Go-packed files have
also been decoded by the original Atari routine with byte-identical results.
Native SNDH export preserves a locally supplied MaxYMiser replay prefix and
rebuilds song/sample offsets, metadata, replay rate and duration. Opening a
supported SNDH keeps its replay available for saving; a new composition can
choose **Settings → Load SNDH replay**, then **Export SNDH**. Original executable
data is loaded at runtime and is not bundled with the Go application.

YM files are played by YM Player, including its compressed-file support and
chip effects. Their register data is retained separately from the native
tracker project. **Listen YM** and **Listen score** compare the original
recording with the reconstructed candidate.

```sh
 go run ./cmd/maxymiser -song /path/to/song.mys -bank /path/to/voices.myv
 go run ./cmd/maxymiser -song /path/to/song.snd -wav music.wav -duration 30s
 go run ./cmd/maxymiser -song /path/to/song.mys \
   -template /path/to/maxymiser.snd -sndh finished.snd -duration 3m
```

The headless command requires no graphics window. WAV export produces stereo
16-bit PCM at 48 kHz and refuses to overwrite an existing output file.
The graphical editor renders WAVs in the background; choose the duration in
**Settings → WAV export seconds** (up to one hour).

## YM reconstruction and composer profiles

**Range / grid** selects the source frame interval and proposed row spacing.
The default frame grid retains timing detail; coarser grids create fewer rows
and can omit modulation within them. The analysis records this approximation.

Reconstruction estimates notes, timbres and recurring 64-frame patterns. It
cannot recover the original instrument names, private sequence definitions or
pattern boundaries uniquely. A pitch change can be a new note, an arpeggio,
vibrato, a slide or a fixed-period command. The original YM remains available
for comparison.

A composer corpus improves the evidence supporting a candidate. Pitch-relative
and volume-normalized event fingerprints are compared across recordings.
Repeated eight-event phrases are indexed separately. Identical register
recordings are counted once; constant tones receive lower specificity scores.
Scores measure matching support, not the probability of recovering the original
tracker instrument.

```sh
 go run ./cmd/ymlearn -directory /path/to/composer/ym \
   -author "Composer" -output composer-profile.json
```

Load the resulting JSON through **Composer profile** in the YM workspace before
reconstruction. The profile records matches, occurrences and source examples.
The recording itself remains the comparison reference. See
[YM reconstruction](docs/YM_RECONSTRUCTION.md) for the corpus method and its limits.

Different arrangements of a composition can also be compared across composers:

```sh
 go run ./cmd/ymcompare -left /path/to/first-composer \
   -right /path/to/second-composer -output comparison.json
```

The report separates title hints, transposition/tempo-normalized musical
phrases and timbre differences, with source times for checking each example.

## Implementation status

The detailed [feature inventory](docs/FEATURES.md) records current support and
remaining format, editing and playback work.

This is a developer edition. The pattern engine includes sequence playback,
both effect columns, shared YM noise/envelope behaviour, detune masks,
portamento, arpeggios, slides, timer synthesis and two PCM sample voices.
Pattern arpeggios add to instrument arpeggios, fixed periods retain masked
vibrato, and DigiDrums use the native signed PCM-to-YM DAC mapping.

The native timer paths are implemented in Go and feed the YM Player chip.
Subsample timing, oscillator synchronisation, mixed timer combinations and STe
mixing have not yet been validated against a complete Atari recording matrix;
they should not be described as bit-exact hardware emulation. MIDI input is
available on macOS, including notes, program changes, controllers, transport,
external clock and Song Position Pointer. Native A–E channel assignments are
editable in Settings (hexadecimal 00–0F); matching channels can allocate YM
polyphony. The D/E assignments control the two PCM voices. MIDI clocks advance
one tracker row per six pulses while sound sequences retain their replay rate.
Controllers 44–47 queue patterns at the next pattern boundary.
**MIDI output** lists macOS destinations and connects only to the selected one.
The renderer queues transport, clock and the two PCM-pattern voices in MIDI mode.
CoreMIDI sends are handled outside the audio callback. End-to-end output was
checked through a temporary virtual destination; physical hardware and precise
future-timestamp scheduling remain unverified. Physical Sync24 hardware is not
included in this edition.

## Verification

```sh
 go test ./...
 go test -race ./internal/native ./internal/replay ./internal/ymimport
 go vet ./...
```

Checks cover native round trips, truncation rejection, notes and periods,
sequence/effect behaviour, PCM timing, live note triggering, MIDI framing,
YM-reference playback, reconstruction and duplicate-aware corpus evidence.
Cross-composer checks cover tempo changes, transposition, channel reassignment,
noise-only events and misleading filenames.
Longer native example traces compare 8,778 calls under Hatari, including pattern
loops and saved Jam behaviour. Isolated timer tests verify native levels,
frequency steps and MFP divider/data calculations.
See [native replay verification](docs/REPLAY_VERIFICATION.md) for the scope,
method and register-trace verifier.
The [SNDH corpus audit](docs/SNDH_CORPUS.md) covers 674 imported files, 850 native
subtunes and 654 unchanged export/reload checks, with explicit remaining issues.

## Credits

MaxYMiser code and design: Gareth Morris / gwEm. Original design: Mathieu
Stempell / Dma-Sc. Original graphics: Sebastien Larnac / STSurvivor. The replay
format and frequency tables are based on the
[original replay source](https://github.com/gwEm303/maxYMiser_replay).

Go implementation: Olivier Houte / Malakh Software. Native music credits remain
those of their respective composers.
