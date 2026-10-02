# YM reconstruction and composer evidence

A YM register recording contains the output of the original replayer rather
than its editable score. The same trace can be produced by several combinations
of notes, fixed frequencies, arpeggios, vibrato, slides and sequence definitions.
Reconstruction therefore creates a candidate, never an authenticated recovery
of the original instruments.

## Composer corpus

The learner decodes the register recordings and removes exact duplicate traces.
It detects candidate onsets, then represents each event with 16 normalized
feature samples: relative pitch, relative volume, mixer state, active noise
period and active envelope shape. Inactive shared noise/envelope registers are
ignored so another channel cannot invent a different instrument fingerprint.

A family is retained only when it occurs in at least two distinct recordings.
Eight-event phrases are indexed separately using pitch intervals and relative
durations. Source filenames and occurrence counts remain attached to the
result, making the evidence reviewable.

The matching score combines normalized feature distance with information
content. Constant tones receive a lower specificity score because many
instruments produce the same unchanging waveform. A score of 100 means an exact
normalized feature match, not a 100% probability of finding the original
instrument.

The Mad Max / Jochen Hippel directory contains 513 YM files. All decode through
the register importer; two traces are identical, leaving 511 distinct
recordings and 3,529,873 frames. The current detector finds 5,776 recurrent
timbral fingerprints and 17,220 recurrent phrases. These counts include
variants and common steady sounds; they are not a count of original instruments.

## Trying the pipeline

```sh
 go run ./cmd/ymlearn -directory /path/to/Mad-Max \
   -author "Mad Max / Jochen Hippel" -output madmax-profile.json

 go run ./cmd/ymimport -input /path/to/music.ym \
   -profile madmax-profile.json -output candidate.mys
```

The second command writes `candidate.mys`, `candidate.myv` and an analysis JSON.
In the application, open the original YM, load **Composer profile**, choose
**Reconstruct**, then compare **Listen YM** and **Listen score**. The candidate
can be edited in the native tracker workspaces and saved independently.

A Roll out 1 test currently proposes five timbres, 32 frame-grid patterns and
25 order positions, with 28 corpus matches. This is an editable transcription;
its patterns are not claimed to be the original composition's patterns.

## Selection and proposed row grids

Long recordings can be reconstructed in a selected frame range. `-start-frame`
and `-end-frame` use the original YM timeline, with an exclusive end; zero end
means the entire recording. `-row-frames` selects how many source frames make
one editable tracker row. The default one-frame grid retains the finest timing.

```sh
 go run ./cmd/ymimport -input /path/to/music.ym \
   -start-frame 500 -end-frame 3500 -row-frames 6 -output excerpt.mys
```

The YM workspace's **Range / grid** control exposes the same options. Zero row
spacing requests an onset-alignment estimate, and the analysis report records
the ranked candidates. A coarser grid samples states and can omit vibrato,
arpeggio or envelope changes within the row. It is a proposed arrangement,
not proof of the original tracker speed. The original YM remains independently
available for listening and register inspection.

## Limits

The first transcription stage uses a frame grid and semitone pitch estimates.
Original musical row spacing, instrument names and causal assignments between
melody and arpeggio are not uniquely recovered. Hardware envelope periods,
retrigger timing, DigiDrums and timer-driven waveforms require additional
hypotheses. Those effects remain audible in the separately retained YM
reference. The corpus provides supporting examples and stronger comparisons;
it does not turn ambiguous traces into unique source data.

## Learning from paired SNDH and YM files

A SNDH contains executable playback code and that player's music data. It can
therefore provide source instrument IDs, note commands and pattern boundaries
that a YM register recording does not retain. These labels make supervised
learning possible once the particular player has a verified decoder. A SNDH
does not need to use MaxYMiser to be useful for this purpose.

SNDH is a container, not one common tracker format. Author and title metadata
do not identify a safe data layout. The first source decoder recognizes the
Mad Max player carried by the standard and SID versions of **Last Ninja**. It
uses instruction signatures and file-relative pointer validation, retains the
original 27 referenced patterns and 32 instrument definitions, and simulates
the source note/control timing. The alternate instrument-bank command and
additional subtunes are rejected until their layouts are verified.
Instrument extraction retains the six base settings, the volume sequence and
the signed arpeggio sequence, including its cadence and hold/repeat behavior.
It also retains the validated noise-attack programs described below. Other
hardware-program layouts still require their own extraction and translation.

