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
a distinct unpacked payload, against the source decoders with a 6,000-frame
analysis limit. The current decoders extract 48 files: the two Last Ninja
versions and 46 classic files from the Best in Galaxy collection. Of these,
44 produce an editable 6,000-frame excerpt with long-envelope conversion.
Two excerpts contain source note 127, whose current pitch interpretation lies outside the verified tracker
mapping; two exceed the native 240-pattern capacity. All four remain
inspectable and can produce shorter editable ranges. A further classic file has
an invalid arpeggio definition; 308 files use other player layouts. These counts describe
source extraction and conversion coverage, not full audio fidelity or cross-song
model accuracy.

Expanding paired learning therefore requires verified source decoders for more
players, checked SNDH/YM alignment and validation on entire compositions absent
from the training set. Instrument IDs must remain local to each source bank;
combining identically numbered instruments from unrelated songs would create
incorrect training labels.

### Corpus profiles and whole-composition evaluation

`ympaircorpus` reads an explicit manifest of candidate pairs. Paths may be
absolute or relative to the manifest. Every pair must pass pitch/timing alignment
before training. Use one profile per verified player family, frame rate and chip
clock. The manifest's `composition_group` keeps every recording and arrangement
of the same composition together:

```json
{
  "version": 1,
  "pairs": [
    {"composition_group": "song-a", "sndh": "song-a.sndh", "ym": "song-a.ym"},
    {"composition_group": "song-b", "sndh": "song-b.sndh", "ym": "song-b.ym"}
  ]
}
```

```sh
go run ./cmd/ympaircorpus -manifest pairs.json -frames 6000 \
  -output cross-song-report.json -model corpus-profile.json

go run ./cmd/ymimport -input another-song.ym -end-frame 1200 \
  -paired-profile corpus-profile.json -output candidate.mys
```

The evaluation leaves out a complete composition group at a time, including all
of its variants. Duplicate unpacked source hashes, decoded register streams and
normalized source-note timelines cannot be assigned to different groups. These
checks do not replace deliberate grouping of arrangements that change timing.
The default analysis covers the first 6,000 decoded frames of each source; it
does not measure complete-song fidelity.

Instrument classes compare the retained definition, player and frame rate.
File offsets and local instrument numbers are excluded; an arpeggio's local
index is replaced by its resolved values, cadence and repeat. Noise programs and
volume settings remain part of the identity. A similar audible fragment does
not establish that two complete native definitions are identical.

The report separates classification using known source-note boundaries from
recognition using independently detected YM events. The latter predicts from
YM data alone, then scores one-to-one source onset matches within two frames.
Unknown definitions, missed notes and accepted additional segments are counted
explicitly. Accepted additional segments can include native legato changes;
they are not automatically evidence of an incorrect sounding note.

A first evaluation pairs **Ace 2, Commando, Warhawk, Crazy Comets and Human
Race** from the verified classic family with their supplied YM recordings. Across
the five excluded-composition folds, 207 of 7,367 eligible events use definitions
present in the training music. Classification at source boundaries labels 89 of
these correctly, abstains on 118 and accepts 643 events whose complete native
definition was not in training. The independently detected YM onsets match
5,684 source events, correctly label 193 of the 207 known-definition events,
accept 326 unknown-definition events and produce 352 accepted additional
segments. These results demonstrate limited transfer and inadequate rejection
of unfamiliar definitions, not reliable recovery of arbitrary original banks.

The model trained on all five compositions contains 51 observed definitions,
553 feature prototypes and 1,112 editable synthesis recipes. It remains an
experimental similarity model. The tracker loads it through **YM → Paired
source profile**, identifies its labels as **Corpus candidates**, and retains
the independent YM reference. Original pattern IDs and score timelines are not
merged into this model. Its training groups also appear in reconstruction
reports, so a target included in training cannot be presented as an unseen test.

Recipe pitch trajectories are fitted to the target's first observed tone;
absolute vibrato corrections are recalculated for that pitch. Independent
regressions verify octave changes while keeping volume and the learned recipe
unchanged. A fitted recipe is still accepted only when its measured register
error improves the proposed passage without degrading volume. Blind 1,200-frame
imports of Warhawk, Commando and Crazy Comets, each trained on the other four
compositions, accept no substitutions at present. They preserve the existing
transcription, with identical register playback after native MYS/MYV save and
reload. Improving useful transfer remains separate from merely adding corpus
files or reporting a high precision on already-known sounds.

