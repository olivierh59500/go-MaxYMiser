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

## Limits

The first transcription stage uses a frame grid and semitone pitch estimates.
Original musical row spacing, instrument names and causal assignments between
melody and arpeggio are not uniquely recovered. Hardware envelope periods,
retrigger timing, DigiDrums and timer-driven waveforms require additional
hypotheses. Those effects remain audible in the separately retained YM
reference. The corpus provides supporting examples and stronger comparisons;
it does not turn ambiguous traces into unique source data.

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
