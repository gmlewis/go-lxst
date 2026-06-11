// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

package opus

import (
	"errors"
	"fmt"

	layeh_gopus "layeh.com/gopus"
)

var (
	ErrUnsupportedProfile = errors.New("unsupported profile")
	ErrInvalidFrameSize   = errors.New("invalid frame size")
)

// Opus implements the Codec interface for Opus audio using CGO/libopus.
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
		profile:          profile,
		frameQuantumMs:   FRAME_QUANTA_MS,
		frameMaxMs:       FRAME_MAX_MS,
		validFrameMs:     ValidFrameMs,
		channels:         cfg.Channels,
		inputChannels:    cfg.Channels,
		outputChannels:   2,
		bitdepth:         16,
		bitrateCeiling:   cfg.BitrateCeiling,
		outputBytes:      0,
		outputMs:         0,
		outputBitrate:    0,
		sourceSampleRate: cfg.SampleRate,
		sinkSampleRate:   cfg.SampleRate,
	}

	app := layeh_gopus.Voip
	if cfg.Application == AppAudio {
		app = layeh_gopus.Audio
	}

	enc, err := layeh_gopus.NewEncoder(cfg.SampleRate, cfg.Channels, app)
	if err != nil {
		return nil, err
	}
	o.opusEncoder = enc

	dec, err := layeh_gopus.NewDecoder(cfg.SampleRate, cfg.Channels)
	if err != nil {
		return nil, err
	}
	o.opusDecoder = dec

	return o, nil
}

func (o *Opus) profileChannels(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 1
	}
	return cfg.Channels
}

func (o *Opus) profileSampleRate(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 8000
	}
	return cfg.SampleRate
}

func (o *Opus) profileApplication(profile int) layeh_gopus.Application {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return layeh_gopus.Voip
	}
	if cfg.Application == AppAudio {
		return layeh_gopus.Audio
	}
	return layeh_gopus.Voip
}

func (o *Opus) profileBitrateCeiling(profile int) int {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 6000
	}
	return cfg.BitrateCeiling
}

func (o *Opus) SetProfile(profile int) error {
	if !isValidProfile(profile) {
		return fmt.Errorf("%w: %d", ErrUnsupportedProfile, profile)
	}
	o.profile = profile
	cfg := profileConfigs[profile]
	o.channels = cfg.Channels
	o.inputChannels = cfg.Channels
	o.sourceSampleRate = cfg.SampleRate
	o.opusEncoder.SetApplication(o.profileApplication(profile))
	return nil
}

func (o *Opus) updateBitrate(frameDurationMs float64) {
	o.bitrateCeiling = o.profileBitrateCeiling(o.profile)
	MaxBytesPerFrame(o.bitrateCeiling, frameDurationMs)
	o.opusEncoder.SetBitrate(o.bitrateCeiling)
}

func (o *Opus) Encode(frame [][]float32) []byte {
	if len(frame) == 0 || len(frame[0]) == 0 {
		return []byte{}
	}

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

	frameSize := o.sourceSampleRate / 50
	if frameSize < 120 {
		frameSize = 120
	}
	if frameSize > 960 {
		frameSize = 960
	}
	decoded, err := o.opusDecoder.Decode(frameBytes, frameSize, false)
	if err != nil {
		decoded, err = o.opusDecoder.Decode(frameBytes, 960, false)
		if err != nil {
			return [][]float32{}
		}
	}

	result := make([][]float32, len(decoded)/o.channels)
	for i := range result {
		result[i] = make([]float32, o.channels)
		for c := 0; c < o.channels; c++ {
			result[i][c] = float32(decoded[i*o.channels+c]) / TYPE_MAP_FACTOR
		}
	}

	return result
}

func (o *Opus) PreferredSampleRate() int { return o.sourceSampleRate }
func (o *Opus) FrameQuantumMs() float64  { return o.frameQuantumMs }
func (o *Opus) FrameMaxMs() float64      { return o.frameMaxMs }
func (o *Opus) ValidFrameMs() []float64  { return o.validFrameMs }

func (o *Opus) SetSourceSampleRate(rate int) { o.sourceSampleRate = rate }
func (o *Opus) SetSinkSampleRate(rate int)   { o.sinkSampleRate = rate }
