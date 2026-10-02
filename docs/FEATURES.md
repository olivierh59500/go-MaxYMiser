# MaxYMiser Go feature status

This inventory relates the Go edition to the useful editing and playback
functions of MaxYMiser FM 1.67. It distinguishes editable format support from
replay validation and hardware-specific integration.

| Area | Implemented | Remaining verification or work |
| --- | --- | --- |
| Tracker | Three YM voices, two PCM voices, both effect columns, song/pattern recording, native Scroll control, song-start/pattern-cursor playback, safe unused pattern/sequence selection, masked block editing, transpose/remap, row tools, octave/percussion shortcuts and undo/redo | Further platform-specific keyboard conveniences |
| Arrangement | Independent pattern lists, position insertion/deletion and clipboard ranges, occurrence cloning, length/repeat, Jam jumps, seamless song/pattern/record changes, sequencer-only mutes, independent song/bank clearing with undo and validated pattern/sequence packing | Broader hardware-controlled live performance verification |
| Instruments | 32 definitions, live scalar/mask editing, serialized sequence links, MYI0–3 exchange with independent stride/sample fixtures, score/live reference reservations and copying | Broader original instrument-library verification |
| Sequences | 256 sequences, live length/repeat/word edits, phase-preserving shared refresh, native clipboard shortcuts, generation, signed values, range modification and morphing | Further native timer phase comparisons |
| Samples | Eight banks, signed PCM/WAV import, gain, interpolated tuning, trim, direct length changes with signed-silence extension, sign conversion, native YMise DAC quantization, STe DAC cadence/rate fixtures, save and preview | Analog mixer/filter response is approximated digitally |
| Native files | MYS/MYV lossless round trips, MYI0–3 decode/MYI3 export, CNF exchange/reload, composition-year metadata, independent subtune editing, shared-bank SNDH import, guarded single-song export, complete collection export for verified relative selectors and ICE packing | Other multi-song selectors, aliases and additional optimized layouts |
| YM | YM Player reference playback, register inspector, candidate score with selection ranges/explicit grids, onset-alignment proposals, composer corpus, cross-arrangement comparison, verified source-labelled SNDH/YM profiles, source envelope/arpeggio/noise banks, explicit source inspection/editable excerpts with verified ordinary-tone vibrato/slide, measured instrument recipes and browsable source-pattern candidates | More source-player decoders, cross-song validation, original arrangement recovery, other source pattern effects and hardware-program reconstruction |
| Replay | Sequences, commands, native frequency/DAC tables, timer waveforms, PCM note rates/modes | Broader full-song and mixed-timer evidence; sample-grid timer scheduling remains distinct from cycle-exact hardware |
| MIDI | macOS notes, program changes, native controller enable/scales and editable bank changes, verified STe Microwire controller quantization, complete MMC play/stop, external clock/SPP with full replay cadence, Start/Continue latency compensation, channel/sound/DD mapping, duplicate-channel allocation, live PCM transpose/attenuation, selected CoreMIDI output ports, external-clock relay and native note/legato ordering | Physical Sync24 hardware; tighter output timestamp scheduling and physical-device timing measurements |
| Workflow | Resizable interface, visually checked editor/source/file panels, Unicode field erasure, drag/drop, Save as, rollback-protected native pair saving, repeatable staged sound/configuration/SNDH saves with preserved permissions, asynchronous robust WAV export, measured arrangement durations and selectable help | Platform-specific device and clipboard conveniences |

Source extraction covers the verified Last Ninja and classic Best in Galaxy
player families. The current Mad Max audit decodes 48/357 supplied SNDH files;
44 produce an editable 6,000-frame excerpt with long-envelope volume commands;
the remaining four stay inspectable and support shorter valid selections.
Other layouts, unverified pitch mappings
and native capacity limits remain explicit. See
[YM reconstruction](YM_RECONSTRUCTION.md) for the measured scope.
`ympaircorpus` now trains from explicit SNDH/YM pairs while keeping instrument
identities independent of bank-local numbers. Whole-composition validation
reports unfamiliar-definition acceptance and independently detected YM onsets.
The initial five-composition result still lacks reliable rejection of unfamiliar
definitions; imported corpus labels therefore remain experimental candidates.
Native save/reload of three blind imports retains the preceding transcription
when no learned recipe improves its measured output.
Long ordinary-tone envelopes use native pattern volume without consuming an
effect column. Their bank definitions retain the other converted sound settings;
the report identifies definitions that need the generated score for playback.
Verified classic fixed-pitch mixer/noise programs use editable M/N commands,
preserving alternating phase, the noise shadow shared between source voices and
the verified classic eight-call noise sweep.

