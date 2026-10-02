# Local SNDH corpus compatibility

The native codecs are also checked against a local collection containing 5,897
SNDH files. Of these, 704 contain MaxYMiser native instrument signatures.
The other files contain different replay routines and are not editable MaxYMiser
projects merely because they share the SNDH container format.

The current audit imports 663 files containing 839 editable native subtunes.
648 files use supported replay template layouts. All 648 export/reload checks
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

The remaining 56 reported issues include:

- MaxYMiser signatures embedded in nonstandard or repacked replay layouts.
- Archives with sample pointers beyond the physical file or truncated samples.
- Song positions referencing patterns that are absent from the stored payload.
- Importable subtunes whose outer replay wrapper is not a supported export template.

The importer does not invent missing musical data to make a truncated file pass.
Some missing pattern/sample references are unused at runtime, but distinguishing
that requires native execution evidence and a documented recovery policy.