An audit of the supplied Mad Max SNDH directory checked all 357 files, each with
a distinct unpacked payload, against the source decoder with a 6,000-frame
analysis limit. Only `Last_Ninja.sndh` and `SID/Last_Ninja.sndh` decoded; both
yielded 32 translated instrument definitions. The remaining 355 files were
rejected at player-layout recognition. These counts describe source extraction
coverage, not full audio fidelity or cross-song model accuracy. Related parser
instructions occur in other files, but those short signatures do not establish
compatible tables, commands or replay behavior.

Expanding paired learning therefore requires verified source decoders for more
players, checked SNDH/YM alignment and validation on entire compositions absent
from the training set. Instrument IDs must remain local to each source bank;
combining identically numbered instruments from unrelated songs would create
incorrect training labels.

Native execution under Hatari provided 298 note events with their source
offsets, instrument IDs, timing and legato flags. Every event agreed with the
Go source decoder. Original binaries, captured memory and learned music data
remain external local inputs; they are not included in the repository.

```sh
 go run ./cmd/ympair -sndh /path/to/Last_Ninja.sndh \
   -ym "/path/to/Last Ninja.ym" -frames 6000 -output last-ninja-paired.json

 go run ./cmd/ympair -sndh /path/to/Last_Ninja.sndh \
   -output source-score.json -bank source-instruments.myv

 go run ./cmd/ymimport -input /path/to/music.ym \
   -paired-profile last-ninja-paired.json -output candidate.mys
```

Without `-ym`, the JSON output retains the native source tables and timeline.
The optional `-score` output translates the selected source-note excerpt to an
editable native MYS/MYV pair and writes a conversion report alongside it:

```sh
 go run ./cmd/ympair -sndh /path/to/Last_Ninja.sndh -frames 6000 \
   -start-frame 0 -end-frame 6000 -score source-excerpt.mys \
   -output source-labels.json
```

The original source IDs remain in `source-labels.json`. Generated patterns use
64 tracker rows at one source frame per row; they are not a recovery of the
source player's variable-length pattern encoding. A shortened last pattern
ends at the selected frame. Cropping a sounding note restarts its envelope;
earlier modulation phase is not restored.

The graphical **Open** action also recognizes the supported source player. Its
inspection view retains the current composition and audio until **Import editable
excerpt** is selected. It lists raw base settings, converted sounds, unsupported
definitions and untranslated commands. Source imports have an independent MYS/MYV
save destination and retain no foreign executable as a MaxYMiser export template.
Opening an unrelated native project, a YM or a new project clears the old source
inspection. Dropped source SNDH files use the same inspection workflow.

For the first 6,000 standard Last Ninja frames, conversion produces 86 generated
patterns, 94 order positions and 45 sequences. Saving and reloading the native
pair retains all 1,561 note/rest events, including their source instrument IDs
and timestamps, with an exact 120-second excerpt traversal. No note events in
this excerpt use an unsupported instrument. Pattern commands 82 and 84
are translated for ordinary tone programs with a constant zero arpeggio. These
data checks do not establish complete sound fidelity:
unsupported definitions stay silent and marked `?`, while the translated sounds
retain the base volume/arpeggio behavior described below.

### Source vibrato and pitch slide

The source timeline retains the exact execution frame, offset and operands of
vibrato enable/disable, pitch slide and instrument selection. Note events remain
independent, so an effect does not become an invented note or pattern boundary.

Original 68000 arithmetic probes establish the triangle's extra low-end hold,
its octave-dependent period scale and its initial delay. Pitch-slide probes
establish the signed 16-bit accumulator and the initial delay before updates
occur on every subsequent replay call. A retrigger resets the vibrato delay;
legato retains its running phase. Instrument selection and vibrato selection
retain their distinct phase changes.

The excerpt exporter writes these period deltas as editable MaxYMiser vibrato
sequences, split before instrument retriggers and at the 63-word sequence limit.
Rendering checks cover sequence/pattern boundaries and native bank round trips.
The 6,000-frame example contains 1,157 assigned pitch segments, reusing sequence
definitions. Hardware/noise programs and nonconstant source arpeggios remain
outside this pitch conversion, and native period-table rounding still differs
from MaxYMiser's base pitch table. These checks establish the modulation values
and timing, not complete audio parity with the original song.

