# go-lxst — Lightweight Extensible Signal Transport (Go)

A complete Go port of the [LXST](https://github.com/markqvist/LXST) real-time audio streaming library, built on top of [Reticulum](https://reticulum.network).

## Features

- **Pure Go** — no CGO required; builds with `go build ./...`
- **Cross-platform** — works on Linux, macOS, Windows, and Android
- **Audio codecs** — Opus, Codec2, Raw PCM, FLAC, MP3, Vorbis
- **Real-time filters** — HighPass, LowPass, BandPass, AGC (parity-verified against C reference)
- **Signal processing** — RMS, Peak, VAD, Normalize, Resample, Channel conversion
- **Audio I/O** — oto backend (pure Go); optional malgo backend via CGO
- **Pipeline architecture** — Source → Filter → Codec → Sink with staged routing
- **Audio mixing** — N-source mixer with per-source gain control
- **Tone generation** — Configurable frequency, gain, and easing
- **Embedded sounds** — Built-in ringer and soft tones
- **Hardware support** — Mock keypad/display interfaces (GPIO/I2C planned)

## Installation

### Library

```bash
go get github.com/gmlewis/go-lxst
```

### gornphone CLI

Install directly from GitHub — no clone needed:

```bash
go install github.com/gmlewis/go-lxst/cmd/gornphone@latest
```

This puts the `gornphone` binary on your `$GOPATH/bin` (or `$GOBIN`). Make
sure that directory is on your `PATH`.

## Quick Start

### Play a Tone

```go
package main

import (
    "time"
    "github.com/gmlewis/go-lxst/lxst/generators"
    "github.com/gmlewis/go-lxst/lxst/sinks"
)

func main() {
    sink := sinks.NewLineSink("", true, false)
    tone := generators.NewToneSource(440.0, 0.1, true, 20.0, 20.0, nil, sink, 1)
    tone.Start()
    time.Sleep(2 * time.Second)
    tone.Stop()
    sink.Stop()
}
```

### Apply Filters

```go
package main

import (
    "math"
    "github.com/gmlewis/go-lxst/lxst/filters"
)

func main() {
    hp := filters.NewHighPass(300)
    lp := filters.NewLowPass(3400)
    agc := filters.NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

    frame := make([][]float32, 480)
    for i := range frame {
        frame[i] = []float32{float32(math.Sin(2*math.Pi*440.0*float64(i)/48000))}
    }

    filtered := hp.HandleFrame(frame, 48000)
    filtered = lp.HandleFrame(filtered, 48000)
    filtered = agc.HandleFrame(filtered, 48000)
}
```

### Encode/Decode Audio

```go
package main

import (
    "github.com/gmlewis/go-lxst/lxst/codecs"
    raw "github.com/gmlewis/go-lxst/lxst/codecs/raw"
)

func main() {
    codec := raw.NewRaw(1, 16)
    frame := make([][]float32, 160)
    // ... fill frame ...
    encoded := codec.Encode(frame)
    decoded := codec.Decode(encoded, 1)
}
```

## Package Overview

| Package | Description |
|---------|-------------|
| `lxst/filters` | HighPass, LowPass, BandPass, AGC filters |
| `lxst/generators` | ToneSource signal generator |
| `lxst/mixer` | Multi-source audio mixer |
| `lxst/pipeline` | Source → Codec → Sink pipeline |
| `lxst/processing` | RMS, Peak, VAD, Normalize, Resample, etc. |
| `lxst/sources` | LineSource, OpusFileSource, Loopback |
| `lxst/sinks` | LineSink, OpusFileSink |
| `lxst/codecs` | Codec interface, Resample utilities |
| `lxst/codecs/opus` | Opus codec (CGO for encoding, pure-Go stub for metadata) |
| `lxst/codecs/raw` | Raw PCM codec |
| `lxst/codecs/codec2` | Codec2 codec (stub, CGO needed for full impl) |
| `lxst/codecs/flac` | FLAC file decoder (pure Go) |
| `lxst/codecs/mp3` | MP3 file decoder (pure Go) |
| `lxst/codecs/vorbis` | Vorbis file decoder (pure Go) |
| `lxst/platforms` | oto audio backend (malgo available via CGO) |
| `lxst/sounds` | Embedded audio resources (ringer, soft) |
| `lxst/call` | Telephony call endpoint management |
| `lxst/network` | Reticulum audio streaming |
| `lxst/primitives/hardware` | Keypad and display interfaces |
| `lxst/primitives/players` | File playback primitives |
| `lxst/primitives/recorders` | File recording primitives |
| `lxst/primitives/telephony` | DTMF, tones, dialing state machines |

## Testing

```bash
# Run all tests
go test ./...

# Run integration/parity tests
go test -tags=integration ./...

# Run benchmarks
go test -bench=. ./lxst/filters/... ./lxst/codecs/... ./lxst/mixer/... ./lxst/processing/...
```

## Parity with Python LXST

The Go implementation has been verified against the Python LXST C native filter
implementation using integration tests (`//go:build integration`). Key findings:

- **HighPass, LowPass, BandPass**: Full parity with C native implementation
- **AGC**: Full parity with C native implementation (block processing, peak limiting)
- **Python fallback** has a bug (line 102 double-applies alpha) — Go matches C, not Python fallback

## Architecture

```
Source → [Filter] → [Mixer] → Codec → Network → Codec → [Filter] → Sink
                              ↕
                          Loopback
```

Sources produce audio frames, filters process them, mixers combine multiple
sources, codecs encode/decode for transport, and sinks consume the final output.

## Making Phone Calls Over Reticulum

`gornphone` is a wire-compatible Go port of the Python `rnphone.py` from the
[LXST repository](https://github.com/markqvist/LXST). Both use the same
`lxst.telephony` RNS destination name and signalling protocol, so a Go
`gornphone` can call a Python `rnphone` and vice versa.

### Prerequisites

**On the Go side** — install `gornphone`:

```bash
go install github.com/gmlewis/go-lxst/cmd/gornphone@latest
```

**On the Python side** — install LXST (includes `rnphone`):

```bash
pip install LXST
```

Both sides need a [go-reticulum](https://github.com/gmlewis/go-reticulum) or
[Reticulum](https://github.com/markqvist/Reticulum) transport configured so
the two machines can reach each other.

### Step 1: Configure RNS Transport

Reticulum uses `~/.reticulum/config` to define how nodes reach each other.
**If your system already has a working RNS config** (for example, one that
connects to an RNS testnet or another RNS node), you can skip this step —
`gornphone` and `rnphone` will use whatever transport Reticulum already has.
Any two Reticulum nodes that can reach each other through the network can
make phone calls.

If you don't have an existing RNS config, the simplest setup for two machines
on the same LAN is a direct TCP connection.

On **Machine A** (the machine running `gornphone`), create
`~/.reticulum/config`:

```ini
[[TCPServer]]
    tcp_address = 0.0.0.0
    tcp_port = 2222
```

On **Machine B** (the machine running Python `rnphone`), create
`~/.reticulum/config`:

```ini
[[TCPClient]]
    target_host = <Machine A's LAN IP address>
    target_port = 2222
```

For other transport options (RNode LoRa, AutoInterface, etc.), see the
[Reticulum docs](https://reticulum.network/manual/interfaces.html).

### Step 2: Start `gornphone` (Go side)

```bash
gornphone
```

On first run, `gornphone` creates `~/.rnphone/config` and `~/.rnphone/identity`.
Note the **identity hash** printed on startup — this is your phone number that
you share with the other person.

### Step 3: Start `rnphone` (Python side)

```bash
rnphone
```

On first run, `rnphone` creates `~/.rnphone/config` and `~/.rnphone/identity`.
Note the identity hash printed on startup.

### Step 4: Make a Call

Once both machines are running and have discovered each other's paths
(either through announces or by entering the identity hash directly):

```bash
# On the Go phone, dial the Python phone's 32-char hex identity hash:
> <32-char identity hash>

# Or use the phonebook — add to ~/.rnphone/config:
# [phonebook]
#     Alice = <32-char identity hash>

# Then dial by name:
> alice
```

On the Python side, the incoming call will appear with a prompt to answer
(press Enter) or reject (press `r`).

### Interactive Commands

When `gornphone` is in the available state:

| Key | Command | Description |
|-----|---------|-------------|
| `<hash>` | — | Dial a 32-char hex identity hash |
| `<name>` | — | Dial a phonebook entry by name |
| `p` | phonebook | Show phonebook entries |
| `r` | redial | Redial the last called identity |
| `i` | identity | Show identity hash (share this with others to call you) |
| `d` | desthash | Show destination hash (for RNS path/announce) |
| `a` | announce | Send an announce on the network |
| `q` | quit | Exit gornphone |
| `h` | help | Show help |

When ringing (incoming call): press **Enter** to answer, any other key to
reject. When in a call: press **Enter** to hang up.

### Audio Architecture

```
┌──────────────┐                          ┌────────────────┐
│  Go gornphone│                          │ Python rnphone │
│              │                          │                │
│ Mic → Enc ───┼──────────────────────────┼──→ Dec → Spk   │
│ Spk ← Dec ───┼──────────────────────────┼──← Enc ← Mic   │
│              │      ◄── RNS Link ──►    │                │
└──────────────┘       (Opus/Codec2)      └────────────────┘

Transmit: LineSource → Mixer → Codec → Packetizer → RNS Link
Receive:  RNS Link → LinkSource → Mixer → Codec → LineSink
```

The `TelephoneEndpoint` in `cmd/gornphone/rns.go` wires the audio pipeline
automatically when a link is established — both for incoming calls (via
`incomingLinkEstablished`) and outgoing calls (via `Call`).

### Audio Profile Selection

`gornphone` supports the same audio profiles as Python `rnphone`:

| Profile | Code | Codec | Frame Time | Use Case |
|---------|------|-------|------------|----------|
| Ultra Low Bandwidth | 0x10 | Codec2 700C | 400ms | Weak links |
| Very Low Bandwidth | 0x20 | Codec2 1600 | 320ms | Narrow links |
| Low Bandwidth | 0x30 | Codec2 3200 | 200ms | Moderate links |
| Medium Quality | 0x40 | Opus | 60ms | Default |
| High Quality | 0x50 | Opus | 60ms | Good links |
| Super High Quality | 0x60 | Opus | 60ms | Excellent links |
| Low Latency | 0x70 | Opus | 20ms | Real-time |
| Ultra Low Latency | 0x80 | Opus | 10ms | Ultra real-time |

Select a profile at startup:

```bash
gornphone -profile 0x50
```

### Phonebook Configuration

Add entries to `~/.rnphone/config` to dial by name or numerical alias:

```ini
[phonebook]
    Alice = <32-char hex identity hash>
    Bob = <32-char hex identity hash>, 42
```

Then dial with `alice`, `bob`, or the alias `42`.

### Caller Access Control

Configure who can call you in `~/.rnphone/config`:

```ini
[telephone]
    # Allow everyone (default)
    allowed_callers = all

    # Block everyone
    allowed_callers = none

    # Only allow phonebook entries
    allowed_callers = phonebook

    # Only allow specific identity hashes
    allowed_callers = b8d80b1b7a9d3147880b366995422a45, fcfb80d4cd3aab7c8710541fb2317974

    # Block specific callers (overrides allow list)
    blocked_callers = f3e8c3359b39d36f3baff0a616a73d3e
```

### Bluetooth Audio on macOS

On macOS with Bluetooth earbuds, the oto backend uses CoreAudio which
automatically routes to the system's default audio device. To select
specific devices:

```bash
# List available devices
gornphone -l

# Use a specific speaker/mic
gornphone --speaker "AirPods Pro" --mic "AirPods Pro"
```

### Running as a Service

`gornphone` can run as a headless service that auto-answers incoming calls:

```bash
gornphone --service
```

To install as a systemd service on Linux:

```bash
gornphone --systemd    # prints a systemd unit file
```

## License

Reticulum License — see [LICENSE](LICENSE) for details.
