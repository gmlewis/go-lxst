// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package codec2 implements the Codec2 audio codec.
// Note: This requires CGO and libcodec2 to be installed on the system.
// For now, we provide a stub implementation that can be completed when libcodec2 is available.
package codec2

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedMode = errors.New("unsupported codec2 mode")
	ErrNotInitialized  = errors.New("codec2 not initialized - requires CGO with libcodec2")
)

// Codec2 mode constants matching Python LXST
const (
	MODE_700C = 700
	MODE_1200 = 1200
	MODE_1300 = 1300
	MODE_1400 = 1400
	MODE_1600 = 1600
	MODE_2400 = 2400
	MODE_3200 = 3200
	MODE_700B = 701 // 700B mode
)

const (
	INPUT_RATE      = 8000
	OUTPUT_RATE     = 8000
	FRAME_QUANTA_MS = 40.0
	TYPE_MAP_FACTOR = 32767 // int16 max
)

var MODE_HEADERS = map[int]byte{
	MODE_700C: 0x00,
	MODE_1200: 0x01,
	MODE_1300: 0x02,
	MODE_1400: 0x03,
	MODE_1600: 0x04,
	MODE_2400: 0x05,
	MODE_3200: 0x06,
	MODE_700B: 0x07,
}

var HEADER_MODES = map[byte]int{
	0x00: MODE_700C,
	0x01: MODE_1200,
	0x02: MODE_1300,
	0x03: MODE_1400,
	0x04: MODE_1600,
	0x05: MODE_2400,
	0x06: MODE_3200,
	0x07: MODE_700B,
}

var validModes = []int{
	MODE_700C, MODE_1200, MODE_1300, MODE_1400,
	MODE_1600, MODE_2400, MODE_3200, MODE_700B,
}

// Codec2 implements the Codec interface for Codec2 audio.
// This is a stub - real implementation requires CGO binding to libcodec2.
type Codec2 struct {
	mode             int
	frameQuantumMs   float64
	channels         int
	bitdepth         int
	c2               interface{} // Would be *pycodec2.Codec2 in Python
	outputSampleRate int
	modeHeader       byte
	initialized      bool
}

// NewCodec2 creates a new Codec2 codec with the given mode.
func NewCodec2(mode int) (*Codec2, error) {
	if !isValidMode(mode) {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedMode, mode)
	}

	c := &Codec2{
		mode:             mode,
		frameQuantumMs:   FRAME_QUANTA_MS,
		channels:         1,
		bitdepth:         16,
		outputSampleRate: OUTPUT_RATE,
		modeHeader:       MODE_HEADERS[mode],
		initialized:      false,
	}

	// TODO: Initialize actual libcodec2 via CGO
	// c.c2 = codec2_create(mode)
	// c.initialized = true

	return c, nil
}

func isValidMode(mode int) bool {
	for _, m := range validModes {
		if m == mode {
			return true
		}
	}
	return false
}

func (c *Codec2) SetMode(mode int) error {
	if !isValidMode(mode) {
		return fmt.Errorf("%w: %d", ErrUnsupportedMode, mode)
	}
	c.mode = mode
	c.modeHeader = MODE_HEADERS[mode]
	// TODO: Reinitialize libcodec2 with new mode
	return nil
}

func (c *Codec2) Encode(frame [][]float32) []byte {
	if len(frame) == 0 || len(frame[0]) == 0 {
		return []byte{}
	}

	// Stub implementation: return just the mode header
	// TODO: Actual encoding via CGO would go here
	return []byte{c.modeHeader}
}

func (c *Codec2) Decode(frameBytes []byte, channelsHint int) [][]float32 {
	if len(frameBytes) < 1 {
		return [][]float32{}
	}

	// Check for mode header (always parse header regardless of initialization)
	header := frameBytes[0]
	if m, ok := HEADER_MODES[header]; ok && m != c.mode {
		_ = c.SetMode(m)
	}
	// frameBytes = frameBytes[1:] // Keep header for stub

	// Return empty for stub
	return [][]float32{}
}

func (c *Codec2) PreferredSampleRate() int {
	return INPUT_RATE
}

func (c *Codec2) FrameQuantumMs() float64 {
	return c.frameQuantumMs
}

func (c *Codec2) FrameMaxMs() float64 {
	return 0 // No max in Codec2
}

func (c *Codec2) ValidFrameMs() []float64 {
	return []float64{FRAME_QUANTA_MS} // Only 40ms frames
}
