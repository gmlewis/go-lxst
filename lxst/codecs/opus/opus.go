// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package opus implements the Opus audio codec with profile support.
package opus

import (
	"errors"
	"fmt"
	"math"

	layeh_gopus "layeh.com/gopus"
)

var (
	ErrUnsupportedProfile = errors.New("unsupported profile")
	ErrInvalidFrameSize   = errors.New("invalid frame size")
)

// Opus profile constants matching Python LXST
const (
	PROFILE_VOICE_LOW     = 0x00
	PROFILE_VOICE_MEDIUM  = 0x01
	PROFILE_VOICE_HIGH    = 0x02
	PROFILE_VOICE_MAX     = 0x03
	PROFILE_AUDIO_MIN     = 0x04
	PROFILE_AUDIO_LOW     = 0x05
	PROFILE_AUDIO_MEDIUM  = 0x06
	PROFILE_AUDIO_HIGH    = 0x07
	PROFILE_AUDIO_MAX     = 0x08
)

const (
	FRAME_QUANTA_MS = 2.5
	FRAME_MAX_MS    = 60.0
	TYPE_MAP_FACTOR = 32767 // int16 max
)

var VALID_FRAME_MS = []float64{2.5, 5, 10, 20, 40, 60}

type profileConfig struct {
	sampleRate       int
	channels         int
	application      layeh_gopus.Application
	bitrateCeiling   int
}

var profileConfigs = map[int]profileConfig{
	PROFILE_VOICE_LOW:    {8000, 1, layeh_gopus.Voip, 6000},
	PROFILE_VOICE_MEDIUM: {24000, 1, layeh_gopus.Voip, 8000},
	PROFILE_VOICE_HIGH:   {48000, 1, layeh_gopus.Voip, 16000},
	PROFILE_VOICE_MAX:    {48000, 2, layeh_gopus.Voip, 32000},
	PROFILE_AUDIO_MIN:    {8000, 1, layeh_gopus.Audio, 8000},
	PROFILE_AUDIO_LOW:    {12000, 1, layeh_gopus.Audio, 14000},
	PROFILE_AUDIO_MEDIUM: {24000, 2, layeh_gopus.Audio, 28000},
	PROFILE_AUDIO_HIGH:   {48000, 2, layeh_gopus.Audio, 56000},
	PROFILE_AUDIO_MAX:    {48000, 2, layeh_gopus.Audio, 128000},
}

var validProfiles = []int{
	PROFILE_VOICE_LOW, PROFILE_VOICE_MEDIUM, PROFILE_VOICE_HIGH, PROFILE_VOICE_MAX,
	PROFILE_AUDIO_MIN, PROFILE_AUDIO_LOW, PROFILE_AUDIO_MEDIUM, PROFILE_AUDIO_HIGH, PROFILE_AUDIO_MAX,
}

// Opus implements the Codec interface for Opus audio.
type Opus struct {
	profile           int
	frameQuantumMs    float64
	frameMaxMs        float64
	validFrameMs      []float64
	channels          int
	inputChannels     int
	outputChannels    int
	bitdepth          int
	opusEncoder       *layeh_gopus.Encoder
	opusDecoder       *layeh_gopus.Decoder
	encoderConfigured bool
	decoderConfigured bool
	bitrateCeiling    int
	outputBytes       int
	outputMs          int
	outputBitrate     int

	// Source/sink references for resampling
	sourceSampleRate  int
	sinkSampleRate    int
}

// NewOpus creates a new Opus codec with the given profile.
func NewOpus(profile int) (*Opus, error) {
	if !isValidProfile(profile) {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedProfile, profile)
	}

	cfg := profileConfigs[profile]

	o := &Opus{
		profile:           profile,
		frameQuantumMs:    FRAME_QUANTA_MS,
		frameMaxMs:        FRAME_MAX_MS,
		validFrameMs:      VALID_FRAME_MS,
		channels:          cfg.channels,
		inputChannels:     cfg.channels,
		outputChannels:    2,
		bitdepth:          16,
		bitrateCeiling:    cfg.bitrateCeiling,
		outputBytes:       0,
		outputMs:          0,
		outputBitrate:     0,
		sourceSampleRate:  cfg.sampleRate,
		sinkSampleRate:    cfg.sampleRate,
	}

	// Create encoder/decoder with initial config
	enc, err := layeh_gopus.NewEncoder(cfg.sampleRate, cfg.channels, cfg.application)
	if err != nil {
		return nil, err
	}
	o.opusEncoder = enc

	dec, err := layeh_gopus.NewDecoder(cfg.sampleRate, cfg.channels)
	if err != nil {
		return nil, err
	}
	o.opusDecoder = dec

	return o, nil
}

func isValidProfile(profile int) bool {
	for _, p := range validProfiles {
		if p == profile {
			return true
		}
	}
	return false
}

func (o *Opus) profileChannels(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 1
	}
	return cfg.channels
}

func (o *Opus) profileSampleRate(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 8000
	}
	return cfg.sampleRate
}

func (o *Opus) profileApplication(profile int) layeh_gopus.Application {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return layeh_gopus.Voip
	}
	return cfg.application
}

func (o *Opus) profileBitrateCeiling(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 6000
	}
	return cfg.bitrateCeiling
}

// MaxBytesPerFrame calculates max bytes per frame for given bitrate and frame duration.
func MaxBytesPerFrame(bitrateCeiling int, frameDurationMs float64) int {
	return int(math.Ceil((float64(bitrateCeiling) / 8.0) * (frameDurationMs / 1000.0)))
}

