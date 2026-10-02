# Native format notes

All native multi-byte values are big-endian.

## Configuration

`MYM.CNF` contains 29 bytes with no header. Fields were mapped from the original
1.67 save/load routines: pattern scrolling, appearance settings, hardware flags,
composition year, five MIDI channel/instrument assignments and MIDI clock/latency
settings. The Go edition applies mapped tracker settings and retains other bytes
for native round trips. Unsupported display/hardware fields remain available
when exporting the file back to Atari.

## ICE wrappers

MYS, MYV, MYI and own SNDH imports accept Pack-Ice wrappers before validating the
native payload. The decoder reads backwards, checks literal/reference bounds,
limits output to 16 MiB, and supports the optional ST bitplane transform.
Compatibility fixtures include original synthetic data compressed by the native
editor. A supplied example also decompressed to its exact 2,194-byte source.
Native compression of a 3,800-byte bank, a 1,086-byte MYI3 instrument and a
41,938-byte SNDH was also decoded byte for byte and accepted by their respective
Go native importers. The parser rejects malformed lengths and offsets;
bounded fuzzing covers unexpected streams without panics.
The writer uses bounded LZ candidate searches and emits the same backwards
grammar. Go-packed MYS and SNDH files were decoded by the original routine with
byte-identical results. Saving supports either packed or unpacked files;
compression does not apply the optional graphics bitplane transform.

## Song

`MYM0TRAK` is followed by 64 bytes of tracker/editor state, 256 four-byte order
entries, one length byte and one repeat byte. The pattern payload starts at
file offset 1098. Each stored row consists of note, instrument, volume, effect
1, value 1, effect 2, value 2 and an RLE skip count. Stored rows plus skipped
empty rows form a 64-row pattern.

Patterns below `F0` are ordinary patterns. `FF` is empty, `FE` is note-off and
`FD` is the jam-loop marker. Notes use `00` for unchanged and `01` for note-off.
The displayed maximum volume, zero attenuation, is encoded as `10`; `00` means
unchanged.

## Voice bank

The first 32 bytes contain eight sample pointers relative to their own pointer
positions. `MYM1INST` follows, then 32 instruments of 64 bytes and eight
four-byte sample parameter records. Instrument bytes 0–15 contain the name;
16–21 are the detune masks, 32–41 hold scalar parameters and 48–55 link the
sequences.

Version 1 sequences use 126 bytes of word values and final length/repeat bytes.
Version 0 uses half that stride. `MYM1DIGI` precedes the sample payload. Unused
sample tails and instrument reserved bytes are preserved when saving.
INST and DIGI versions are independent; legacy 31-word sequence banks can contain
signed-byte DIGI1 samples. Both versions are preserved on save.

## Individual instruments

An individual instrument starts with `MYM0.MYI` through `MYM3.MYI`, followed by
the first 48 bytes of its native instrument definition. Sequence IDs are omitted;
the actual sequence records follow instead. Versions 0/1 carry seven 64-byte
records. Version 2 carries seven 128-byte records. Version 3 carries eight
128-byte records, including PWM. Its sample payload starts at offset 1080.

When the instrument uses a DigiDrum, remaining bytes are the signed sample in
versions 1–3. Version 0 uses four-bit DAC levels terminated by a negative byte;
the import uses the original editor's inverse DAC lookup. New exports use MYI3.
Import remaps definitions into unused sequence and sample slots and fails
without changing the bank when there is insufficient room.
The project-level importer additionally reserves sequence IDs called by either
effect column, PCM sample IDs from both lanes and reserved order rows, MIDI
sample assignments and live voice overrides. A referenced empty definition is
not free storage for an imported instrument.

Independent byte fixtures cover MYI0–3's full sequence stride, final signed word,
length/repeat bytes and version-specific sample boundary. MYI0's negative end
marker is separate from the 32 KiB decoded sample limit; a complete 32 KiB
payload followed by its marker is accepted, while a longer sample is rejected.

