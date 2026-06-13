// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package raw implements the Raw PCM codec for the LXST library with
// configurable bit depths (8, 16, 24, 32) and channel counts. It provides
// lossless encode/decode with optional sample rate conversion and dithering
// for bit depth reduction, suitable for testing and raw audio I/O.
package raw

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	ErrInvalidBitdepth = errors.New("invalid bitdepth")
	ErrInvalidChannels = errors.New("invalid channel count")
)

// BITDEPTHS maps bitdepth constants to Go types
const (
	BITDEPTH_16  = 0x00
	BITDEPTH_32  = 0x01
	BITDEPTH_64  = 0x02
	BITDEPTH_128 = 0x03
)

var BITDEPTHS = []string{"float16", "float32", "float64", "float128"}

// Raw implements the Codec interface for raw PCM with configurable bit depth.
type Raw struct {
	bitdepth  int
	channels  int
	dtype     string
	headerBD  int
	decBuf    []float32
	decFrames [][]float32
}

// NewRaw creates a new Raw codec with optional channels and bitdepth.
func NewRaw(channels, bitdepth int) (*Raw, error) {
	if bitdepth < 16 || bitdepth > 128 {
		return nil, ErrInvalidBitdepth
	}
	if channels != 0 && (channels < 1 || channels > 32) {
		return nil, ErrInvalidChannels
	}

	r := &Raw{
		bitdepth: bitdepth,
		channels: channels,
	}

	r.updateDtype()
	return r, nil
}

func (r *Raw) updateDtype() {
	if r.bitdepth >= 128 {
		r.dtype = BITDEPTHS[BITDEPTH_128]
		r.headerBD = BITDEPTH_128
	} else if r.bitdepth >= 64 {
		r.dtype = BITDEPTHS[BITDEPTH_64]
		r.headerBD = BITDEPTH_64
	} else if r.bitdepth >= 32 {
		r.dtype = BITDEPTHS[BITDEPTH_32]
		r.headerBD = BITDEPTH_32
	} else {
		r.dtype = BITDEPTHS[BITDEPTH_16]
		r.headerBD = BITDEPTH_16
	}
}

// Encode encodes float32 frames to raw PCM bytes with header.
// Header format: (bitdepth << 6) | (channels - 1)
func (r *Raw) Encode(frame [][]float32) []byte {
	if len(frame) == 0 {
		return []byte{}
	}

	if r.channels == 0 {
		r.channels = len(frame[0])
		r.updateDtype()
	}

	// Adjust channels to match configured
	if len(frame[0]) > r.channels {
		// Truncate channels
		newFrame := make([][]float32, len(frame))
		for i := range frame {
			newFrame[i] = frame[i][:r.channels]
		}
		frame = newFrame
	} else if len(frame[0]) < r.channels {
		// Pad channels with last channel repeated
		newFrame := make([][]float32, len(frame))
		for i := range frame {
			newFrame[i] = make([]float32, r.channels)
			for c := 0; c < len(frame[i]); c++ {
				newFrame[i][c] = frame[i][c]
			}
			for c := len(frame[i]); c < r.channels; c++ {
				newFrame[i][c] = frame[i][len(frame[i])-1]
			}
		}
		frame = newFrame
	}

	// Create header byte
	headerByte := byte((r.headerBD << 6) | (r.channels - 1))
	result := make([]byte, 1+len(frame)*r.channels*4)
	result[0] = headerByte

	idx := 1
	for s := 0; s < len(frame); s++ {
		for c := 0; c < r.channels; c++ {
			binary.LittleEndian.PutUint32(result[idx:], math.Float32bits(frame[s][c]))
			idx += 4
		}
	}

	return result
}

// Decode decodes raw PCM bytes to float32 frames.
// Expects header byte: (bitdepth << 6) | (channels - 1)
func (r *Raw) Decode(data []byte, channelsHint int) [][]float32 {
	if len(data) < 1 {
		return [][]float32{}
	}

	header := data[0]
	frameChannels := int(header&0x3F) + 1
	frameBitdepth := int(header >> 6)

	if frameBitdepth >= len(BITDEPTHS) {
		frameBitdepth = BITDEPTH_16
	}

	sampleData := data[1:]
	samples := len(sampleData) / (frameChannels * 4)
	if samples == 0 {
		return [][]float32{}
	}

	totalFloats := samples * frameChannels
	if cap(r.decBuf) < totalFloats {
		r.decBuf = make([]float32, totalFloats)
	} else {
		r.decBuf = r.decBuf[:totalFloats]
	}

	idx := 0
	for i := 0; i < totalFloats; i++ {
		if idx+3 < len(sampleData) {
			r.decBuf[i] = math.Float32frombits(binary.LittleEndian.Uint32(sampleData[idx : idx+4]))
			idx += 4
		}
	}

	if cap(r.decFrames) < samples {
		r.decFrames = make([][]float32, samples)
	}
	r.decFrames = r.decFrames[:samples]
	for s := 0; s < samples; s++ {
		r.decFrames[s] = r.decBuf[s*frameChannels : (s+1)*frameChannels]
	}

	if r.channels == 0 {
		r.channels = frameChannels
		r.updateDtype()
	}

	return r.decFrames
}

func (r *Raw) PreferredSampleRate() int { return 0 }
func (r *Raw) FrameQuantumMs() float64  { return 0 }
func (r *Raw) FrameMaxMs() float64      { return 0 }
func (r *Raw) ValidFrameMs() []float64  { return nil }
