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
The separate noise and hardware-effect programs are not yet fully translated.

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

Without `-ym`, `ympair` exports only the native source tables and timeline.
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
generic instruments. Last Ninja currently yields 24 translated definitions
and eight unsupported ones. The original first-step envelope cadence is
preserved, including speed-zero envelopes advancing before the first output.
Native execution checks covered 52 triggered notes and 1,186 volume-register
values with no mismatches. Pattern-controlled vibrato and slides remain
outside this base bank conversion.

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
