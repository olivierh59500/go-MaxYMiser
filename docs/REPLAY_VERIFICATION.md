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

The three supplied example pairs each match all 14 registers and the explicit
R13 envelope-write flag for these 199 calls: **597 complete calls, no mismatch**.
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

Every register and envelope write is checked in order; the first mismatch
reports its tick, register and both values. The project is cloned for the check.

This result covers the initial 3.98 seconds of each example at the replay-call
level. It does not prove full-song parity, timer phase accuracy, every tracker
effect or STe DMA resampling fidelity. Broader comparisons must extend the
native captures, include effect-specific fixtures and inspect timer writes
within the calls. PCM mixing/filter differences between synthesizers are a
separate question from the sequencer's register output.
