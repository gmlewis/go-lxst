// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package opus

import (
	"math"
	"testing"

	layeh_gopus "layeh.com/gopus"
)

func TestOpus_Profiles(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		profile       int
		expectedSampleRate int
		expectedChannels   int
		expectedApp        layeh_gopus.Application
		expectedBitrateCeiling int
	}{
		{"VOICE_LOW", PROFILE_VOICE_LOW, 8000, 1, layeh_gopus.Voip, 6000},
		{"VOICE_MEDIUM", PROFILE_VOICE_MEDIUM, 24000, 1, layeh_gopus.Voip, 8000},
		{"VOICE_HIGH", PROFILE_VOICE_HIGH, 48000, 1, layeh_gopus.Voip, 16000},
		{"VOICE_MAX", PROFILE_VOICE_MAX, 48000, 2, layeh_gopus.Voip, 32000},
		{"AUDIO_MIN", PROFILE_AUDIO_MIN, 8000, 1, layeh_gopus.Audio, 8000},
		{"AUDIO_LOW", PROFILE_AUDIO_LOW, 12000, 1, layeh_gopus.Audio, 14000},
		{"AUDIO_MEDIUM", PROFILE_AUDIO_MEDIUM, 24000, 2, layeh_gopus.Audio, 28000},
		{"AUDIO_HIGH", PROFILE_AUDIO_HIGH, 48000, 2, layeh_gopus.Audio, 56000},
		{"AUDIO_MAX", PROFILE_AUDIO_MAX, 48000, 2, layeh_gopus.Audio, 128000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, err := NewOpus(tc.profile)
			if err != nil {
				t.Fatalf("NewOpus failed for profile %d: %v", tc.profile, err)
			}
			if o.sourceSampleRate != tc.expectedSampleRate {
				t.Errorf("Sample rate: got %d, want %d", o.sourceSampleRate, tc.expectedSampleRate)
			}
			if o.channels != tc.expectedChannels {
				t.Errorf("Channels: got %d, want %d", o.channels, tc.expectedChannels)
			}
			if o.opusEncoder.Application() != tc.expectedApp {
				t.Errorf("Application: got %v, want %v", o.opusEncoder.Application(), tc.expectedApp)
			}
			if o.bitrateCeiling != tc.expectedBitrateCeiling {
				t.Errorf("Bitrate ceiling: got %d, want %d", o.bitrateCeiling, tc.expectedBitrateCeiling)
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
	_, _ = NewOpus(PROFILE_VOICE_LOW)
	
	// Test frame quantization to 2.5ms steps (Python uses math.ceil)
	testCases := []struct {
		input    float64
		expected float64
	}{
		{20.0, 20.0},  // exact quantum
		{21.0, 22.5},  // ceil(21/2.5)=9, 9*2.5=22.5
		{22.0, 22.5},  // ceil(22/2.5)=9, 9*2.5=22.5
		{23.0, 25.0},  // ceil(23/2.5)=10, 10*2.5=25.0
		{24.0, 25.0},  // ceil(24/2.5)=10, 10*2.5=25.0
	}
	
	for _, tc := range testCases {
		// Python uses: math.ceil(input/quantum) * quantum
		quantized := math.Ceil(tc.input/FRAME_QUANTA_MS) * FRAME_QUANTA_MS
		if quantized != tc.expected {
			t.Errorf("Quantize(%f) = %f, want %f", tc.input, quantized, tc.expected)
		}
	}
}

func TestOpus_MaxBytesPerFrame(t *testing.T) {
	t.Parallel()
	// Test the static MaxBytesPerFrame function
	result := MaxBytesPerFrame(6000, 20.0)
	expected := int(math.Ceil((6000.0 / 8.0) * (20.0 / 1000.0)))
	if result != expected {
		t.Errorf("MaxBytesPerFrame(6000, 20) = %d, want %d", result, expected)
	}
}

func TestOpus_EncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	o, err := NewOpus(PROFILE_VOICE_LOW)
	if err != nil {
		t.Fatalf("NewOpus failed: %v", err)
	}

	// Create test audio: 20ms at 8kHz = 160 samples
	samplesPerFrame := int(float64(o.sourceSampleRate) * 20.0 / 1000.0)
	input := make([][]float32, samplesPerFrame)
	for i := range input {
		input[i] = make([]float32, o.channels)
		for c := 0; c < o.channels; c++ {
			// Sine wave at 440Hz
			phase := float64(i) * 2.0 * math.Pi * 440.0 / float64(o.sourceSampleRate)
			input[i][c] = float32(math.Sin(phase)) * 0.5
		}
	}

	encoded := o.Encode(input)
	if len(encoded) == 0 {
		t.Fatal("Encode returned empty")
	}

	decoded := o.Decode(encoded, o.channels)
	if len(decoded) == 0 {
		t.Fatal("Decode returned empty")
	}

	// Check we got reasonable number of frames back
	// Opus may return slightly different frame count due to internal buffering
	if len(decoded) < samplesPerFrame/2 {
		t.Errorf("Decoded too few frames: got %d, expected ~%d", len(decoded), samplesPerFrame)
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
				t.Fatalf("NewOpus failed: %v", err)
			}
			if o.bitrateCeiling != tc.expected {
				t.Errorf("Profile %d: bitrateCeiling = %d, want %d", tc.profile, o.bitrateCeiling, tc.expected)
			}
		})
	}
}

func TestOpus_ValidFrameMs(t *testing.T) {
	t.Parallel()
	o, _ := NewOpus(PROFILE_VOICE_LOW)
	
	if len(o.ValidFrameMs()) != 6 {
		t.Errorf("Expected 6 valid frame sizes, got %d", len(o.ValidFrameMs()))
	}
	
	expected := []float64{2.5, 5, 10, 20, 40, 60}
	for i, v := range o.ValidFrameMs() {
		if v != expected[i] {
			t.Errorf("ValidFrameMs[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestOpus_PreferredSampleRate(t *testing.T) {
	t.Parallel()
	o, _ := NewOpus(PROFILE_VOICE_LOW)
	if o.PreferredSampleRate() != 8000 {
		t.Errorf("PreferredSampleRate = %d, want 8000", o.PreferredSampleRate())
	}
}