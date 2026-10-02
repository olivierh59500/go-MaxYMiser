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
The verifier excludes main-call envelope flags only when a native write mask
identifies a SyncBuzzer-owned R13. Timer envelope writes are checked separately.

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

## Combined effect songs

Longer captures of supplied tunes compare 1,000 main calls of PWMWVJAM, 980 calls
of SEQUENCE in STe mode, and 994 calls of FMERGENCY. All captured main-call
register writes match. FMERGENCY's start-sync path uses additional tone writes;
its call boundaries are identified by the main write block rather than one
fixed R0 instruction. SyncBuzzer R13 ownership is separated from main calls.
This extends sequencer evidence while retaining the sample-grid timer limitation.

## PCM and Microwire

Synthetic signed PCM fixtures were exported by Go and executed by the native
SNDH replayer under Hatari in STe mode. Audio captures contain one sine sample,
two mixed sine samples, and native-rate playback. Expected components near
259 Hz / 176 Hz and 782 Hz are present in both renderers. Amplitude and analog
filter response are not asserted to be identical between Hatari and YM Player.

These captures and replay source exposed STe sample-and-hold at 25,033 Hz in
resampling modes and the skipped initial sample byte in native mode. Regression
tests cover DAC cadence, note rates, shifts, voice allocation and note-off.

Microwire master/pan controls now use the native 2 dB attenuation increments
independently of editor volume. Bass/treble control the Go digital shelving
filters using 2 dB steps; default flat settings bypass processing. Frequency
response tests verify low/high boosts and cuts. This preserves the intended
controls but is not a transistor-level emulation of the LMC1992 analog circuit.

## External clock and latency

The 1.67 editor's original MIDI dispatch tests the external-clock and Sync24
selection bits before accepting F8, Start, Continue, Stop or Song Position
Pointer. Internal timer calls skip the playing replayer with external clock;
each accepted pulse calls the complete routine. Instrument sequences and
effects therefore follow the input pulses too. Stopped instrument preview
continues to use the internal timer.

Selecting a clock in the native interface sets row speed to six. Pattern `S`
commands are ignored in external mode. Controller 21 still writes its scaled
speed directly, so live MIDI control can override that default spacing.

Start and Continue execute the normal clock routine once for each unsigned
latency value. The Go audio pass consumes these queued calls together, retaining
PCM and envelope triggers until the synthesizer receives them. Values retain
their native units: pulses, rather than milliseconds or rows.

Bounded calls to the original editor under Hatari in STe mode recorded:

| Native operation | Latency | Row after the call | Pulses consumed in that row |
| --- | --- | --- | --- |
| Start | 7 | 1 | 1 |
| Stop | 7 | 1 | 1 |
| Continue from that stop | 7 | 2 | 2 |
| Start from the beginning | 255 | 42 | 3 |

Regression checks use these retained native positions and cover pulse-driven
volume envelopes, ignored internal callbacks, clock-source filtering, order
boundaries and a compensated PCM trigger reaching the audio renderer once.
These checks establish transport/cadence behavior; they do not measure physical
MIDI-device latency or establish timestamp-accurate delivery of batched input.

## MIDI note output and relay

Native debugger captures also inspect the output byte at the original editor's
ACIA transmit instruction. They confirm the attenuation table, seven-bit
transposition wrap, zero-velocity release, and ordering controlled by the current
sample number. For channel 4, a C4 with transpose +12 and attenuation 2+1 emits
note 72 with velocity 79. A subsequent sample-zero E4 emits note 76 before
releasing 72; a nonzero sample then releases 76 before emitting 79. Attenuation
eight emits zero velocity. The native routine sends no program change for a
sample number.

The Go checks use these captured values and ordering, including a transposed
MIDI note zero. The Go output omits native cleanup messages for a previous note
that was not active; genuine note zero is tracked independently and released.
Changing an output channel releases the previous note on its original channel.

The renderer handles each queued external pulse separately, emitting its clock
and notes before moving to the following call. A seven-pulse batch preserves
both its initial note and the following row's note. A dense two-voice fixture
with the maximum 255 compensation calls produces 1,274 queued events without
overflow. The fixed queue holds 2,048 events and reports excess events; the audio
path allocates no memory during this external-clock check.

An end-to-end local CoreMIDI test delivered 24 generated tracker messages to a
temporary virtual destination unchanged: 14 relayed pulses, three note onsets,
Stop and Continue, and the corresponding releases. This verifies the native-port
delivery path without a physical synthesizer; hardware latency and timestamp
scheduling remain separate measurements.
