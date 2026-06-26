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

// Uses pydub internally in Python; Go port uses gonum or simple linear interpolation.
func ResampleBytes(sampleBytes []byte, bitdepth, channels, inputRate, outputRate int, normalize bool) []byte {
	// Simple pass-through for now - full implementation needs gonum DSP
	if inputRate == outputRate {
		return sampleBytes
	}
	// TODO: Implement proper resampling using gonum
	return sampleBytes
}

// Resample resamples float32 samples from input_rate to output_rate.
func Resample(inputSamples [][]float32, bitdepth, channels, inputRate, outputRate int, normalize bool) [][]float32 {
	if inputRate == outputRate {
		return inputSamples
	}
	// TODO: Implement proper resampling using gonum
	return inputSamples
}