Native execution under Hatari provided 298 note events with their source
offsets, instrument IDs, timing and legato flags. Every event agreed with the
Go source decoder. Original binaries, captured memory and learned music data
remain external local inputs; they are not included in the repository.

### Classic player family

`mad-max-classic-1988-v1` identifies a complete 5,452-byte program by its
normalized digest, with independent music/program relocation bases and checked
initializer references. A short parser signature or matching title does not
select this decoder. The source code contains the digest and pointer layout;
the original executable and its music remain external inputs.

The decoder retains the classic command set: `8C` has no operand, `90` consumes
one byte and schedules a step without stopping the sounding note, and a legato
`80` leaves the envelope active. Fixed-pitch instrument selection consumes one
opaque following byte and triggers native note 16; later ordinary note commands
retain their source bytes while identifying the imposed pitch. An order `FE`
stops all channels together. Later-player commands `91`/`92`, per-step pitch
commands and unverified global transpose changes are rejected.

Native execution verifies 223 Ace 2 and 289 Commando note triggers across 1,200
calls each, including their timestamps, source offsets, instrument IDs and
legato flags. Commando includes 27 fixed-pitch triggers. Sanxion Title stops at
frame 8,628 in both implementations, with all following native volume registers
zero. Native save/reload and rendering now retain both the source instrument's
long volume envelope and all 1,200 verified Ace 2 vibrato deltas, without a
replacement constant-volume voice. A separate synthetic native score checks
660 volume-register values through timed decay, legato and retrigger.

The Ace 2 SNDH/YM pair aligns at two YM frames with 518/518 eligible tonal events
agreeing. In the held-out chronological section, 228 of 302 known instrument
events are labelled correctly, 73 remain unresolved and one accepted label is
incorrect. This remains a same-song evaluation. Combined hardware programs
and additional mixer/noise effects can remain unconverted;
recognized source notes do not imply complete editable sound reproduction.

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
ends at the selected frame. Cropping a sounding note restarts its instrument
sequences; long envelopes represented by score volume retain their source
volume phase. Earlier arpeggio/mixer phase is not restored.

The graphical **Open** action also recognizes the supported source players. Its
inspection view retains original labels even if the default excerpt cannot be
converted. The current composition and audio remain active until **Import editable
excerpt** is selected. It lists raw base settings, converted sounds, unsupported
definitions and untranslated commands. Source imports have an independent MYS/MYV
save destination and retain no foreign executable as a MaxYMiser export template.
Opening an unrelated native project, a YM or a new project clears the old source
inspection. Dropped source SNDH files use the same inspection workflow.
Conversion failures display their specific reason and replace the import button
with **Choose another excerpt**. Selecting a valid range makes import available;
an invalid selection retains the previous valid preview. Excluded out-of-range
source pitches do not invalidate a selected interval, but a pitch sounding at
the selection boundary must have a verified mapping. Values are not clamped or
substituted with arbitrary notes.

In the supplied corpus, Crazy Comets and Sanxion Loader convert over frames
`0:2000`, while Hunter Patrol and Geoff Capes Strongman convert over `0:3000`.
Long-envelope volume commands make the latter's complete default excerpt exceed
the native pattern capacity. These are editable excerpts
with their normal unsupported-sound report; the shorter selections do not prove
complete original sound or full-song conversion.

For the first 6,000 standard Last Ninja frames, conversion produces 114 generated
patterns and 94 order positions. Saving and reloading the native
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
The 6,000-frame example contains 1,159 assigned pitch segments, reusing sequence
definitions. Hardware/noise programs and nonconstant source arpeggios remain
outside this pitch conversion, and native period-table rounding still differs
from MaxYMiser's base pitch table. These checks establish the modulation values
and timing, not complete audio parity with the original song.
Repeated zero arpeggio words are treated as a constant zero arpeggio too; their
presence does not disable otherwise supported vibrato or pitch-slide conversion.

### Long volume envelopes in the score

