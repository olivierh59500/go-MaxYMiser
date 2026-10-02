# Native replay verification

Decoding a song correctly is not sufficient to establish faithful playback.
The Go sequencer is checked against actual YM register writes from the supplied
native binary replayer running under Hatari 2.6.1 in Atari ST mode.

A local 68000 harness initializes the binary with an example MYS/MYV pair and
calls its replay entry at 50 Hz. It retains the original timer interrupts, so
the native code executes its normal initialization and sound configuration.
The bounded harness stops after 200 calls. The final call cannot be isolated
from deinitialization in the capture; 199 complete calls are compared instead.
Boot, operating-system and deinitialization writes are excluded by the PC of
the main register-writing routine. Subtick timer writes are separate evidence.

The three supplied example pairs first matched all 14 registers and the explicit
R13 envelope-write flag for 199 calls each. Longer traces now cover **2,999 calls
for examples 1 and 2 and 2,780 calls for example 3**, including repeated patterns
and the saved Jam section. A missing Jam-state restore was found and corrected.
The original songs, binary, harness and traces remain local analysis data.

The verifier accepts a JSON array of register ticks:

```json
[
  {"Registers": [0,0,0,0,239,0,0,251,16,0,10,71,0,10], "EnvelopeWrite": true}
]
```

```sh
 go run ./cmd/replaycheck -song /path/to/example.mys \
   -bank /path/to/example.myv -trace /path/to/native-ticks.json
```

Every captured register and envelope write is checked in order; the first mismatch
reports its tick, register and both values. The project is cloned for the check.
Optional `Written` flags identify registers actually written by the main call.
SID/PWM/DigiDrum volume and FM frequency registers can belong to timer
interrupts and are deliberately not rewritten by the native main call. Comparing
their held values as though they were main-call targets would give false failures.
Fixtures without a `Written` mask continue to compare all fourteen registers.

These traces cover 59.98 seconds for examples 1 and 2 and 55.6 seconds for example 3
at the replay-call level. They do not prove full-song parity, timer phase accuracy, every tracker
effect or STe DMA resampling fidelity. Broader comparisons must extend the
native captures, include effect-specific fixtures and inspect timer writes
within the calls. PCM mixing/filter differences between synthesizers are a
separate question from the sequencer's register output.

## Isolated timer fixtures

Small original test projects were also passed through the supplied native binary.
MFP divider/data writes and timer-interrupt YM values provide independent expected
values for the Go tests:

| Effect | Native observations used by the checks |
| --- | --- |
| SID | Levels 15, 10, 0, 10; MFP divider /10, data 235 |
| PWM | Levels 15 and 3; intervals /50 × 129 and /16 × 177 |
| SyncBuzzer | Shapes A, E, 8 from the low byte; /50 × 94 cadence |
| SyncSquare / FM | Tone periods 478, 239, 319 for transposes 0, 12, 7 |
| SyncBuzzer FM | Envelope periods 30, 15, 20 and shapes A, E, 8 |

These checks found an incorrectly selected timer frequency table, missing PWM
sequence levels, and main-call writes that overwrote timer-owned registers.
The corrections are covered by regression tests. The audio renderer still
schedules writes on its sample grid; matching these discrete values and rates
does not claim cycle-exact 68000 interrupt timing.
