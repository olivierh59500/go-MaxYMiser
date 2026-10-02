# Native format notes

All native multi-byte values are big-endian.

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

## Embedded MaxYMiser exports

An unpacked MaxYMiser SNDH places its tracker payload between the voice-bank
sequence data and the sample tag. Extracting a native voice bank removes that
song block and adjusts the relative sample pointers by its length. The import
checks the native tags and decodes both resulting payloads before accepting the
project.

## YM recordings

The inspection decoder accepts YM2, YM3, YM3b, YM5 and YM6 register recordings,
including LZH wrappers through YM Player. YM playback itself uses YM Player's
full supported format set. Sampled tracker recordings such as YMT1/YMT2 do not
contain a three-channel YM register score and are excluded from chip-score
reconstruction.
