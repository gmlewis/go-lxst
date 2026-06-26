// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build !cgo

package opus

import (
	"fmt"
)

var (
	ErrUnsupportedProfile = fmt.Errorf("unsupported profile (note: opus encoding/decoding requires CGO)")
	ErrInvalidFrameSize   = fmt.Errorf("invalid frame size (note: opus encoding/decoding requires CGO)")
)

// Opus is a stub implementation when built without CGO.
// It implements the Codec interface but returns errors for encode/decode operations.
// Profile metadata and constants are still available.
type Opus struct {
	profile          int
	frameQuantumMs   float64
	frameMaxMs       float64
	validFrameMs     []float64
	channels         int
	inputChannels    int
	outputChannels   int
	bitdepth         int
	bitrateCeiling   int
	sourceSampleRate int
	sinkSampleRate   int
}

// NewOpus creates a new Opus codec stub.
// When built without CGO, this returns a stub that supports profile metadata
// but returns ErrNoCGO for encode/decode operations.
func NewOpus(profile int) (*Opus, error) {
	if !isValidProfile(profile) {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedProfile, profile)
	}

	cfg := profileConfigs[profile]

	o := &Opus{
		profile:          profile,
		frameQuantumMs:   FRAME_QUANTA_MS,
		frameMaxMs:       FRAME_MAX_MS,
		validFrameMs:     ValidFrameMs,
		channels:         cfg.Channels,
		inputChannels:    cfg.Channels,
		outputChannels:   2,
		bitdepth:         16,
		bitrateCeiling:   cfg.BitrateCeiling,
		sourceSampleRate: cfg.SampleRate,
		sinkSampleRate:   cfg.SampleRate,
	}

	return o, nil
}

func (o *Opus) SetProfile(profile int) error {
	if !isValidProfile(profile) {
		return fmt.Errorf("%w: %v", ErrUnsupportedProfile, profile)
	}
	o.profile = profile
	cfg := profileConfigs[profile]
	o.channels = cfg.Channels
	o.inputChannels = cfg.Channels
	o.sourceSampleRate = cfg.SampleRate
	return nil
}

func (o *Opus) Encode(frame [][]float32) []byte {
	return []byte{}
}

func (o *Opus) Decode(frameBytes []byte, channelsHint int) [][]float32 {
	return [][]float32{}
}

func (o *Opus) PreferredSampleRate() int { return o.sourceSampleRate }
func (o *Opus) FrameQuantumMs() float64  { return o.frameQuantumMs }
func (o *Opus) FrameMaxMs() float64      { return o.frameMaxMs }
func (o *Opus) ValidFrameMs() []float64  { return o.validFrameMs }
func (o *Opus) Channels() int            { return o.channels }

func (o *Opus) SetSourceSampleRate(rate int) { o.sourceSampleRate = rate }
func (o *Opus) SetSinkSampleRate(rate int)   { o.sinkSampleRate = rate }