func (o *Opus) SetProfile(profile int) error {
	if !isValidProfile(profile) {
		return fmt.Errorf("%w: %d", ErrUnsupportedProfile, profile)
	}
	o.profile = profile
	cfg := profileConfigs[profile]
	o.channels = cfg.channels
	o.inputChannels = cfg.channels
	o.sourceSampleRate = cfg.sampleRate
	o.opusEncoder.SetApplication(cfg.application)
	return nil
}

func (o *Opus) updateBitrate(frameDurationMs float64) {
	o.bitrateCeiling = o.profileBitrateCeiling(o.profile)
	maxBytesPerFrame := MaxBytesPerFrame(o.bitrateCeiling, frameDurationMs)
	o.opusEncoder.SetBitrate(o.bitrateCeiling) // Use SetBitrate instead
	_ = maxBytesPerFrame // Used in Encode
}

func (o *Opus) Encode(frame [][]float32) []byte {
	if len(frame) == 0 {
		return []byte{}
	}
	if len(frame[0]) == 0 {
		return []byte{}
	}

	// Handle channel mismatch
	if len(frame[0]) > o.inputChannels {
		newFrame := make([][]float32, len(frame))
		for i := range frame {
			newFrame[i] = frame[i][:o.inputChannels]
		}
		frame = newFrame
	} else if len(frame[0]) < o.inputChannels {
		newFrame := make([][]float32, len(frame))
		for i := range frame {
			newFrame[i] = make([]float32, o.inputChannels)
			for c := 0; c < len(frame[i]); c++ {
				newFrame[i][c] = frame[i][c]
			}
			for c := len(frame[i]); c < o.inputChannels; c++ {
				newFrame[i][c] = frame[i][len(frame[i])-1]
			}
		}
		frame = newFrame
	}

	// Convert float32 to int16
	inputSamples := make([]int16, len(frame)*o.inputChannels)
	for i, s := range frame {
		for c := 0; c < o.inputChannels; c++ {
			val := s[c] * TYPE_MAP_FACTOR
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			inputSamples[i*o.inputChannels+c] = int16(val)
		}
	}

	frameDurationMs := float64(len(frame)) / float64(o.sourceSampleRate) * 1000.0
	o.updateBitrate(frameDurationMs)

	if !o.encoderConfigured {
		o.inputChannels = o.channels
		o.encoderConfigured = true
	}

	maxBytes := MaxBytesPerFrame(o.bitrateCeiling, frameDurationMs)
	encoded, err := o.opusEncoder.Encode(inputSamples, len(frame), maxBytes)
	if err != nil {
		return []byte{}
	}

	o.outputBytes += len(encoded)
	o.outputMs += int(frameDurationMs)
	if o.outputMs > 0 {
		o.outputBitrate = (o.outputBytes * 8 * 1000) / o.outputMs
	}

	return encoded
}

func (o *Opus) Decode(frameBytes []byte, channelsHint int) [][]float32 {
	if !o.decoderConfigured {
		if channelsHint > 0 {
			o.channels = channelsHint
		} else if o.sinkSampleRate != 0 {
			o.channels = o.outputChannels
			if o.channels > o.inputChannels {
				o.channels = o.inputChannels
			}
		} else {
			o.channels = o.outputChannels
			if o.channels > o.inputChannels {
				o.channels = o.inputChannels
			}
		}
		o.decoderConfigured = true
	}

	// Use default frame size for decoder (960 samples max at 48kHz for 20ms)
	// For 8kHz mono, max frame is 160 samples for 20ms
	frameSize := o.sourceSampleRate / 50 // 20ms worth of samples
	if frameSize < 120 {
		frameSize = 120
	}
	if frameSize > 960 {
		frameSize = 960
	}
	decoded, err := o.opusDecoder.Decode(frameBytes, frameSize, false)
	if err != nil {
		// Try with larger frame size
		decoded, err = o.opusDecoder.Decode(frameBytes, 960, false)
		if err != nil {
			return [][]float32{}
		}
	}

	// Convert int16 to float32
	result := make([][]float32, len(decoded)/o.channels)
	for i := range result {
		result[i] = make([]float32, o.channels)
		for c := 0; c < o.channels; c++ {
			result[i][c] = float32(decoded[i*o.channels+c]) / TYPE_MAP_FACTOR
		}
	}

	return result
}

func (o *Opus) PreferredSampleRate() int {
	return o.sourceSampleRate
}

func (o *Opus) FrameQuantumMs() float64 {
	return o.frameQuantumMs
}

func (o *Opus) FrameMaxMs() float64 {
	return o.frameMaxMs
}

func (o *Opus) ValidFrameMs() []float64 {
	return o.validFrameMs
}

// ProfileConfig returns the sample rate, channels, bitrate ceiling, and application
// for a given profile.
func ProfileConfig(profile int) (sampleRate int, channels int, bitrateCeiling int, application int) {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 8000, 1, 6000, int(layeh_gopus.Voip)
	}
	return cfg.sampleRate, cfg.channels, cfg.bitrateCeiling, int(cfg.application)
}

// SetSourceSampleRate sets the source sample rate for resampling.
func (o *Opus) SetSourceSampleRate(rate int) {
	o.sourceSampleRate = rate
}

// SetSinkSampleRate sets the sink sample rate for resampling.
func (o *Opus) SetSinkSampleRate(rate int) {
	o.sinkSampleRate = rate
}