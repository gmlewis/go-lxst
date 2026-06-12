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

## License

Reticulum License — see [LICENSE](LICENSE) for details.