With `-ym`, it first aligns source notes with the register recording. The
search supports a recording lead-in and a constant pitch transposition, and
requires at least 20 tonal events with 90% pitch agreement. It does not assume
that similarly named files are the same arrangement. Different frame rates,
tempo changes and channel permutations need a different alignment method.
Pitch agreement verifies a musical timeline, not an identical timbre or replay
variant: the SID variant can share notes with the standard recording. Labels
describe the bank selected by `-sndh`; register-level verification is needed
before treating two versions as identical sound definitions.

The labelled model stores normalized feature prototypes associated with native
instrument IDs. The last chronological quarter is excluded from training.
The first few native envelope values refine each training boundary within two
YM frames of the global alignment. This accounts for trigger sampling without
changing the source note timeline. Events crossing the training cutoff are
excluded rather than leaking later register values into an example.
Ambiguous matches and unfamiliar sounds produce no label. Source IDs belong
to that original bank; the same number in another song does not identify the
same instrument. Neither distance nor matching margin is a probability.

For the supplied standard Last Ninja pair, the first 6,000 source frames align
at an offset of six YM frames: 846/849 eligible tonal events agree. The held-out
section contains 358 examples of instruments encountered during training:
313 labels are correct and 45 are left unresolved, with no accepted incorrect
labels in this test. This is about 87% coverage as correct labels. Validation
still uses repeated material from one song; these numbers
do not establish performance on another composition or on all Mad Max music.

In the application, **YM → Paired source profile** loads the generated JSON;
**Reconstruct** includes source-labelled candidates alongside the existing
composer evidence. `-paired-profile /path/to/profile.json` selects the same
profile at startup. The analysis report records each accepted event's source
instrument, channel, frame range, feature distance and competing-label margin.

### Editable instrument conversion

`ympair -bank` writes a native MYV containing the supported square-wave
envelopes and arpeggios. Source ID zero maps to tracker instrument 01, and so
on. Unsupported hardware programs retain their source data in the JSON and
are listed in the separate MYV analysis report; they do not become invented
generic instruments. Last Ninja currently yields 32 translated definitions.
The original first-step envelope cadence is
preserved, including speed-zero envelopes advancing before the first output.
Native execution checks covered 52 triggered notes and 1,186 volume-register
values with no mismatches. Pattern-controlled vibrato and slides remain
outside this base bank conversion.

Six additional Last Ninja sounds combine an arpeggio with a short native noise
attack. The decoder retains the program's validated pointer, operation and
period bytes. The player skips the first pair before its first output, executes
one remaining pair per call, and holds the mixer state at the final FF marker.
These operations become independent editable mixer and noise sequences.

An original 68000 probe confirms noise-only mixer 37 on the first call and tone
mixer 3E from the second call, with no extra startup frame. The first noise write
is 2F; only its low five bits select the chip period, so the translated sequence
uses 0F. The arpeggio and volume sequence continue independently. Synthetic
round-trip and replay checks retain this timing without copying the original
music into the repository.

The two drum definitions also select automatic pitch programs: a one-semitone
descent per replay call and a signed period accumulator advancing by +72.
Original calls from a 477-period starting tone produce 578, 680 and 784 while
the stored note descends and the accumulator reaches 72, 144 and 216. Mixer
state follows the independent noise program.

These programs become editable arpeggio and vibrato-word sequences for their
finite audible duration. Conversion requires a zero source arpeggio and a final
silent volume step; an unbounded audible tail remains unsupported. The last
pitch value holds only after the volume becomes silent. Source-excerpt export
also preserves the accumulator's phase across assigned sequence segments.
MaxYMiser's base period-table rounding remains distinct from the source table,
so the conversion does not claim identical hardware output at every note.

Paired training also produces editable instrument recipes for registered
volume, relative pitch, vibrato correction, mixer and noise sequences. Different
phases of the same source sound can yield different recipes. They are synthesis
candidates extracted from labelled output, not a claim that the original
tracker used those MaxYMiser settings. Envelope and timer programs requiring
another replay model are omitted.

