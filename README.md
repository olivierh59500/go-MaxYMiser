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
  **Insert / Delete** work at the selected position. **Copy range / Paste**
  insert complete four-track order rows and preserve the repeat anchor.
  **Clone track** copies the selected occurrence's pattern to an independent
  definition; other uses of the original pattern remain unchanged.
- **Instruments**: both banks of 16 instruments, detune masks, sequence links,
  envelope, timer, sample and PWM parameters. Linked sequence values are shown
  beside the direct settings; click the values to open that sequence. Sequence
  definitions are shared by every instrument referring to the same ID.
  **Load MYI / Save MYI** exchange individual instruments with their sequences
  and optional sample. New exports use the original editor's MYI3 layout.
  Import reserves sequence/sample IDs referenced by the partition, MIDI settings
  or live voices, including deliberately empty definitions. Allocation failure
  retains the preceding bank and score; successful import supports undo.
  **Copy** duplicates a sound while retaining its shared sequence/sample links.
  The Square/Buzzer/Timer matrix toggles effect masks directly. **Preview** and
  note keys in Instruments/Sequences audition sounds without writing the score.
  Edited parameters update voices using that instrument without restarting the
  arrangement. Assigning a sequence also extends the serialized bank when needed.
- **Sequences**: up to 256 native 16-bit sequences, with length and repeat,
  ramp/triangle/sine/square generation, signed words, copying and morphing.
  **Modify range** adds or scales selected words without changing loop metadata.
  **Cut / Copy / Paste** retains words, length and repeat, using Ctrl+X/C/V or
  the native F3/F4/F5 shortcuts in this workspace. Shared sequence edits refresh
  sounding voices at their current phase, including held final values.
- **Samples**: eight banks of signed PCM, with raw PCM and 8/16-bit WAV import,
  gain, tuning with interpolation, trimming, sign conversion, PCM save and
  independent preview.
  **Length** changes the selected payload directly (0–32768 bytes), keeping its
  prefix and extending with signed silence. The operation supports undo/redo.
  **YMise** quantizes a sample using the original editor's YM DAC tables.
- **Edit**: row ranges, masked block copy/cut/paste with overwrite/overlay/underlay,
  row insertion/deletion, expand/shrink, transposition, attenuation and sound remap.
  Sound remapping can cover the current block or every arrangement pattern
  belonging to the selected YM/PCM track type.
  **All columns** enables or disables the complete copy/paste mask.
  **Pack project** removes duplicate pattern/sequence definitions and remaps their
  references, including sequence-selection commands in both effect columns.
  **Clear song** empties notes and arrangement while keeping the sound bank,
  composition metadata and playback/edit settings. **Clear bank** empties the
  instruments, sequences and eight sample banks while keeping the partition.
  Both stop recording/playback, retain the current save destination and support
  undo/redo. They affect only the currently selected subtune.
- **YM**: original YM playback, live register inspection and proposed tracker
  reconstruction, with optional composer-corpus evidence and source-labelled
  profiles learned from verified SNDH/YM pairs. Experimental corpus profiles
  group retained instrument definitions across recordings and include a separate
  evaluation excluding complete compositions; similarity candidates remain
  distinct from known source IDs. On the one-frame grid, labelled
  instrument recipes are retained where their measured replay improves the
  candidate while preserving its volume accuracy.
- **Settings**: playback rate, speed, edit step, octave, volume, timer mask,
  saved Jam mode, one/two-voice/native-rate PCM modes, native CNF exchange and
  macOS MIDI input.

