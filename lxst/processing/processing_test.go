// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package processing

import (
	"math"
	"testing"
)

func TestRMS_Silence(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.0, 0.0}
	}

	rms := RMS(frame)
	if rms != 0.0 {
		t.Errorf("RMS of silence should be 0, got %f", rms)
	}
}

func TestRMS_SingleChannel(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{1.0}
	frame[1] = []float32{-1.0}
	frame[2] = []float32{1.0}
	frame[3] = []float32{-1.0}

	rms := RMS(frame)
	if math.Abs(rms-1.0) > 0.001 {
		t.Errorf("RMS of [1,-1,1,-1] should be 1.0, got %f", rms)
	}
}

func TestRMS_Stereo(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 2)
	frame[0] = []float32{0.5, 0.5}
	frame[1] = []float32{0.5, 0.5}

	rms := RMS(frame)
	if math.Abs(rms-0.5) > 0.001 {
		t.Errorf("RMS of stereo 0.5 should be 0.5, got %f", rms)
	}
}

func TestRMS_KnownValue(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{0.5}
	frame[1] = []float32{0.5}
	frame[2] = []float32{0.5}
	frame[3] = []float32{0.5}

	rms := RMS(frame)
	if math.Abs(rms-0.5) > 0.001 {
		t.Errorf("RMS of [0.5,0.5,0.5,0.5] should be 0.5, got %f", rms)
	}
}

func TestPeak_Silence(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.0, 0.0}
	}

	peak := Peak(frame)
	if peak != 0.0 {
		t.Errorf("Peak of silence should be 0, got %f", peak)
	}
}

func TestPeak_Positive(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{0.1}
	frame[1] = []float32{0.9}
	frame[2] = []float32{0.3}
	frame[3] = []float32{0.2}

	peak := Peak(frame)
	if math.Abs(peak-0.9) > 0.001 {
		t.Errorf("Peak should be 0.9, got %f", peak)
	}
}

func TestPeak_Negative(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{-0.1}
	frame[1] = []float32{-0.8}
	frame[2] = []float32{-0.3}
	frame[3] = []float32{-0.2}

	peak := Peak(frame)
	if math.Abs(peak-0.8) > 0.001 {
		t.Errorf("Peak of absolute values should be 0.8, got %f", peak)
	}
}

func TestPeak_Mixed(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{0.5}
	frame[1] = []float32{-0.7}
	frame[2] = []float32{0.3}
	frame[3] = []float32{0.1}

	peak := Peak(frame)
	if math.Abs(peak-0.7) > 0.001 {
		t.Errorf("Peak should be 0.7, got %f", peak)
	}
}

func TestIsSilence_True(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.001, -0.001}
	}

	if !IsSilence(frame, 0.01) {
		t.Error("Near-zero frame should be silent with threshold 0.01")
	}
}

func TestIsSilence_False(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.5, -0.5}
	}

	if IsSilence(frame, 0.01) {
		t.Error("Loud frame should not be silent")
	}
}

func TestIsSilence_EmptyFrame(t *testing.T) {
	t.Parallel()

	if !IsSilence(nil, 0.01) {
		t.Error("Nil frame should be silent")
	}

	if !IsSilence([][]float32{}, 0.01) {
		t.Error("Empty frame should be silent")
	}
}

func TestRMSdB_Silence(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.0}
	}

	db := RMSdB(frame)
	if db != math.Inf(-1) {
		t.Errorf("RMSdB of silence should be -Inf, got %f", db)
	}
}

func TestRMSdB_FullScale(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	for i := range frame {
		frame[i] = []float32{1.0}
	}

	db := RMSdB(frame)
	if math.Abs(db-0.0) > 0.001 {
		t.Errorf("RMSdB of full-scale should be 0, got %f", db)
	}
}

func TestRMSdB_HalfScale(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	for i := range frame {
		frame[i] = []float32{0.5}
	}

	db := RMSdB(frame)
	expected := 20.0 * math.Log10(0.5) // ~-6.02 dB
	if math.Abs(db-expected) > 0.01 {
		t.Errorf("RMSdB of half-scale should be ~%f, got %f", expected, db)
	}
}

func TestVAD_Silence(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.001}
	}

	if VAD(frame, 0.01) {
		t.Error("Silent frame should not trigger VAD")
	}
}

func TestVAD_Voice(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.5}
	}

	if !VAD(frame, 0.01) {
		t.Error("Loud frame should trigger VAD")
	}
}

func TestConvertChannels_MonoToStereo(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 3)
	frame[0] = []float32{0.5}
	frame[1] = []float32{0.3}
	frame[2] = []float32{0.1}

	result := ConvertChannels(frame, 2)
	if len(result) != 3 {
		t.Fatalf("Expected 3 frames, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 2 {
			t.Errorf("Frame %v: expected 2 channels, got %v", i, len(result[i]))
		}
		if result[i][0] != frame[i][0] || result[i][1] != frame[i][0] {
			t.Errorf("Frame %v: expected [%f,%f], got [%f,%f]",
				i, frame[i][0], frame[i][0], result[i][0], result[i][1])
		}
	}
}