The supplied Atari editor was used to load a Go-exported MYI3: instrument
parameters, arpeggio/mixer/volume/PWM sequences and the sample's initial bytes
were verified in its memory. Its MYI3 loader retains a fixed 504-byte subtraction
for the sample length, despite the 1080-byte prefix, adding 576 bytes to the
reported sample length. The Go importer uses the actual version-specific prefix
and does not reproduce this out-of-bounds legacy behaviour.

## Embedded MaxYMiser exports

An unpacked MaxYMiser SNDH places its tracker payload between the voice-bank
sequence data and the sample tag. Extracting a native voice bank removes that
song block and adjusts the relative sample pointers by its length. The import
checks the native tags and decodes both resulting payloads before accepting the
project.
Optimized wrappers can place tracker data before or after the complete bank.
The importer locates actual sample boundaries through relative pointers and
validates candidate song/bank pairs. `DecodeContainers` exposes independently
editable subtunes; `DecodeContainer` selects the first accepted pair.
Copying selectors can arrange songs differently from physical payload order.
The importer recognizes a bounded 68000 instruction sequence and validates
every table target against a decoded bank before adopting the original order.
An unresolved, duplicate or out-of-file target leaves the generic decoded
ordering unchanged. Native payload counts and header-declared song counts
remain distinct, especially for collections mixing several replay formats.

### Native SNDH export

The exporter reuses an existing MaxYMiser replay prefix selected at runtime.
The imported prefix remains outside the source repository. Opening a supported
MaxYMiser SNDH retains it automatically; an independent MYS/MYV project can load
a replay through **Settings → Load SNDH replay**.

The first three branch entry points, executable data positions and voice-data
pointer are preserved. The song pointer is rebuilt, and sample-relative offsets
are adjusted for the newly inserted tracker block. Title, author, timer-C rate
and optional duration tags are updated within the header's original capacity.
An unsupported layout, incompatible bank version or oversized metadata is
rejected rather than silently producing a corrupt executable.

An original Go composition exported this way was reopened in the supplied Atari
editor: title/author, arrangement length and instrument names were verified in
memory. A separate native replay harness executed its entry points; all captured
register writes and envelope-write flags matched Go for 600 complete calls.

## Native multi-song export

`ParseMultiSNDHTemplate` recognizes the documented native wrapper that writes
voice offset, song offset and song byte length into the common binary replayer's
three pointer cells. Each selector slot must decode as a complete native bank
and song. The following song-rate address instruction must resolve to that
song's saved replay frequency. The header's declared count must equal the decoded
slot count; mixed players and partial collections are rejected.

`EncodeMultiSNDH` retains the runtime executable prefix and reconstructs every
voice bank and song. It patches all three selector operands and the song-rate
address, updates the complete TIME array, and preserves the three entry points.
Collection title/author/year come from the first project. Individual song rates
remain in their song data; the native wrapper's 200 Hz interpolation header is
preserved. The original executable is still an external local template.

The supplied two-song MULTSNDH example was re-exported with changed pattern and
sample sizes and rates 50/100 Hz. Both editable payloads round-trip unchanged.
Under Hatari in STe mode, executing the resulting original selector initializes
slot one with rebuilt offsets 12/23380 and length 1818, and slot two with offsets
25202/44750 and length 1882. The native rate cells contain 50 and 100 respectively,
and both deinitialization calls return normally. These checks prove selector
relocation and initialization for that wrapper; broader collection playback
parity still requires replay traces.

## YM recordings

The inspection decoder accepts YM2, YM3, YM3b, YM5 and YM6 register recordings,
including LZH wrappers through YM Player. YM playback itself uses YM Player's
full supported format set. Sampled tracker recordings such as YMT1/YMT2 do not
contain a three-channel YM register score and are excluded from chip-score
reconstruction.
