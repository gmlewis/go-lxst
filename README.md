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

```bash
go get github.com/gmlewis/go-lxst
```

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

### Current Status

The `gornphone` CLI (`cmd/gornphone/`) provides the foundation for Reticulum
network telephony. The RNS signaling layer (link establishment, call state
machine, identity management) is functional. The audio I/O layer (oto backend,
codecs, filters, mixers) is functional. However, **the audio pipeline is not
yet wired to the RNS link layer** — meaning you cannot hear audio over a
remote call yet. The signaling works (calls can be established and torn down),
but no audio flows through the link.

### What Works Today

- **RNS identity management** — creates/loads persistent identities
- **RNS Destination/Link** — can establish encrypted links to remote peers
- **Call signaling** — ringing, connect, established, hangup state machine
- **Local audio I/O** — microphone capture and speaker playback via oto
- **Audio codecs** — Opus, Codec2, Raw PCM encoding/decoding
- **Audio filtering** — HighPass, LowPass, BandPass, AGC (parity-verified)
- **Cross-platform** — pure Go, works on macOS (M1/M2), Linux, Windows

### What's Not Yet Connected

The audio pipeline components exist (`lxst/network` Package: `Packetizer`,
`LinkSource`, `SignallingReceiver`) but are not wired into `gornphone`'s call
flow. Specifically:

1. When a call is established, `main.go` does not start the audio pipeline
2. Microphone audio is not encoded and sent over the RNS Link
3. Received audio from the Link is not decoded and played through the speaker

### Setting Up the Python Side (for when audio is wired)

The Go `gornphone` is wire-compatible with the Python `rnphone.py` from the
[LXST repository](https://github.com/markqvist/LXST). Both use the same
`lxst.telephony` primitive with matching RNS destination names and signalling
protocol.

**Step 1: Install Python LXST and RNS on the other machine**

```bash
# On the other person's machine (Linux or macOS):
pip install LXST
# RNS should be installed automatically as a dependency
```

**Step 2: Set up a shared Reticulum network**

Both machines need to be on the same Reticulum network. The simplest setup
is a direct TCP connection:

```bash
# On Machine A (the Go phone):
# Start gornphone — it will create a config directory at ~/.rnphone/
gornphone

# The first run creates ~/.rnphone/config and ~/.rnphone/identity
# Note the identity hash printed on startup — you'll share this
```

```bash
# On Machine B (the Python phone):
rnphone --config /path/to/rnphone-config

# The first run creates a default config
# Note the identity hash printed on startup
```

**Step 3: Configure RNS transport (TCP example)**

Create an RNS config file on each machine that tells RNS how to reach
the other. For a direct LAN connection, create
`~/.reticulum/config` (or `~/.config/reticulum/config`):

```ini
# Machine A config (~/.reticulum/config)
[[TCPServer]]
    tcp_address = 0.0.0.0
    tcp_port = 2222
```

```ini
# Machine B config (~/.reticulum/config)
[[TCPClient]]
    target_host = <Machine A's IP address>
    target_port = 2222
```

Both machines must also have a shared `trusted` entry for the other's
identity so they can form links. Alternatively, use announce + path
discovery.

**Step 4: Make a call**

Once both machines are running and have discovered each other's paths:

```bash
# On Machine A (Go phone), enter Machine B's identity hash:
> <Machine B's 32-char hex identity hash>

# On Machine B (Python phone), you'll see an incoming call
# Press Enter to answer
```

### Architecture When Audio Is Wired

```
┌──────────────┐                        ┌──────────────┐
│  Go gornphone│                        │ Python rnphone│
│              │  ◄─── RNS Link ───►   │              │
│  Mic → Enc ──┼───────────────────────┼──► Dec → Speaker
│  Speaker ◄───┼───────────────────────┼── Enc ← Mic │
│              │      (Opus/Codec2)    │              │
└──────────────┘                        └──────────────┘

Transmit: LineSource → Mixer → Codec → Packetizer → RNS Link
Receive:  RNS Link → LinkSource → Mixer → Codec → LineSink
```

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

### Bluetooth Audio on macOS

On macOS with Bluetooth earbuds, the oto backend uses CoreAudio which
automatically routes to the system's default output device. If your
Bluetooth earbuds are set as the default audio output in System Settings,
`gornphone` will use them. To select specific devices:

```bash
# List available devices
gornphone -l

# Use a specific speaker/mic
gornphone --speaker "AirPods Pro" --mic "AirPods Pro"
```

## License

Reticulum License — see [LICENSE](LICENSE) for details.