The window can be resized. The waveform at the bottom displays the actual
synthesizer output.
**Settings → Load CNF / Save CNF** exchanges the original 29-byte `MYM.CNF`
configuration, preserving hardware/display fields. `-config /path/to/MYM.CNF`
loads it explicitly at startup.
**Reload CNF** reapplies the loaded personal configuration after opening a native
song or selecting a subtune. The preference uses the original CNF reload flag;
the composition's tempo, notes, instruments and samples remain those of the
loaded music. Reapplied saved settings mark the project as modified.
`-ym-library /path/to/ym-recordings` indexes candidate YM alternatives for SNDH
files whose native score is not available. A title match is a version hint;
the user chooses the recording before playback or reconstruction.
Open, new-path saves, individual instrument exchange, sample import/export and
composer profile loading use the built-in file browser. Navigate folders, scroll
the list, select a file and confirm; a full path can also be typed.
Opening an editable native song selects its first ordinary arrangement track
and replaces the instrument/pattern view. An unsupported SNDH shows a visible
explanation and preserves the current composition and playback. SNDH files using
other replay formats do not carry editable MaxYMiser instruments or patterns.
Their own music data can still support reconstruction after the player format
has been decoded. Source decoders cover the Mad Max Last Ninja player and a
verified classic player used by Best in Galaxy; see
[YM reconstruction](docs/YM_RECONSTRUCTION.md) for commands,
verification results and the current scope.
Opening a SNDH recognized by these source decoders displays its original note and
instrument identifiers in **YM → SNDH source data**. Inspection preserves the
current composition and playback. The view lists translated definitions,
unsupported synthesis and untranslated pattern commands before **Import editable
excerpt** replaces the score. **Excerpt start:end** selects a range within the
first 6,000 decoded frames; longer analysis is available through `ympair`.
If the default excerpt exceeds native capacity or contains an unverified pitch,
its source definitions remain visible and **Choose another excerpt** selects a
convertible interval. Failed conversion preserves the current composition and
never becomes an importable preview.
Imported source notes use a one-frame grid with generated 64-row patterns.
Unsupported sounds remain silent and named with `?`. Native vibrato and pitch
slide are translated for ordinary tones with a constant zero arpeggio; other
source programs and period-table rounding still require work. **Save as** writes a separate native
MYS/MYV pair, preserving the original executable. This is partial editable import,
not complete playback of arbitrary Mad Max SNDH files.
The decoded noise-attack programs also become editable mixer/noise sequences.
Long ordinary-tone volume envelopes use the generated score's native volume
column, retaining their independent timing, legato and retrigger behavior.
The source view identifies these sounds; their standalone bank preview holds
a constant volume, so their envelope playback requires the generated MYS/MYV pair.
Basic classic fixed-pitch sounds also retain their alternating noise/tone mixer
through generated `M`/`N` commands, using the noise period shared by all voices.
Their original timbre likewise depends on the generated score, as identified
in the source view and conversion report.
The standard Last Ninja bank currently translates all 32 definitions, including
the two finite automatic drum-pitch programs. Other source layouts and period
rounding remain outside a claim of complete original playback.
Dropping a MYS and its matching MYV uses the same validated opening workflow.
Both files are decoded before changing the composition or its playback; dropped
files use **Save as** rather than treating virtual paths as filesystem targets.

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
| Save as | Ctrl+Shift+S |
| Undo / redo | Ctrl+Z / Ctrl+Y |
| Next unused pattern / sequence | Ctrl+N in its workspace |
| Jam mode | F10 |
| Disable Jam | Shift+F10 |
| Previous / next song position | Ctrl+Left / Ctrl+Right |
| Previous / next live pattern | Shift+Left / Shift+Right |
| Octave selection | F1–F8 |
| Percussion note keyboard | F9 |
| Copy / paste selected row range | Ctrl+C / Ctrl+V |
| Cut selected row range | Ctrl+X |
| Insert / delete row | Insert / Delete |

Instrument and sequence values use hexadecimal notation. Playback settings use
decimal notation. Notes entered during playback and recording target the
currently playing row.
**Unused**, or Ctrl+N in Patterns/Edit or Sequences, selects a blank definition
after the current ID and wraps through the native capacity. Stored content,
reserved arrangement rows, instrument links, both effect columns and live
overrides are protected. Pattern selection preserves playback; allocating new
native storage participates in undo. Sequence 00 remains reserved.
**Record** starts song playback with note entry enabled; **Stop** leaves recording
mode. F9 maps note keys to instruments played at middle C. **Help** provides
separate keyboard, effects, instruments, native format/YM and MIDI references.
MIDI notes received in Record/Edit are written into the mapped voice's native
pattern columns and participate in undo, including same-note retriggers.
**Rec pattern** in Patterns records the selected pattern combination. Right
Shift starts pattern recording while stopped, or toggles recording during
playback without restarting the transport. **Follow rows** in Settings follows
the native saved Scroll preference for YM and PCM views.
While playback is running, **Pattern**, **Song** and **Record** switch modes
without restarting the row, envelope state or audio. In song mode, pressing
**Song** again stops playback.
Right-click **Song** to restart from position 00 and row 00. Right-click
**Pattern** to start the selected pattern combination at the cursor row.
In Jam song mode, position changes wait for the current pattern boundary; the
Song workspace shows the queued target. Jam pattern mode queues the selected
track's next pattern. Repeated navigation adjusts the pending target, including
pattern 00. **Stop** discards pending jumps. Track mutes disable score sequencing
while allowing live keyboard/MIDI notes and sample previews, as in the native
performance workflow.