func TestConvertChannels_StereoToMono(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 2)
	frame[0] = []float32{0.6, 0.4}
	frame[1] = []float32{0.8, 0.2}

	result := ConvertChannels(frame, 1)
	if len(result) != 2 {
		t.Fatalf("Expected 2 frames, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 1 {
			t.Errorf("Frame %v: expected 1 channel, got %v", i, len(result[i]))
		}
		expected := (frame[i][0] + frame[i][1]) / 2.0
		if math.Abs(float64(result[i][0]-float32(expected))) > 0.001 {
			t.Errorf("Frame %v: expected %f, got %f", i, expected, result[i][0])
		}
	}
}

func TestConvertChannels_SameChannels(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 2)
	frame[0] = []float32{0.5, 0.3}
	frame[1] = []float32{0.7, 0.1}

	result := ConvertChannels(frame, 2)
	if len(result) != 2 {
		t.Fatalf("Expected 2 frames, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 2 {
			t.Errorf("Frame %v: expected 2 channels, got %v", i, len(result[i]))
		}
		if result[i][0] != frame[i][0] || result[i][1] != frame[i][1] {
			t.Errorf("Frame %v: same-channel conversion changed values", i)
		}
	}
}

func TestConvertChannels_Truncate(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 2)
	frame[0] = []float32{0.5, 0.3, 0.1}
	frame[1] = []float32{0.7, 0.1, 0.2}

	result := ConvertChannels(frame, 2)
	if len(result) != 2 {
		t.Fatalf("Expected 2 frames, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 2 {
			t.Errorf("Frame %v: expected 2 channels, got %v", i, len(result[i]))
		}
	}
}

func TestResample_Up(t *testing.T) {
	t.Parallel()

	// Resample from 8000 to 16000 (2x)
	frame := make([][]float32, 4)
	for i := range frame {
		frame[i] = []float32{float32(i) / 4.0}
	}

	result := Resample(frame, 8000, 16000)
	if len(result) != 8 {
		t.Errorf("Expected 8 frames after 2x upsampling, got %v", len(result))
	}
}

func TestResample_Down(t *testing.T) {
	t.Parallel()

	// Resample from 16000 to 8000 (0.5x)
	frame := make([][]float32, 8)
	for i := range frame {
		frame[i] = []float32{float32(i) / 8.0}
	}

	result := Resample(frame, 16000, 8000)
	if len(result) != 4 {
		t.Errorf("Expected 4 frames after 0.5x downsampling, got %v", len(result))
	}
}

func TestResample_SameRate(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	for i := range frame {
		frame[i] = []float32{float32(i) / 4.0, float32(i) / 4.0}
	}

	result := Resample(frame, 48000, 48000)
	if len(result) != len(frame) {
		t.Errorf("Same-rate resample should return same length: got %v, want %v",
			len(result), len(frame))
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{0.1}
	frame[1] = []float32{0.5}
	frame[2] = []float32{0.3}
	frame[3] = []float32{0.2}

	result := Normalize(frame)
	peak := Peak(result)
	if math.Abs(peak-1.0) > 0.001 {
		t.Errorf("Normalized peak should be 1.0, got %f", peak)
	}
}

func TestNormalize_Silence(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	for i := range frame {
		frame[i] = []float32{0.0}
	}

	result := Normalize(frame)
	for i := range result {
		if result[i][0] != 0.0 {
			t.Errorf("Normalizing silence should return silence, got %f", result[i][0])
		}
	}
}

func TestClipDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		frame     [][]float32
		wantClips int
	}{
		{
			name:      "no clipping",
			frame:     [][]float32{{0.5}, {0.3}, {-0.7}},
			wantClips: 0,
		},
		{
			name:      "positive clip",
			frame:     [][]float32{{1.0}, {0.5}},
			wantClips: 1,
		},
		{
			name:      "negative clip",
			frame:     [][]float32{{-1.0}, {0.5}},
			wantClips: 1,
		},
		{
			name:      "both clips",
			frame:     [][]float32{{1.0}, {-1.0}},
			wantClips: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clips := ClipCount(tc.frame)
			if clips != tc.wantClips {
				t.Errorf("ClipCount = %v, want %v", clips, tc.wantClips)
			}
		})
	}
}

func TestEnergy(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 4)
	frame[0] = []float32{1.0}
	frame[1] = []float32{-1.0}
	frame[2] = []float32{1.0}
	frame[3] = []float32{-1.0}

	energy := Energy(frame)
	if math.Abs(energy-4.0) > 0.001 {
		t.Errorf("Energy of [1,-1,1,-1] should be 4.0, got %f", energy)
	}
}

func TestZeroCrossingRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		frame    [][]float32
		expected float64
	}{
		{
			name:     "constant positive",
			frame:    [][]float32{{0.5}, {0.5}, {0.5}, {0.5}},
			expected: 0.0,
		},
		{
			name:     "alternating",
			frame:    [][]float32{{1.0}, {-1.0}, {1.0}, {-1.0}},
			expected: 1.0,
		},
		{
			name:     "single crossing",
			frame:    [][]float32{{0.5}, {0.5}, {-0.5}, {-0.5}},
			expected: 1.0 / 3.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			zcr := ZeroCrossingRate(tc.frame)
			if math.Abs(zcr-tc.expected) > 0.001 {
				t.Errorf("ZeroCrossingRate = %f, want %f", zcr, tc.expected)
			}
		})
	}
}
