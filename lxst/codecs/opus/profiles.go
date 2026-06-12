// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package opus implements the Opus audio codec for the LXST library.
// It provides Opus encoding and decoding with multiple profile presets
// (voice, music, low-bitrate) for different use cases. When CGO is
// enabled, it wraps libopus for high-performance encoding/decoding.
// When CGO is disabled, a pure-Go stub is provided for codec metadata.
package opus

import (
	"errors"
	"math"
)

// Opus profile constants matching Python LXST.
const (
	PROFILE_VOICE_LOW    = 0x00
	PROFILE_VOICE_MEDIUM = 0x01
	PROFILE_VOICE_HIGH   = 0x02
	PROFILE_VOICE_MAX    = 0x03
	PROFILE_AUDIO_MIN    = 0x04
	PROFILE_AUDIO_LOW    = 0x05
	PROFILE_AUDIO_MEDIUM = 0x06
	PROFILE_AUDIO_HIGH   = 0x07
	PROFILE_AUDIO_MAX    = 0x08
)

// Frame and codec constants.
const (
	FRAME_QUANTA_MS = 2.5
	FRAME_MAX_MS    = 60.0
	TYPE_MAP_FACTOR = 32767 // int16 max
)

// ValidFrameMs returns the valid Opus frame durations in milliseconds.
var ValidFrameMs = []float64{2.5, 5, 10, 20, 40, 60}

var validProfiles = []int{
	PROFILE_VOICE_LOW, PROFILE_VOICE_MEDIUM, PROFILE_VOICE_HIGH, PROFILE_VOICE_MAX,
	PROFILE_AUDIO_MIN, PROFILE_AUDIO_LOW, PROFILE_AUDIO_MEDIUM, PROFILE_AUDIO_HIGH, PROFILE_AUDIO_MAX,
}

// Application constants for Opus encoding.
const (
	AppVoip  = 0
	AppAudio = 1
)

// ProfileConfigEntry holds the configuration for an Opus profile.
type ProfileConfigEntry struct {
	SampleRate     int
	Channels       int
	Application    int
	BitrateCeiling int
}

var profileConfigs = map[int]ProfileConfigEntry{
	PROFILE_VOICE_LOW:    {8000, 1, AppVoip, 6000},
	PROFILE_VOICE_MEDIUM: {24000, 1, AppVoip, 8000},
	PROFILE_VOICE_HIGH:   {48000, 1, AppVoip, 16000},
	PROFILE_VOICE_MAX:    {48000, 2, AppVoip, 32000},
	PROFILE_AUDIO_MIN:    {8000, 1, AppAudio, 8000},
	PROFILE_AUDIO_LOW:    {12000, 1, AppAudio, 14000},
	PROFILE_AUDIO_MEDIUM: {24000, 2, AppAudio, 28000},
	PROFILE_AUDIO_HIGH:   {48000, 2, AppAudio, 56000},
	PROFILE_AUDIO_MAX:    {48000, 2, AppAudio, 128000},
}

// GetProfileConfig returns the ProfileConfigEntry for a given profile.
func GetProfileConfig(profile int) (ProfileConfigEntry, bool) {
	cfg, ok := profileConfigs[profile]
	return cfg, ok
}

func isValidProfile(profile int) bool {
	for _, p := range validProfiles {
		if p == profile {
			return true
		}
	}
	return false
}

// MaxBytesPerFrame calculates max bytes per frame for given bitrate and frame duration.
func MaxBytesPerFrame(bitrateCeiling int, frameDurationMs float64) int {
	return int(math.Ceil((float64(bitrateCeiling) / 8.0) * (frameDurationMs / 1000.0)))
}

// ProfileConfig returns the sample rate, channels, bitrate ceiling, and application
// type for a given profile. This is the exported version.
func ProfileConfig(profile int) (sampleRate int, channels int, bitrateCeiling int, application int) {
	cfg, ok := profileConfigs[profile]
	if !ok {
		return 8000, 1, 6000, AppVoip
	}
	return cfg.SampleRate, cfg.Channels, cfg.BitrateCeiling, cfg.Application
}

// ErrNoCGO is returned when Opus encoding/decoding is attempted
// but the package was built without CGO (and thus without libopus).
var ErrNoCGO = errors.New("opus codec requires CGO; build with CGO_ENABLED=1")
