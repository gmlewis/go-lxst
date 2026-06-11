// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package opus

import (
	"math"
	"testing"
)

func TestOpus_Profiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		profile             int
		expectedSampleRate  int
		expectedChannels    int
		expectedBitrateCeil int
	}{
		{"VOICE_LOW", PROFILE_VOICE_LOW, 8000, 1, 6000},
		{"VOICE_MEDIUM", PROFILE_VOICE_MEDIUM, 24000, 1, 8000},
		{"VOICE_HIGH", PROFILE_VOICE_HIGH, 48000, 1, 16000},
		{"VOICE_MAX", PROFILE_VOICE_MAX, 48000, 2, 32000},
		{"AUDIO_MIN", PROFILE_AUDIO_MIN, 8000, 1, 8000},
		{"AUDIO_LOW", PROFILE_AUDIO_LOW, 12000, 1, 14000},
		{"AUDIO_MEDIUM", PROFILE_AUDIO_MEDIUM, 24000, 2, 28000},
		{"AUDIO_HIGH", PROFILE_AUDIO_HIGH, 48000, 2, 56000},
		{"AUDIO_MAX", PROFILE_AUDIO_MAX, 48000, 2, 128000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, err := NewOpus(tc.profile)
			if err != nil {
				t.Fatalf("NewOpus failed for profile %d: %v", tc.profile, err)
			}
			if o.PreferredSampleRate() != tc.expectedSampleRate {
				t.Errorf("Sample rate: got %d, want %d", o.PreferredSampleRate(), tc.expectedSampleRate)
			}
			if o.channels != tc.expectedChannels {
				t.Errorf("Channels: got %d, want %d", o.channels, tc.expectedChannels)
			}
			if o.bitrateCeiling != tc.expectedBitrateCeil {
				t.Errorf("Bitrate ceiling: got %d, want %d", o.bitrateCeiling, tc.expectedBitrateCeil)
			}
		})
	}
}

func TestOpus_InvalidProfile(t *testing.T) {
	t.Parallel()
	_, err := NewOpus(99)
	if err == nil {
		t.Error("Expected error for invalid profile")
	}
}

func TestOpus_FrameQuantization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    float64
		expected float64
	}{
		{20.0, 20.0},
		{21.0, 22.5},
		{22.0, 22.5},
		{23.0, 25.0},
		{24.0, 25.0},
	}

	for _, tc := range testCases {
		quantized := math.Ceil(tc.input/FRAME_QUANTA_MS) * FRAME_QUANTA_MS
		if quantized != tc.expected {
			t.Errorf("Quantize(%f) = %f, want %f", tc.input, quantized, tc.expected)
		}
	}
}

func TestOpus_MaxBytesPerFrame(t *testing.T) {
	t.Parallel()

	result := MaxBytesPerFrame(6000, 20.0)
	expected := int(math.Ceil((6000.0 / 8.0) * (20.0 / 1000.0)))
	if result != expected {
		t.Errorf("MaxBytesPerFrame(6000, 20) = %d, want %d", result, expected)
	}
}

func TestOpus_ValidFrameMs(t *testing.T) {
	t.Parallel()

	if len(ValidFrameMs) != 6 {
		t.Errorf("Expected 6 valid frame sizes, got %d", len(ValidFrameMs))
	}

	expected := []float64{2.5, 5, 10, 20, 40, 60}
	for i, v := range ValidFrameMs {
		if v != expected[i] {
			t.Errorf("ValidFrameMs[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestOpus_PreferredSampleRate(t *testing.T) {
	t.Parallel()
	o, err := NewOpus(PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	if o.PreferredSampleRate() != 8000 {
		t.Errorf("PreferredSampleRate = %d, want 8000", o.PreferredSampleRate())
	}
}

func TestOpus_ProfileConfig(t *testing.T) {
	t.Parallel()

	sr, ch, br, app := ProfileConfig(PROFILE_VOICE_LOW)
	if sr != 8000 || ch != 1 || br != 6000 || app != AppVoip {
		t.Errorf("ProfileConfig(VOICE_LOW) = (%d, %d, %d, %d), want (8000, 1, 6000, %d)", sr, ch, br, app, AppVoip)
	}

	sr, ch, br, app = ProfileConfig(PROFILE_AUDIO_MAX)
	if sr != 48000 || ch != 2 || br != 128000 || app != AppAudio {
		t.Errorf("ProfileConfig(AUDIO_MAX) = (%d, %d, %d, %d), want (48000, 2, 128000, %d)", sr, ch, br, app, AppAudio)
	}
}

func TestOpus_BitrateCeiling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		profile  int
		expected int
	}{
		{"VOICE_LOW", PROFILE_VOICE_LOW, 6000},
		{"VOICE_MEDIUM", PROFILE_VOICE_MEDIUM, 8000},
		{"VOICE_HIGH", PROFILE_VOICE_HIGH, 16000},
		{"VOICE_MAX", PROFILE_VOICE_MAX, 32000},
		{"AUDIO_MIN", PROFILE_AUDIO_MIN, 8000},
		{"AUDIO_LOW", PROFILE_AUDIO_LOW, 14000},
		{"AUDIO_MEDIUM", PROFILE_AUDIO_MEDIUM, 28000},
		{"AUDIO_HIGH", PROFILE_AUDIO_HIGH, 56000},
		{"AUDIO_MAX", PROFILE_AUDIO_MAX, 128000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, err := NewOpus(tc.profile)
			if err != nil {
				t.Skipf("Opus not available: %v", err)
			}
			if o.bitrateCeiling != tc.expected {
				t.Errorf("Profile %d: bitrateCeiling = %d, want %d", tc.profile, o.bitrateCeiling, tc.expected)
			}
		})
	}
}

func TestGetProfileConfig(t *testing.T) {
	t.Parallel()

	cfg, ok := GetProfileConfig(PROFILE_VOICE_HIGH)
	if !ok {
		t.Fatal("GetProfileConfig should return true for valid profile")
	}
	if cfg.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cfg.SampleRate)
	}
	if cfg.Channels != 1 {
		t.Errorf("Channels = %d, want 1", cfg.Channels)
	}
	if cfg.BitrateCeiling != 16000 {
		t.Errorf("BitrateCeiling = %d, want 16000", cfg.BitrateCeiling)
	}
}