In **Sequences**, choose **Generate / morph** to create envelopes or oscillations.
The selected sequence's length defines the generated length. Ramps hold their
last value; oscillations loop. Morphing fills IDs between two endpoints with the
same length and repeat. **Samples** supports 1.5 dB gain steps and decimal tuning
in semitones, including 0.125-semitone fine steps. These operations support undo.
Sample preview uses the Go PCM voice and native note-rate table: 8287 Hz at C3
and 16574 Hz at C4. Notes above the native range wrap down by octaves.
Title and artist are runtime/export metadata; the separate native MYS/MYV format
does not contain SNDH title/artist tags.
**Settings → Year** edits the four-digit composition year. Own SNDH imports
retain this tag, SNDH exports write it, and native CNF keeps the same four
configuration bytes. MYS/MYV do not store the composition year.
The **Edit** workspace applies operations to the pattern selected in **Patterns**.
Full-track copy/paste starts at row zero; smaller blocks paste at the cursor.
Expand/shrink keep displaced rows in the block clipboard. PCM transposition and
remapping operate on both sample voices and preserve their volume columns.

## Native files and audio

`.MYS` and `.MYV` files are decoded and saved using the original binary layout,
including reserved state, RLE pattern rows, instrument parameters, sequences,
sample offsets and sample tails. Save writes the corresponding native pair.
The three supplied example song/bank pairs round-trip byte for byte.
Both native payloads are encoded, validated, staged and synced before replacing
the saved pair. A failed replacement restores the preceding files, including
when the second file fails. Original filename case and file permissions are
retained. This protects against write/rename failures; it is not a filesystem
transaction that guarantees both files switch together during power loss.
The **As** button beside Save, or Ctrl+Shift+S, chooses a new native destination.
Individual MYI, signed PCM, CNF and native SNDH saves also stage and sync the
complete file before replacing an existing regular target. Instruments and
samples can therefore be edited and saved repeatedly under the same filename.
Existing permissions are preserved; directories and symbolic links are rejected.
Saving a sound independently retains the composition's unsaved-edit state.

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

The Song workspace can select a native subtune directly or move to the next
one. Each subtune retains its own edits, cursor, undo/redo history, replay
template and MYS/MYV save destination. Multi-song source files default to saving
individual native pairs. To export one selected song as SNDH, choose a compatible
single-song replay template; a multi-song selector cannot be retained while
discarding the payloads it references. Shared-bank containers such as Yoomp
expose each stored song independently. Declared songs without a decoded payload
are reported explicitly rather than assigned invented data.
**Song → Export collection** exports every edited subtune using the original
complete selector when it matches the verified relative-offset wrapper. Voice,
song, song-length and per-song rate references are rebuilt together. The current
Settings export duration is written for every song, and ICE packing applies to
the resulting executable too. Collection export keeps each song's independent
native-pair save destination and undo history. Mixed-player, partial and other
selector layouts remain explicitly unsupported.

YM files are played by YM Player, including its compressed-file support and
chip effects. Their register data is retained separately from the native
tracker project. **Listen YM** and **Listen score** compare the original
recording with the reconstructed candidate.

```sh
 go run ./cmd/maxymiser -song /path/to/song.mys -bank /path/to/voices.myv
 go run ./cmd/maxymiser -song /path/to/song.snd -wav music.wav -duration 30s
 go run ./cmd/maxymiser -song /path/to/collection.sndh -subtune 2 \
   -wav second-song.wav -duration 30s
 go run ./cmd/maxymiser -song /path/to/song.mys \
   -template /path/to/maxymiser.snd -sndh finished.snd -duration 3m
```