On the default one-frame reconstruction grid, the importer tries a bounded
set of source-labelled recipes. It renders each candidate and accepts it only
where register error decreases without increasing volume error or changing a
previously correct mixer state. Shared patterns are cloned before a selected
occurrence is edited, and the following transcription state is restored. A
coarser grid keeps the ordinary transcription with source labels only.

For the first 6,000 Last Ninja frames, 126 passages accepted a recipe. The
total absolute tone-period error decreased from 73,149 to 68,648 units across
audible tone frames; total volume error remained zero and the mixer mismatch
count remained 2,021. This is a measured pitch improvement, not complete audio
fidelity. The remaining mixer discrepancies, continuous modulation phases,
hardware programs and native pattern structure need further reconstruction.
The report includes the before/after error for every accepted passage.

### Source-pattern candidates

Paired profiles retain complete source-pattern occurrences as 64-sample phrase
fingerprints, including relative pitch, volume, mixer, noise and envelope.
Training uses the original order boundaries and excludes passages crossing the
held-out quarter. Constant tones and nearly empty patterns do not supply enough
musical information and are omitted.

On import, the matcher searches independently detected phrase starts in the
selected YM range. It does not copy the training arrangement's timestamps.
Matching permits pitch transposition and another channel while retaining the
learned duration. Adjacent sampling offsets are combined into one candidate,
and equally plausible source IDs remain listed together.

In Last Ninja's held-out section, 22 of 26 passages with a known source pattern
are recognized. Of 30 candidate passages, 22 include the expected source ID and
agree with its boundaries. The remaining eight are recorded in the validation
report. Some repeated material supports an internal or shifted boundary, so
these candidates are not used to rewrite the arrangement automatically.

**YM → Patterns** displays the candidate frame ranges and source IDs. Click a
passage to hear that position in the original YM. This browsing preserves the
editable composition. Source-pattern IDs describe the original player's data;
they are not the same as the generated MaxYMiser pattern numbers.

If the imported register recording is exactly the one associated with the
paired profile, the view instead uses that source's known pattern IDs and
order boundaries. Equality requires the chip clock, frame rate, length and
every decoded register frame to match; a filename or title never selects this
path. The profile's SNDH/YM association still identifies the source variant,
and displayed time ranges use its verified alignment. Recordings with sample
or timer payloads are excluded from register-only identity checks. A changed
recording falls back to candidate matching, and older profiles remain readable.

## Comparing arrangements from different composers

Two arrangements of a common composition are complementary evidence. A phrase
that survives different sounds, tempos or channel assignments is more likely
to belong to the musical content. The remaining register differences can then
be examined as arrangement and synthesis choices. This still cannot prove which
private instrument or effect definition produced them.

```sh
 go run ./cmd/ymcompare -left /path/to/Hubbard-Robb \
   -right /path/to/Mad-Max -output comparison.json
```

The comparator checks all channel pairs and all recordings, including files
with different titles. It excludes noise-only events, collapses consecutive
equal pitches and indexes eight-pitch interval phrases. Matching relative
onset durations permits a different tempo. A match is extended to at most
32 notes; the report separately counts contexts of at least 16 notes.
Filename similarity is only a candidate hint. Common scales and constant
interval repetitions are excluded. Repeated windows can overlap, so counts
must not be interpreted as independent sections of a song.

Each example includes both filenames, channel indices, source times,
transposition, duration ratio, rhythm error and normalized timbre distance.
The report provides reviewable evidence, not an automatic authorship or
instrument attribution. A zero result can also mean the onset detector missed
an arrangement with extensive modulation.

A local comparison of eight Hubbard-directory recordings against 513
Mad Max-directory recordings decoded every file. Selected results are:

| Recordings | Eight-note phrase matches | Contexts with 16+ notes |
| --- | ---: | ---: |
| Thrust / Big - Thrust | 33 | 20 |
| Thundercats / Union Demo - Thundercats | 140 | 111 |
| Warhawk / Big - Warhawk | 18 | 4 |
| Goldrunner / Little Color Demo - Human Race 2 | 35 | 20 |

The Thundercats examples have a median right-to-left duration ratio of 1.2.
This describes the aligned passages rather than a global tempo for the whole
recording. Goldrunner's differently named candidate illustrates why comparing
only filenames would miss useful evidence. Conversely, Warhawk / Warhawk remix
currently yields no accepted phrases; the filename alone cannot confirm an
alignment. Directory placement is not verified composer metadata.
