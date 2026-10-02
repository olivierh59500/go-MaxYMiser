# MaxYMiser Go feature status

This inventory relates the Go edition to the useful editing and playback
functions of MaxYMiser FM 1.67. It distinguishes editable format support from
replay validation and hardware-specific integration.

| Area | Implemented | Remaining verification or work |
| --- | --- | --- |
| Tracker | Three YM voices, two PCM voices, both effect columns, song/pattern recording, native Scroll control, song-start/pattern-cursor playback, safe unused pattern/sequence selection, masked block editing, transpose/remap, row tools, octave/percussion shortcuts and undo/redo | Further platform-specific keyboard conveniences |
| Arrangement | Independent pattern lists, position insertion/deletion and clipboard ranges, occurrence cloning, length/repeat, Jam jumps, seamless song/pattern/record changes, sequencer-only mutes and validated pattern/sequence packing | Broader hardware-controlled live performance verification |
| Instruments | 32 definitions, live scalar/mask editing, serialized sequence links, MYI exchange and copying | More native MYI legacy/sample fixtures |
| Sequences | 256 sequences, live length/repeat/word edits, phase-preserving shared refresh, native clipboard shortcuts, generation, signed values, range modification and morphing | Further native timer phase comparisons |
| Samples | Eight banks, signed PCM/WAV import, gain, interpolated tuning, trim, sign conversion, native YMise DAC quantization, STe DAC cadence/rate fixtures, save and preview | Analog mixer/filter response is approximated digitally |
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

The individual MYI3 export has been opened in the original editor. Its instrument
parameters, sequence remapping and embedded sample bytes were checked in memory.
See [native formats](NATIVE_FORMATS.md) for layout and legacy loader details.

WAV re-export streams to a staged file with bounded buffers, preserving the
previous complete audio until the replacement is rendered and synced. Existing
permissions are retained; failed reads/writes/commits leave the old output intact.
The UI renderer keeps composition editing and live playback independent.