Native register evidence currently includes 8,778 complete replay calls across
the three supplied examples and isolated timer fixtures. The register checks
cover only captured writes, with timer-owned registers inspected separately.
See [replay verification](REPLAY_VERIFICATION.md).

Project packing protects YM/PCM interpretation across all 256 stored positions,
the current/saved native editor snapshots and independent edited, live or queued
pattern selections. Disabling PCM does not make its stored sample references
eligible for YM sequence remapping. Conflicting shared roles reject the operation
without changing the project; unchanged shared sequence IDs remain usable.
Regression checks include activating prepared PCM material after packing,
native song save/reload and editor undo. The three supplied native examples also
retain all 8,778 captured register calls after compaction and MYS/MYV save/reload;
their sequence counts change from 136 to 6, 136 to 12 and 13 to 13 respectively.

Packing also substitutes exact empty/note-off pattern presets, as described in
the native editor's Zap/Pack workflow. Track roles determine whether a stop can
use the YM or two-lane PCM preset; partial, shared and unassigned stop material
is retained. The original replay's PCM preset clears both lanes on the first
row, including muted and single-channel modes. Go replay now follows that rule,
and preset selections remain visible in the editor after packing. Native-pair
reload and 1,300-tick YM/PCM comparisons verify the substitutions without
changing stored musical playback.

The individual MYI3 export has been opened in the original editor. Its instrument
parameters, sequence remapping and embedded sample bytes were checked in memory.
See [native formats](NATIVE_FORMATS.md) for layout and legacy loader details.

WAV re-export streams to a staged file with bounded buffers, preserving the
previous complete audio until the replacement is rendered and synced. Existing
permissions are retained; failed reads/writes/commits leave the old output intact.
The UI renderer keeps composition editing and live playback independent.

The complete `go test -race ./...` suite has also passed with the macOS display
available, including the UI regressions for source-inspection recovery,
independent subtune editing, unused-slot selection and repeated background WAV
export. Window captures checked the pattern, arrangement, instrument, sequence,
sample, block-edit and settings panels, sequence generation, native file browsing,
converted/failed source previews and the complete collection export controls.
Sequence help text and long status messages were adjusted to preserve readable
clipboard and transport controls. These checks verify those editor workflows;
they do not extend the hardware or source-player coverage stated above.

The latest native-instrument allocation and direct sample-length changes have
passed their backend and UI regression checks with the race detector. The
complete suite has passed again with the macOS display available, including
reservation of saved PCM references during MYI import and undoable sample-length
editing. The current final backend check also passes `go vet`,
re-verifies all 8,778 example replay calls and retains the SNDH corpus totals:
690 editable imports, 880 subtunes and 666 unchanged single-song export/reload
checks. The previously reported corpus issues and hardware limits remain open
compatibility boundaries rather than evidence of complete Atari emulation.

Verified relative selectors also recover complete native collections whose empty
sample tag/guards follow their song data while retaining stale bank pointers.
The ten-song Tony Montezumas Gold collection adds 4,970 original and 4,973
regenerated native-call comparisons, alongside MYS/MYV save/reload checks.
SNDH duration export supports both TIME and FRMS arrays, retaining the outer
wrapper's call rate for collections and rejecting truncated or overflowing
metadata. Existing feature/hardware tags and executable boundaries are retained.

Native pattern-boundary parsing also recovers both PHF Rally 2 songs, including
their sequence alignment word and longer empty-sample suffix. The two source
projects save/reload without changes to their musical data; original and
regenerated collections each add 994 captured native-call comparisons.

The optional `-defaults` directory implements reusable native startup loading
for the GUI and headless command: CNF, SND/SNDH, a complete song/bank pair,
bank-only or individual MYI fallback. Explicit command-line files take precedence.
Native reload preferences are remembered independently of a song's settings and
remain active after later opens. Missing/corrupt candidates are reported, with
no partial song/bank replacement or source-file changes. Regression fixtures
check precedence, fallback, case-insensitive names, ambiguous names and editor
reload behavior; supplied native examples also load and render WAV through the
actual command, including explicit-file precedence.

Instrument copying retains a sounding destination's parameters and sequence
phase, then reloads its replaced bank definition on the next sequenced trigger.
Regression checks exercise consecutive score rows, unrelated sounding voices
and undo, so an unchanged instrument number cannot retain stale parameters.

Single-song binary replay wrappers now export through verified relative voice,
song-copy and rate operands, supporting eight additional corpus files. Both
direct and adjusted PC-relative offset tables are covered. Export retains the
outer call rate, executable prefix and entry points, with strict source bounds.
