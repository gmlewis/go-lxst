// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package codecs provides audio codec interfaces and implementations for the
// LXST library. It defines the Codec interface that all codecs implement,
// along with shared utilities like Resample and ResampleBytes for sample
// rate conversion. Supported codecs include Opus, Codec2, Raw PCM, FLAC,
// MP3, and Vorbis.
package codecs

import (
	"encoding/binary"
	"errors"
	"math"
)

// CodecError represents a codec-related error.
var CodecError = errors.New("codec error")

// Codec defines the interface for audio codecs.
type Codec interface {
	Encode(frame [][]float32) []byte
	Decode(data []byte, channels int) [][]float32
	PreferredSampleRate() int
	FrameQuantumMs() float64
	FrameMaxMs() float64
	ValidFrameMs() []float64
	// Channels returns the number of audio channels for this codec's
	// current profile, matching the Python codec.channels attribute.
	// Returns 0 when the channel count is unknown or unset (e.g.
	// NullCodec), signalling the caller to infer channels from
	// other sources.
	Channels() int
}

// IsNullCodec reports whether the codec is a NullCodec or
// NullCodecBuffered — a passthrough codec that does not actually
// compress or transform audio data. In Python, the Null codec's
// encode is a no-op that returns the original frame unchanged.
func IsNullCodec(c Codec) bool {
	if c == nil {
		return false
	}
	switch c.(type) {
	case NullCodec, *NullCodec, *NullCodecBuffered:
		return true
	}
	return false
}

// NullCodec implements a passthrough codec for raw PCM.
type NullCodec struct{}

func (NullCodec) Encode(frame [][]float32) []byte {
	if len(frame) == 0 {
		return []byte{}
	}
	samples := len(frame)
	channels := len(frame[0])
	result := make([]byte, samples*channels*4)
	idx := 0
	for s := 0; s < samples; s++ {
		for c := 0; c < channels; c++ {
			binary.LittleEndian.PutUint32(result[idx:], math.Float32bits(frame[s][c]))
			idx += 4
		}
	}
	return result
}

func (NullCodec) Decode(data []byte, channels int) [][]float32 {
	if len(data) == 0 {
		return [][]float32{}
	}
	samples := len(data) / (channels * 4)
	result := make([][]float32, samples)
	for i := 0; i < samples; i++ {
		result[i] = make([]float32, channels)
		for c := 0; c < channels; c++ {
			idx := (i*channels + c) * 4
			if idx+3 < len(data) {
				result[i][c] = math.Float32frombits(binary.LittleEndian.Uint32(data[idx : idx+4]))
			}
		}
	}
	return result
}

func (NullCodec) PreferredSampleRate() int { return 0 }
func (NullCodec) FrameQuantumMs() float64  { return 0 }
func (NullCodec) FrameMaxMs() float64      { return 0 }
func (NullCodec) ValidFrameMs() []float64  { return nil }
func (NullCodec) Channels() int            { return 0 }

// NullCodecBuffered implements a passthrough codec for raw PCM with
// buffer reuse for reduced allocations in Decode.
type NullCodecBuffered struct {
	decBuf    []float32
	decFrames [][]float32
}

func (n *NullCodecBuffered) Encode(frame [][]float32) []byte {
	if len(frame) == 0 {
		return []byte{}
	}
	samples := len(frame)
	channels := len(frame[0])
	result := make([]byte, samples*channels*4)
	idx := 0
	for s := 0; s < samples; s++ {
		for c := 0; c < channels; c++ {
			binary.LittleEndian.PutUint32(result[idx:], math.Float32bits(frame[s][c]))
			idx += 4
		}
	}
	return result
}

func (n *NullCodecBuffered) Decode(data []byte, channels int) [][]float32 {
	if len(data) == 0 {
		return [][]float32{}
	}
	samples := len(data) / (channels * 4)
	totalFloats := samples * channels

	if cap(n.decBuf) < totalFloats {
		n.decBuf = make([]float32, totalFloats)
	} else {
		n.decBuf = n.decBuf[:totalFloats]
	}

	for i := 0; i < totalFloats; i++ {
		idx := i * 4
		if idx+3 < len(data) {
			n.decBuf[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[idx : idx+4]))
		}
	}

	if cap(n.decFrames) < samples {
		n.decFrames = make([][]float32, samples)
	}
	n.decFrames = n.decFrames[:samples]
	for s := 0; s < samples; s++ {
		n.decFrames[s] = n.decBuf[s*channels : (s+1)*channels]
	}

	return n.decFrames
}

