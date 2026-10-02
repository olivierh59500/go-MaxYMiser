# Local SNDH corpus compatibility

The native codecs are also checked against a local collection containing 5,897
SNDH files. Of these, 704 contain MaxYMiser native instrument signatures.
The other files contain different replay routines and are not editable MaxYMiser
projects merely because they share the SNDH container format.

The current audit imports 687 files containing 863 editable native subtunes.
667 files use supported replay template layouts. All 667 export/reload checks
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

The remaining 37 reported issues include:

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
