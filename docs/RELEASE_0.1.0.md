# Go MaxYMiser 0.1.0

First tagged development release of the Go/Ebitengine tracker, inspired by
MaxYMiser FM 1.67. The native editing workflow is usable; automatic YM/source
reconstruction remains experimental and has explicit compatibility limits.

## Included

- Three YM voices, two STe PCM lanes, 32 instruments and 256 editable sequences.
- Native pattern/arrangement editing, recording, Jam, shared sound editing,
  clipboard tools, packing and undo/redo.
- MYS/MYV, MYI and CNF exchange; verified native SNDH imports and replay-template
  exports, complete relative-selector collections and ICE compression.
- YM reference playback, register inspection, editable pitch curves, recurring
  melody candidates and source/corpus evidence with explicit provenance.
- Verified classic Mad Max song selection/global transpose and supported Last
  Ninja source extraction, with separate inspection and excerpt import.
- macOS MIDI integration, robust background WAV rendering and a resizable UI.

## Validation

- The complete race-enabled Go suite and `go vet` pass, including the interface
  create/edit/save/reopen/non-silent-WAV workflow.
- Native corpus: 690 editable containers, 880 subtunes and 666 unchanged
  single-song export/reload checks across 704 native candidates.
- Three additional complete arrangements retain 64,562 captured main-call
  comparisons, repeated for their regenerated exports. Existing 8,778 example
  replay calls remain unchanged after the sound corrections.
- Two real YM excerpts retain 6,674 active tone-period comparisons after native
  pair save/reload. Source/global-transpose fixtures use original native execution
  separately from these YM comparisons.
- The instrument, source-subtune and recurring-phrase panels were visually
  checked on macOS. CoreMIDI remains the verified MIDI platform.

## Compatibility boundaries

The native corpus still reports 14 import failures, 24 unsupported single-song
templates and three declared-count discrepancies. Missing waveform bytes are
not invented, and containers mixing foreign players do not become fully editable
MaxYMiser projects. Source decoding covers 48/357 supplied Mad Max files; most
other player layouts remain unsupported.

Exact tone curves preserve register pitch, not all original hardware programs.
Corpus labels remain similarity candidates. On the five-composition evaluation,
requiring two supporting training compositions loses all measured known-label
coverage; that stricter threshold is optional. Original arrangement recovery,
cycle-exact timer scheduling, analog STe filtering and physical Sync24 remain
outside this release's guarantees.

Builds require Go 1.26 or newer. Start with [Getting started](GETTING_STARTED.md),
then consult [Feature status](FEATURES.md), [Native formats](NATIVE_FORMATS.md)
and [Replay verification](REPLAY_VERIFICATION.md).
