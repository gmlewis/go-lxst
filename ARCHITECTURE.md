# go-lxst Architecture

## Overview

go-lxst is a Go port of the Python LXST library for real-time audio processing
and telephony over Reticulum networks. It provides codecs, filters, generators,
sources, sinks, and a pipeline architecture for connecting audio components.

## Package Structure

```
lxst/
├── call/              # Telephony call endpoint management
├── codecs/            # Codec interface and shared utilities
│   ├── codec2/        # Codec2 codec (stub, requires CGO for full impl)
│   ├── opus/          # Opus codec (CGO for encode, pure-Go stub for decode)
│   ├── raw/           # Raw PCM codec with configurable bit depth
│   ├── flac/          # FLAC file decoder (pure-Go via mewkiz/flac)
│   ├── mp3/           # MP3 file decoder (pure-Go via hajimehoshi/go-mp3)
│   └── vorbis/        # Vorbis file decoder (pure-Go via jfreymuth/oggvorbis)
├── filters/           # HighPass, LowPass, BandPass, AGC (pure-Go, 0 allocs/op)
├── generators/        # ToneSource signal generator with easing
├── mixer/              # N-source audio mixer with per-source gain
├── network/            # Reticulum audio streaming (LinkSource, Packetizer)
├── pipeline/           # Source → Codec → Sink pipeline
├── platforms/          # Audio I/O backends (oto, malgo)
├── processing/         # RMS, Peak, VAD, Normalize, Resample, etc.
├── primitives/
│   ├── hardware/       # Keypad and display interfaces (mock implementations)
│   ├── players/        # FilePlayer for audio playback
│   ├── recorders/      # FileRecorder for audio recording
│   └── telephony/       # DTMF, tones, call profiles, Telephone primitive
├── sinks/              # LineSink, OpusFileSink
├── sources/            # LineSource, OpusFileSource, Loopback
└── sounds/             # Embedded sound resources (ringer.opus, soft.opus)
```

## Data Flow

### Audio Pipeline

```
Source → [Filter] → [Mixer] → [Codec] → Network → [Codec] → [Filter] → Sink
                         ↕
                     Loopback
```

Sources produce `[][]float32` frames (samples × channels). Filters process
frames in-place or return new frames. Codecs encode frames to bytes and
decode bytes back to frames. Sinks consume frames for playback or writing.

### Frame Format

All audio data flows as `[][]float32` where:
- First dimension: sample index (e.g., 480 samples at 48kHz = 10ms)
- Second dimension: channel index (1=mono, 2=stereo)
- Values: -1.0 to 1.0 (float32)

### Codec Interface

```go
type Codec interface {
    Encode(frame [][]float32) []byte
    Decode(data []byte, channels int) [][]float32
    PreferredSampleRate() int
    FrameQuantumMs() float64
    FrameMaxMs() float64
    ValidFrameMs() []float64
}
```

Codecs constrain frame timing. When a source or mixer has a codec set,
the codec's `FrameQuantumMs`, `FrameMaxMs`, and `ValidFrameMs` are used
to quantize the target frame duration.

## Filter Design

Filters match the **C native implementation** in `Filters.c`, not the Python
fallback in `Filters.py`. The Python fallback has a bug (line 102 applies
`alpha * (output + input_diff)` a second time after the loop).

All filters (HighPass, LowPass, BandPass, AGC) use **output buffer reuse**
to achieve 0 allocations per frame in steady-state operation.

### AGC Block Processing

The AGC processes audio in blocks where:
- `numBlocks = max(1, int(samples/samplerate/blockTargetS))`
- `blockSize = samples / numBlocks`
- Within each block: compute RMS → compute target gain → smooth gain → apply
- After all blocks: peak limiting at 0.75

This matches the C implementation exactly.

## Memory Optimization

Hot-path components are optimized for real-time audio:

| Component | Allocs/Frame | Technique |
|-----------|-------------|-----------|
| HighPass  | 0           | Output buffer reuse |
| LowPass   | 0           | Output buffer reuse |
| BandPass  | 0           | Cascades HP+LP (0 each) |
| AGC       | 0           | Output buffer reuse |
| RawCodec.Decode | 0   | Flat buffer reuse |
| NullCodecBuffered.Decode | 0 | Flat buffer reuse |
| Mixer     | 0           | Pre-allocated frame combining |
| Processing (RMS, Peak, VAD, etc.) | 0 | Read-only operations |

For performance-critical pipelines, use `NullCodecBuffered` instead of
`NullCodec` to avoid per-frame allocations in Decode.

## Testing

```bash
# Standard unit tests
go test ./...

# Integration/parity tests (compares against Python/C reference values)
go test -tags=integration ./...

# Benchmarks
go test -bench=. ./lxst/filters/... ./lxst/codecs/... ./lxst/mixer/... ./lxst/processing/...
```

## Build Constraints

The default build is pure Go. CGO backends are available as opt-in:

- **`CGO_ENABLED=1`** (opt-in): Enables malgo audio backend and libopus encoding
- **Default build**: Uses oto (pure Go) for audio I/O, pure-Go file decoders (FLAC/MP3/Vorbis)
- Codec2 and Opus encoding require CGO; Opus decoding for file sources is handled by pure-Go libraries