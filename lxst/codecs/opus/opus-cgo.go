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
		return fmt.Errorf("%w: %v", ErrUnsupportedProfile, profile)
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

	// frame is [samples][channels] — outer dimension is samples,
	// inner dimension is channels.
	samples := len(frame)
	channels := len(frame[0])

	// Adjust channel count to match the encoder's expected input channels.
	if channels > o.inputChannels {
		// Trim extra channels.
		for i := range frame {
			frame[i] = frame[i][:o.inputChannels]
		}
		channels = o.inputChannels
	} else if channels < o.inputChannels {
		// Pad by duplicating the last channel.
		for i := range frame {
			extra := make([]float32, o.inputChannels-channels)
			lastVal := frame[i][channels-1]
			for c := range extra {
				extra[c] = lastVal
			}
			frame[i] = append(frame[i], extra...)
		}
		channels = o.inputChannels
	}

	// Interleave samples: [s0ch0, s0ch1, s1ch0, s1ch1, ...]
	inputSamples := make([]int16, samples*channels)
	for i := 0; i < samples; i++ {
		for c := 0; c < channels; c++ {
			val := frame[i][c] * TYPE_MAP_FACTOR
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			inputSamples[i*channels+c] = int16(val)
		}
	}

	frameDurationMs := float64(samples) / float64(o.sourceSampleRate) * 1000.0
	o.updateBitrate(frameDurationMs)

	if !o.encoderConfigured {
		o.inputChannels = o.channels
		o.encoderConfigured = true
	}

	maxBytes := MaxBytesPerFrame(o.bitrateCeiling, frameDurationMs)
	encoded, err := o.opusEncoder.Encode(inputSamples, samples, maxBytes)
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

	// Use a generous maximum frameSize so the decoder can handle any
	// encoded frame duration (up to 120ms at 48kHz = 5760 samples).
	// The actual decoded output will be trimmed to the real frame size.
	frameSize := 5760
	decoded, err := o.opusDecoder.Decode(frameBytes, frameSize, false)
	if err != nil {
		decoded, err = o.opusDecoder.Decode(frameBytes, 960, false)
		if err != nil {
			return [][]float32{}
		}
	}

	// decoded is interleaved int16: [s0ch0, s0ch1, s1ch0, s1ch1, ...]
	// Convert to [][]float32 where outer dim is samples, inner is channels.
	samplesPerChannel := len(decoded) / o.channels
	result := make([][]float32, samplesPerChannel)
	for i := 0; i < samplesPerChannel; i++ {
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
func (o *Opus) Channels() int            { return o.channels }

func (o *Opus) SetSourceSampleRate(rate int) { o.sourceSampleRate = rate }
func (o *Opus) SetSinkSampleRate(rate int)   { o.sinkSampleRate = rate }