func (n *NullCodecBuffered) PreferredSampleRate() int { return 0 }
func (n *NullCodecBuffered) FrameQuantumMs() float64  { return 0 }
func (n *NullCodecBuffered) FrameMaxMs() float64      { return 0 }
func (n *NullCodecBuffered) ValidFrameMs() []float64  { return nil }
func (n *NullCodecBuffered) Channels() int            { return 0 }

// Resample resamples float32 samples from inputRate to outputRate using
// linear interpolation. The input is [samples][channels] float32. Returns
// a new frame with the resampled sample count. When inputRate == outputRate
// the original frame is returned unchanged.
//
// Linear interpolation is sufficient for voice-quality audio and avoids
// the overhead of an external DSP dependency. For high-fidelity
// applications, a higher-quality resampler (e.g. gonum DSP) may be
// substituted.
func Resample(inputSamples [][]float32, bitdepth, channels, inputRate, outputRate int, normalize bool) [][]float32 {
	if inputRate == outputRate || len(inputSamples) == 0 || outputRate <= 0 {
		return inputSamples
	}

	inputLen := len(inputSamples)
	outputLen := int(math.Round(float64(inputLen) * float64(outputRate) / float64(inputRate)))
	if outputLen <= 0 {
		return [][]float32{}
	}

	ch := channels
	if ch == 0 && len(inputSamples[0]) > 0 {
		ch = len(inputSamples[0])
	}
	if ch == 0 {
		return [][]float32{}
	}

	result := make([][]float32, outputLen)
	ratio := float64(inputRate) / float64(outputRate)

	for i := 0; i < outputLen; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := float32(srcPos - float64(srcIdx))

		result[i] = make([]float32, ch)

		if srcIdx+1 < inputLen {
			for c := 0; c < ch && c < len(inputSamples[srcIdx]) && c < len(inputSamples[srcIdx+1]); c++ {
				result[i][c] = inputSamples[srcIdx][c] + frac*(inputSamples[srcIdx+1][c]-inputSamples[srcIdx][c])
			}
		} else if srcIdx < inputLen {
			for c := 0; c < ch && c < len(inputSamples[srcIdx]); c++ {
				result[i][c] = inputSamples[srcIdx][c]
			}
		}
	}

	return result
}

// ResampleBytes resamples raw PCM byte data from inputRate to outputRate.
// The input is interleaved int16 samples as bytes. Returns the resampled
// bytes. When inputRate == outputRate the original bytes are returned
// unchanged.
func ResampleBytes(sampleBytes []byte, bitdepth, channels, inputRate, outputRate int, normalize bool) []byte {
	if inputRate == outputRate || len(sampleBytes) == 0 || outputRate <= 0 {
		return sampleBytes
	}

	bytesPerSample := bitdepth / 8
	if bytesPerSample <= 0 {
		bytesPerSample = 2 // default to int16
	}

	totalSamples := len(sampleBytes) / (bytesPerSample * channels)
	if totalSamples == 0 {
		return sampleBytes
	}

	// Convert bytes to float32 frame [samples][channels].
	frame := make([][]float32, totalSamples)
	for i := 0; i < totalSamples; i++ {
		frame[i] = make([]float32, channels)
		for c := 0; c < channels; c++ {
			idx := (i*channels + c) * bytesPerSample
			if idx+bytesPerSample <= len(sampleBytes) {
				frame[i][c] = math.Float32frombits(binary.LittleEndian.Uint32(sampleBytes[idx : idx+4]))
			}
		}
	}

	resampled := Resample(frame, bitdepth, channels, inputRate, outputRate, normalize)

	// Convert back to bytes.
	outLen := len(resampled) * channels * bytesPerSample
	result := make([]byte, outLen)
	idx := 0
	for i := range resampled {
		for c := 0; c < channels && c < len(resampled[i]); c++ {
			binary.LittleEndian.PutUint32(result[idx:], math.Float32bits(resampled[i][c]))
			idx += bytesPerSample
		}
	}

	return result
}