Ordinary tone definitions whose expanded volume sequence exceeds 63 words can
retain their complete envelope in the generated pattern volume column. Their
bank definition holds level 15; native column attenuation writes exact source
levels 0–15 at each required frame. This leaves the two effect columns available
for independent pitch modulation and other commands.

The source envelope's first value lasts N calls, later values last N+1, and the
final value holds. Retrigger rewinds the pointer; legato reloads cadence without
rewinding it. Classic `90` pauses the pointer and applies its separately timed
volume decay. Cropped excerpts keep the source volume at their starting frame.
Original source settings and envelope bytes remain unchanged in the inspection.

The conversion report lists `pattern_volume_instruments` and the number of
volume commands. The source view identifies these definitions as driven by
their score. A standalone MYV cannot encode that longer envelope and its base
preview holds a constant level; keep the generated MYS/MYV pair for playback.
Other unsupported hardware programs remain silent and explicitly reported.
Additional volume rows can increase the number of distinct native patterns;
conversion rejects capacity overflow rather than dropping envelope changes.

The first 6,000 Ace 2 frames now produce 92 patterns and 314 score-volume
commands, with no note events using an unsupported definition in that excerpt.
This verifies the translated volume/pitch behavior described above, not every
original mixer/noise command or complete analog sound fidelity.

### Classic fixed-pitch mixer and shared noise

Basic classic definitions with flag `02` now retain their original alternating
noise/tone behavior through generated `M` and `N` commands. The period comes
from the shared native noise shadow, including changes made by other voices.
New note steps reset the alternation phase; cropped selections preserve the
phase at the chosen frame. Original fixed-note/source flags remain unchanged
in the source inspection and JSON.

The editable bank retains each sound's translated volume and arpeggio. Its
generated score supplies mixer/noise timing, so standalone MYV preview does not
claim the original timbre. `pattern_mixer_instruments` and
`pattern_mixer_changes` identify these dependencies in the conversion report;
the source view labels the affected definitions too.

The classic `8F` noise sweep follows the native common-tail behavior: the command
sets its counter to `40`, but the following note/wait tail resets it to `30`.
Eight replay calls advance the byte by two to `40`, then hold. The shared shadow
changes after that voice's current noise output, retaining the original call
ordering. An instrument-triggered fixed note skips the ordinary note's noise
assignment before joining the common tail. The generated fixed-pitch M/N score
now incorporates this sweep state rather than excluding those definitions.

After native pair save/reload, Commando's 312 captured fixed-pitch calls match
the original active mixer bits, audible shared noise period and volume values.
A synthetic native score verifies another 120 calls with a different voice
changing the shared shadow. Another 120 native calls verify a sweep on the
fixed-pitch voice, including its counter reset and shared-noise output. The mixer
model was also checked against 1,200
complete calls from both Commando and Ace 2, plus 220 synthetic calls. These
checks concern the stated registers and timing, not complete analog waveform
parity or every original sound definition.

The first 6,000 Commando frames generate 100 patterns and 1,517 mixer/noise
commands; 57 events still use other unsupported definitions. End-of-excerpt
commands occupy an available effect column and never overwrite the last mixer
or pitch command. Full effect rows and native capacities remain explicit errors.

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

## Classic subtunes and global transpose

The verified classic player also supports its native multi-song table. The
zero-based `ympair -subtune` option selects that song's channel orders and initial
speed; invalid table pointers or undeclared selections remain errors. The source
inspector offers Previous/Next song while keeping the current composition and
playback unchanged until an editable excerpt is imported. Selection failure
retains the preceding inspection. The Last Ninja decoder still accepts only its
verified first song.

Classic command 89 changes a shared signed-byte transpose. Held notes on all
three voices change pitch on the same replay call, including voices parsed
before that command, without restarting their envelopes or modulation phase.
Pitch-only source events preserve this distinction; generated tracker rows omit
instrument retriggers for held notes. Tests cover multiple controls on one call,
restoring the initial -12 offset and native-pair save/reload.

Constructed native music executed by the original classic program verifies 24
calls of global transposition across all three cached period outputs. A separate
second-song fixture verifies another 24 calls, its independent three-call step
speed and selected note periods. These compare source interpretation against
the original period table; existing Go/source period-rounding and unsupported
hardware-program limits remain unchanged. The executable remains local.
