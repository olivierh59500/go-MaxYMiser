# Executable SNDH import

SNDH is a container for Atari replay code. It does not define one common song,
pattern or instrument format. Go MaxYMiser therefore uses three complementary
paths when opening a file:

1. MaxYMiser native payloads are extracted directly. Their editable instruments,
   sequences, patterns and supported collections remain available unchanged.
2. Recognized Mad Max player layouts expose verified source IDs in the source
   inspector. Excerpt conversion is an explicit action, with its own limits.
3. Other players execute in the built-in Go 68000/MFP/STE renderer. Original
   executable audio and an editable excerpt are retained separately.

## Opening and comparing

Open or drop an SNDH file, or start with:

```sh
go run . /path/to/music.sndh
```

Executable analysis runs in the background. An unsuccessful import retains the
current composition and transport. The source file is never used as the save
path of a generated score. Plain and ICE-packed containers are supported.

The **YM** workspace shows **Listen SNDH**, **Listen score**, the live registers
and previous/next song controls. Song numbers are one-based; the initial selection
uses the header default. Returning to a previously selected executable song
restores its edits, save destination, cursor and undo history.
**Execute source song** also reaches declared songs that have no directly
extractable native payload in a mixed-player collection or source inspector.
Listening to the original executes its replay routines,
MFP timer handlers and STE DMA audio, rather than playing a sampled register
trace. Seeking is processed separately from the audio callback; a subsequent
seek, source replacement or close cancels an older request.

The initial editable selection contains 400 reference frames: usually eight
seconds at 50 Hz. **Range** selects the first frame, exclusive last frame and
row spacing. **Reconstruct** analyzes that selection again in the background,
including later passages. A last frame of zero requests a default-length excerpt.
The maximum executed range is 16,320 frames. Players outside the tracker's
25–200 Hz grid use a 50 Hz analysis grid; original playback retains the declared
player-call rate. A coarse row grid can lose modulation between rows.

## What is editable

A register transcription proposes notes, instrument definitions and recurring
64-row patterns. It retains observable tone curves where native capacity allows.
These are inferred definitions: they are not recovered original instrument
names, source patterns, source program instructions or author intent. Timer and
DMA effects can sound substantially different when reduced to editable YM rows.
Use **Listen SNDH** to compare.

When the trace contains no playable YM notes, or exceeds reconstruction capacity,
the importer creates a **sampled excerpt**. Its fourth pattern stream triggers
signed eight-bit PCM chunks, using the native STE sample mode. Samples can be
trimmed, tuned or replaced, and their triggers can be rearranged. This is an
audio capture of the mix, not separation of the original voices or recovery of
the source sound bank. Quantization and resampling change its sound. Eight
sample slots of at most 32,768 bytes hold approximately ten seconds at 25,033 Hz,
independently of the tracker grid. Chunks can span pattern boundaries. Any shortened
selection is reported explicitly. The complete original remains independently
available for playback.

Save the edited excerpt as a new **MYS/MYV pair**. The two files belong together.
Generated scores contain no foreign executable replay template. Native SNDH
export still requires a compatible MaxYMiser replay supplied separately.

## Headless commands

Create a bounded editable candidate and its analysis report:

```sh
go run ./cmd/sndhimport -input /path/to/music.sndh \
  -subtune 2 -start-frame 500 -frames 900 -output excerpt.mys
```

The output includes the companion MYV and `excerpt.mys.analysis.json`. The report
identifies inference or sampled import, the actual selected interval and warnings;
its source hash identifies the supplied executable.

Render the original player without opening a graphics window:

```sh
go run ./cmd/maxymiser -song /path/to/music.sndh -subtune 2 \
  -wav original.wav -duration 30s
```

WAV output is stereo S16LE at 48 kHz. The hardware model currently mixes to mono
and duplicates that result into the two channels; it does not reproduce spatial
STE stereo imaging. Existing complete WAV files survive failed rendering.
`-song-duration` uses the selected executable's declared duration when available;
a header duration is not a proof of a seamless musical loop.

Audit executable compatibility independently of editable extraction:

```sh
go run ./cmd/sndhcheck -directory /path/to/sndh-collection \
  -all-subtunes -seconds 5 -workers 8 -output replay-report.json
```

## Measured compatibility

The supplied local corpus contains **5,897 files and 11,758 declared songs**.
All headers parse and all songs initialize. Five-second replay checks pass for
**5,895 files and 11,750 songs**. Every successful check produces changing PCM.
This tests initialization and the beginning of each song, not complete-duration
playback or hardware-identical fidelity.

Two executable wrappers fail during replay: Kelly / **Last Ninja 2** (seven
songs), and Zerkman / **Ah Que Coucou**. Both also produced silence with the
pinned AtariAudio reference build. Their errors are retained rather than replaced
with invented musical data. Exit-routine errors are reported separately:
386 song exits fail despite successful replay; cleanup still releases the Go
renderer.

A first 400-frame register-only audit produced 5,886 save/reload-stable native
candidates, but **524 contained no playable YM notes**, and nine exceeded the
32-timbre limit. Merely observing nonzero synthesized PCM was insufficient:
chip/filter transients also occur for empty scores. These findings motivated the
sampled fallback. Editable-import coverage and original replay are separate
measurements; neither implies recovery of every original source instrument.

The final 400-frame executable-import audit succeeds for **5,895 files**:
**5,362 inferred register scores** and **533 sampled excerpts**. Every generated
candidate contains an editable note or sample trigger, saves as MYS/MYV and
reloads with byte-identical native payloads. This exercises the universal path
on every file; the GUI still prefers direct native extraction and verified
source inspection where available. Only the two replay failures above remain.

A synthetic volume-DAC player also verifies that reloaded sampled-score audio
correlates with its original waveform above 0.98 after discarding startup
transients. Editing that sample changes the resulting audio. Tests cover chunks
spanning patterns, exact partial-pattern duration, sample-capacity truncation,
selection, cancellation, failed imports, independent subtune undo/save paths
and asynchronous seek replacement. The complete race-enabled suite and `go vet`
pass. Interface captures checked a foreign Mad Max import and a Quartet sample
bank; a separate thirty-second Mad Max original WAV was rendered successfully.
These checks do not establish full-song source-instrument recovery or hardware
fidelity for the whole corpus.

The earlier [native corpus audit](SNDH_CORPUS.md) remains relevant to direct,
lossless MaxYMiser payload extraction and native executable export.

## Implementation and attribution

The renderer is Go code, using YM Player for YM2149 synthesis. Each source owns
its machine state and bounded memory. Instruction budgets limit initializer,
replay and timer calls. The header reader checks sizes, song arrays and ICE
output limits. Background imports are cancellable between execution blocks.

The hardware design follows the MIT-licensed
[Arnaud Carré sndh-player](https://github.com/arnaud-carre/sndh-player) and
[AtariAudio](https://github.com/arnaud-carre/AtariAudio), whose notice is retained
in `internal/sndh/ATARIAUDIO_LICENSE`. The Go CPU core comes from
[John Schember's go-chip-m68k](https://github.com/user-none/go-chip-m68k), pinned in
`internal/sndh/m68k/ORIGIN`, with its MIT license retained. SNDH compatibility
allows unaligned data accesses; instruction addresses remain checked. This
compatibility mode is explicitly separate from the CPU's strict default.

This is not cycle-exact Atari emulation. Analog filters, precise interrupt phase,
combined hardware effects and complete-song fidelity need further reference
comparisons. No external emulator, C++ runtime or original music corpus is
required or bundled with the application.
