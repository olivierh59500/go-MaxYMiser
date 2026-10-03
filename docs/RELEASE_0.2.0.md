# Go MaxYMiser 0.2.0

Development release adding executable SNDH playback and editable excerpts to
Go MaxYMiser. Native MaxYMiser payload extraction and verified source inspection
remain the preferred import paths when available.

## Added since 0.1.0

- Built-in Go 68000 execution, MFP timers, YM2149 synthesis through YM Player,
  and STE DMA/Microwire audio. No external emulator is needed at runtime.
- Plain and ICE-packed SNDH headers, default song selection, declared durations
  and selectable executable subtunes, including foreign slots in mixed-player
  containers.
- Background import with adjustable excerpt ranges. Register transcription
  proposes editable notes and sounds; digital-only or over-capacity cases use
  explicitly labeled editable PCM excerpts instead of empty scores.
- Independent executable-song edits, save destinations, cursors and undo/redo.
  New opening requests cancel older analysis; failures retain the current song.
- Original SNDH/score comparison, live registers, cancellable asynchronous seeking
  and original-player WAV export separate from generated-score rendering.
- Headless `sndhimport` and `sndhcheck` commands, usage documentation and retained
  MIT notices for the Go CPU core and hardware implementation references.
- Clearer native mixed-player/repeated-choice collection limits and a guided
  tracker presentation showing import, editing, replay and export.

## Validation

- The complete race-enabled Go test suite and `go vet` pass.
- Local corpus: all **5,897 headers** and **11,758 declared songs** initialize;
  five-second execution succeeds for **5,895 files / 11,750 songs**.
- A separate 400-frame/default-song import audit creates **5,895 nonempty editable
  candidates**: **5,362 inferred register scores** and **533 sampled excerpts**.
  All save/reload as MYS/MYV with byte-identical native payloads.
- Synthetic sampled-score audio correlates with its original DAC waveform above
  0.98 after startup transients. Sample edits change the resulting audio.
- Regression checks cover finite execution, PCM chunk timing/capacity, failed and
  cancelled imports, independent song workspaces and seek replacement.
- macOS interface captures check foreign Mad Max import and Quartet sample
  editing. A thirty-second original Mad Max WAV renders successfully.

## Compatibility boundaries

These executable-import counts measure bounded excerpts, not full-duration
playback or recovery of every original instrument bank. Original names, patterns
and source programs are not generally recoverable from register snapshots.
Sampled excerpts retain the rendered mix as normalized mono eight-bit PCM at
25,033 Hz; eight native sample slots hold approximately ten seconds. Longer
selections that exceed this capacity are shortened explicitly.

Kelly's **Last Ninja 2** and Zerkman's **Ah Que Coucou** still fail during replay;
the pinned reference reader also produces silence for these two wrappers.
Exit errors remain distinct from replay failures. Hardware timing, analog
filters, mixed timer fidelity and stereo STE spatial imaging are not claimed
as exact Atari emulation.

Source reconstruction remains experimental. Native SNDH export requires a
compatible MaxYMiser replay template supplied separately; foreign executable
imports do not become native export templates.

Go 1.26 or newer is required. See [Getting started](GETTING_STARTED.md),
[SNDH import and measured coverage](SNDH_IMPORT.md) and
[Feature status](FEATURES.md).
