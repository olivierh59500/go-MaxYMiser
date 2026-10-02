# MaxYMiser Go feature status

This inventory relates the Go edition to the useful editing and playback
functions of MaxYMiser FM 1.67. It distinguishes editable format support from
replay validation and hardware-specific integration.

| Area | Implemented | Remaining verification or work |
| --- | --- | --- |
| Tracker | Three YM voices, two PCM voices, both effect columns, live/step recording, masked block editing, transpose/remap, row tools, octave/percussion shortcuts, undo/redo | Additional convenience shortcuts can be extended |
| Arrangement | Independent pattern lists, length/repeat, position selection, Jam markers, queued song jumps and track patterns, seamless song/pattern/record mode changes, native sequencer-only mutes and duplicate pattern/sequence packing with reference remapping | Broader hardware-controlled live performance verification |
| Instruments | 32 definitions, scalar/mask editing, linked sequence values, individual MYI exchange and copying | More native MYI legacy/sample fixtures |
| Sequences | 256 sequences, length/repeat, generation, signed values, range add/scale, copying and morphing | Additional shortcuts |
| Samples | Eight banks, signed PCM/WAV import, gain, interpolated tuning, trim, sign conversion, native YMise DAC quantization, STe DAC cadence/rate fixtures, save and preview | Analog mixer/filter response is approximated digitally |
| Native files | MYS/MYV lossless example round trips, MYI0–3 decode, MYI3 export, native CNF exchange/reload, own SNDH import/export using a local replay template, ICE packing/unpacking verified against Atari routines | More cross-version fixtures |
| YM | YM Player reference playback, register inspector, candidate score with selection ranges/explicit grids, onset-alignment proposals, composer corpus, cross-arrangement comparison, verified source-labelled SNDH/YM profiles, source envelope/arpeggio banks, measured instrument recipes and browsable source-pattern candidates | More source-player decoders, cross-song validation, arrangement recovery, continuous modulation and hardware-program reconstruction |
| Replay | Sequences, commands, native frequency/DAC tables, timer waveforms, PCM note rates/modes | Broader full-song and mixed-timer evidence; sample-grid timer scheduling remains distinct from cycle-exact hardware |
| MIDI | macOS notes, program changes, controllers, transport, external clock/SPP, native channel mapping, duplicate-channel voice allocation, DMA input/output voices and selected CoreMIDI output ports | Physical Sync24 hardware; tighter output timestamp scheduling |
| Workflow | Resizable modern interface, drag/drop, file browser, native save, asynchronous WAV export, selectable help for keyboard/effects/instruments/formats/MIDI | Platform-specific device and clipboard conveniences |

Native register evidence currently includes 8,778 complete replay calls across
the three supplied examples and isolated timer fixtures. The register checks
cover only captured writes, with timer-owned registers inspected separately.
See [replay verification](REPLAY_VERIFICATION.md).

The individual MYI3 export has been opened in the original editor. Its instrument
parameters, sequence remapping and embedded sample bytes were checked in memory.
See [native formats](NATIVE_FORMATS.md) for layout and legacy loader details.
