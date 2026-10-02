# MaxYMiser Go feature status

This inventory relates the Go edition to the useful editing and playback
functions of MaxYMiser FM 1.67. It distinguishes editable format support from
replay validation and hardware-specific integration.

| Area | Implemented | Remaining verification or work |
| --- | --- | --- |
| Tracker | Three YM voices, two PCM voices, both effect columns, live/step note entry, masked block cut/copy/paste, paste modes, row insert/delete, expand/shrink, block/song remap, transpose, undo/redo | Additional editing shortcuts |
| Arrangement | Independent pattern lists, length/repeat, position selection, Jam markers, boundary-queued next-pattern controllers, duplicate pattern/sequence packing with reference remapping | More live resequencing scenarios |
| Instruments | 32 definitions, scalar/mask editing, linked sequence values, individual MYI exchange and copying | More native MYI legacy/sample fixtures |
| Sequences | 256 sequences, length/repeat, generation, signed values, copying and morphing | Range modification and additional shortcuts |
| Samples | Eight banks, signed PCM/WAV import, gain, interpolated tuning, trim, sign conversion, save and preview | Native YMise operation and full STe mixer/filter comparison |
| Native files | MYS/MYV lossless example round trips, MYI0–3 decode, MYI3 export, own SNDH import/export using a local replay template, ICE unpacking | ICE packing |
| YM | YM Player reference playback, register inspector, candidate score, composer corpus and cross-arrangement comparison | Musical-grid reconstruction, improved modulation/envelope hypotheses and reconstruction selection ranges |
| Replay | Sequences, commands, native frequency/DAC tables, timer waveforms, PCM note rates/modes | Broader full-song and mixed-timer evidence; sample-grid timer scheduling remains distinct from cycle-exact hardware |
| MIDI | macOS notes, program changes, controllers, transport, external clock/SPP, native channel mapping, duplicate-channel voice allocation and DMA input voices | Output ports and Sync24 hardware |
| Workflow | Resizable modern interface, drag/drop, file browser, native save, asynchronous WAV export with selectable duration | Richer contextual help |

Native register evidence currently includes 8,778 complete replay calls across
the three supplied examples and isolated timer fixtures. The register checks
cover only captured writes, with timer-owned registers inspected separately.
See [replay verification](REPLAY_VERIFICATION.md).

The individual MYI3 export has been opened in the original editor. Its instrument
parameters, sequence remapping and embedded sample bytes were checked in memory.
See [native formats](NATIVE_FORMATS.md) for layout and legacy loader details.
