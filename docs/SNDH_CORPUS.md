# Local SNDH corpus compatibility

The native codecs are also checked against a local collection containing 5,897
SNDH files. Of these, 704 contain MaxYMiser native instrument signatures.
The other files contain different replay routines and are not editable MaxYMiser
projects merely because they share the SNDH container format.

The current audit imports 690 files containing 880 editable native subtunes.
658 files use supported single-song replay templates. All 658 export/reload checks
preserve serialized song data, voice banks and title/author metadata exactly.
This verifies editable data handling, not audio fidelity across those songs.

```sh
 go run ./cmd/sndhaudit -directory /path/to/sndh-collection -output report.json
```

The JSON separates import failures, unsupported replay templates and changed
round trips. Original recordings and executable references are not included in
the repository.

The collection exposed independent INST/DIGI versions, alternate bank/song/sample
ordering, multiple native subtunes and metadata words that resemble SNDH tags.
Regression fixtures cover these cases without copying the source compositions.

The current report contains 49 issues: 14 import failures, 32 unsupported
single-song export templates, and three declared-count discrepancies. These
categories can refer to the same container. Issues include:

- MaxYMiser signatures embedded in nonstandard or repacked replay layouts.
- Archives with sample pointers beyond the physical file or truncated samples.
- Song positions referencing patterns that are absent from the stored payload.
- Importable subtunes whose outer replay wrapper is not a supported export template.

The importer does not invent missing musical data to make a truncated file pass.
Disabled PCM tracks can retain unstored pattern IDs: those are preserved but
not parsed during replay, matching the native mode. Known binary wrappers supply
exact song lengths; their footer bytes are not interpreted as new patterns.
Some missing pattern/sample references are unused at runtime, but distinguishing
that requires native execution evidence and a documented recovery policy.

## Optimized empty sample banks

Thirteen additional containers have valid instrument/sequence/song data, but
their optimizers removed trailing empty sample guards without updating every
pointer. All eight sample lengths are explicitly zero. The native initialization
tests their total length and skips sample conversion when it is zero.

For these containers only, out-of-file trailing pointers are clamped to the
physical end of the empty bank. Header pointers, the first sample/tag boundary
and nonempty sample declarations remain strict. No waveform bytes are created.
Standalone MYV decoding also remains strict; saving the recovered project writes
normal native empty-sample pointers and guards.

**Word** by Excellence in Art was compared against 288 original replay calls
under Hatari: 4,032 register comparisons and envelope-write flags matched after
initializing the Go comparison with the native held-register state. This
initial state accounts for unused tone registers retained from the Atari
environment. Other recovered files are covered by decode/export/reload checks;
this one playback comparison is not evidence for complete audio parity of all
thirteen songs.

## Displaced empty sample blocks in native collections

**Tony Montezumas Gold** contains ten separate native banks and songs. Its
relative selector supplies exact voice starts, song starts, copied lengths and
song-rate pointers. Each bank retains complete instruments/sequences and declares
eight zero-length samples, but its sample pointers retain their old positions.
The actual DIGI tag, eight empty guards and two opaque suffix bytes follow the
song patterns within the selector's copied span.

The importer follows this verified selector, validates the complete pattern
stream before the trailing block, and rebuilds pointers to the existing empty
guards. It preserves the actual INST/DIGI versions and trailing bytes. Recovery
requires zero sample lengths, complete sequence records and a sample tag at a
complete pattern boundary within the selector's copied span. Missing nonempty
waveforms and invalid lengths remain errors.
Standalone MYV decoding remains strict.

All ten banks and songs save/reload as native MYS/MYV without data changes.
Each original subtune also matches 497 captured complete native replay calls,
for a total of 4,970 calls. Regenerated collection slots match another 4,973
captured native calls. Comparisons cover captured main-register writes and
envelope flags; they do not establish complete analog or timer-waveform parity.
The trace boundaries account for Start Sync's repeated tone writes rather than
discarding those calls from comparison.

This collection uses FRMS metadata instead of TIME. Export updates its complete
frame-duration array using the outer TC200 call rate. The individual songs'
internal rates remain independent. A complete multi-song collection still cannot
be exported through a single-song template, so this newly importable collection
moves from the import-failure category to the single-template rejection category.

The same rule now supports an empty trailing suffix after physically present
waveforms. Each normalized slot must declare zero length, all later slots must
also be empty, and the unchanged strict decoder must still validate every
nonempty sample. Hatari 2.1 by Dma-Sc and Vrien by Frequent import through this
layout. Missing or truncated nonempty waveforms remain rejected.

## Subtunes and export templates

Several native songs can share one voice bank. The importer retains all their
separate editable arrangements rather than stopping after the first song.
Yoomp exposes five songs through this layout.

The audit compares decoded payload count with the bounded `##nn` header tag.
Terraboink declares 20 songs with one decoded payload; Nano Cave declares seven
with six; Randomazer declares nine with eight.
Selector aliases, cue positions or unsupported payloads require further native
analysis. These collections are imported partially and recorded as such.

Nano Cave's initializer has an explicit native payload table for songs 1–6.
These table slots use a different order from the physical bank layout. The
importer now follows the validated relative table and exposes those six songs
with their original numbering. Song 7 branches to a separate replay routine
whose music is not a MaxYMiser payload, so it remains outside editable import.
PHF Rally 2 now exposes both native songs in their selector order. Its first bank
has a two-byte alignment word after complete sequence records; both song spans
include a displaced empty DIGI block and a longer opaque suffix. The sample tag
is located only after complete 64-row patterns, never by truncating at an
arbitrary byte match. The suffix remains in the saved sample trailer; normal MYV
output omits the source alignment word. Both songs retain 994 original and 994
regenerated native replay calls in total, alongside unchanged MYS/MYV save/reload.
These checks cover captured main-register writes and envelope flags rather than
complete timer or analog parity.

A multi-song executable prefix cannot safely export one replacement song while
its selector retains references to discarded songs. Single-song template
validation now rejects both declared and physically detected multi-song
containers. This removed eleven previously counted data-only round trips from
the safe export-template total: successful data reload did not prove that those
generated executables could still select every song. Their editable imports
remain available, and selected songs can be saved as MYS/MYV or exported using
a verified single-song replay template.