The headless command requires no graphics window. WAV export produces stereo
16-bit PCM at 48 kHz. Re-exporting stages the complete replacement before
updating an existing regular file, preserving its permissions. The preceding
audio remains readable during rendering and is retained if reading, writing or
committing the new output fails.
The graphical editor renders WAVs in the background; choose the duration in
**Settings → WAV export seconds** (up to one hour).
The adjacent **Song** button measures one arrangement traversal to its first
repeat, respecting speed commands, pattern breaks and Jam sections. Select
internal clock before exporting a tracker WAV: an offline renderer cannot
receive the external timing. The measured traversal is not a
claim that every oscillator or sample loops without a phase difference.
The headless command exposes the same option as `-song-duration`, for WAV or
SNDH exports. WAV rendering handles partial reader chunks and removes its own
staged output on error. Directories and symbolic links are rejected before
audio rendering starts. Editing and live playback remain independent of the
background renderer; each export uses its captured composition or YM reference.

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
polyphony. The D/E assignments control the two PCM voices. Each external MIDI
clock pulse advances the complete replayer, including instrument sequences and
effects. Selecting the clock resets row speed to six pulses, as in the original
editor. The native speed controller can override this live value; pattern `S`
commands and the ordinary speed editor remain disabled with external clock.
**Settings → Latency** accepts 0–255 compensation pulses and applies them on
both Start and Continue. The setting is retained in MYS and native CNF files.
Start, Continue, Stop and Song Position Pointer only control playback when
MIDI clock is selected. An imported Sync24 selection remains preserved; select
MIDI clock to use a MIDI input in its place.
Controllers 44–47 queue patterns at the next pattern boundary.
**MIDI output** lists macOS destinations and connects only to the selected one.
The renderer queues transport, clock and the two PCM-pattern voices in MIDI mode.
External pulses, including native latency compensation, are relayed once per
replay call. Batched pulses retain intermediate notes in their original order.
Continue preserves the current phase; starting at another song position sends
Song Position Pointer followed by Continue instead of resetting the receiver.
MIDI notes use native track transposition and attenuation: total values 0–7
produce velocities 127, 111, 95, 79, 63, 47, 31 and 15; eight or more is silent.
Notes can play without a sample number. With sample zero, a new note precedes
the previous note's release for legato; a sample number selects retrigger order.
The sample number does not automatically send a MIDI program change. Disconnect,
channel changes and switching to the YM reference release sounding notes.
CoreMIDI sends are handled outside the audio callback. End-to-end output was
checked with generated tracker events through a temporary virtual destination;
physical hardware and precise
future-timestamp scheduling remain unverified. Physical Sync24 hardware is not
included in this edition.

**Settings → Controllers** uses the original configuration bit and controls
whether incoming CC messages are applied. Tracker controllers respond globally
on any MIDI channel; instrument controllers edit the assigned YM definition,
including its sequence links, using the native value scales. These bank edits
participate in undo and native saves. The A–E sound selectors support `00` for
disabled input, `01–20` for YM instruments, `DD` for middle-C percussion, and
`01–08` for PCM banks. Channel assignments remain independent.
With DMA enabled, controllers 48–51 adjust Microwire volume, balance, bass and
treble using the original STe value scales. These commands share the pattern
`U` controls and preserve the independent editor volume. Native instruction
checks cover all 128 values of each controller; the analog circuit response
remains approximated digitally.
Native MMC play/stop messages control transport; a remote stop leaves recording
mode. Malformed or unrelated SysEx messages are ignored.

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
The [SNDH corpus audit](docs/SNDH_CORPUS.md) covers 689 imported files, 869 native
subtunes and 658 unchanged single-song export/reload checks, with explicit
remaining issues and incomplete declared collections.

## Credits

MaxYMiser code and design: Gareth Morris / gwEm. Original design: Mathieu
Stempell / Dma-Sc. Original graphics: Sebastien Larnac / STSurvivor. The replay
format and frequency tables are based on the
[original replay source](https://github.com/gwEm303/maxYMiser_replay).

Go implementation: Olivier Houte / Malakh Software. Native music credits remain
those of their respective composers.
