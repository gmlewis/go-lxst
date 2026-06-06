// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package raw

import (
	"math"
	"testing"
)

func TestRaw_EncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		bitdepth int
		channels int
	}{
		{"16bit_mono", 16, 1},
		{"32bit_mono", 32, 1},
		{"64bit_mono", 64, 1},
		{"128bit_mono", 128, 1},
		{"16bit_stereo", 16, 2},
		{"32bit_stereo", 32, 2},
		{"16bit_5ch", 16, 5},
		{"16bit_32ch", 16, 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			codec, err := NewRaw(tc.channels, tc.bitdepth)
			if err != nil {
				t.Fatalf("NewRaw failed: %v", err)
			}

			// Create test frames
			frames := 3
			input := make([][]float32, frames)
			for i := 0; i < frames; i++ {
				input[i] = make([]float32, tc.channels)
				for c := 0; c < tc.channels; c++ {
					input[i][c] = float32(i*10 + c) * 0.1
				}
			}

			encoded := codec.Encode(input)
			if len(encoded) == 0 {
				t.Fatal("Encode returned empty")
			}

			// Check header byte
			expectedHeader := byte((codec.headerBD << 6) | (tc.channels - 1))
			if encoded[0] != expectedHeader {
				t.Errorf("Header byte mismatch: got 0x%02x, want 0x%02x", encoded[0], expectedHeader)
			}

			decoded := codec.Decode(encoded, tc.channels)
			if len(decoded) != frames {
				t.Errorf("Expected %d frames, got %d", frames, len(decoded))
			}

			// Verify values
			for i := 0; i < frames; i++ {
				if len(decoded[i]) != tc.channels {
					t.Errorf("Frame %d: expected %d channels, got %d", i, tc.channels, len(decoded[i]))
					continue
				}
				for c := 0; c < tc.channels; c++ {
					if decoded[i][c] != input[i][c] {
						t.Errorf("Mismatch at [%d][%d]: got %f, want %f", i, c, decoded[i][c], input[i][c])
					}
				}
			}
		})
	}
}

func TestRaw_EncodeDecode_AutoChannels(t *testing.T) {
	t.Parallel()
	codec, _ := NewRaw(0, 16) // Auto-detect channels

	input := [][]float32{
		{0.1, 0.2, 0.3}, // 3 channels
		{0.4, 0.5, 0.6},
	}

	encoded := codec.Encode(input)
	decoded := codec.Decode(encoded, 0)

	if len(decoded) != 2 {
		t.Errorf("Expected 2 frames, got %d", len(decoded))
	}
	if len(decoded[0]) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(decoded[0]))
	}
	for i := range input {
		for c := range input[i] {
			if decoded[i][c] != input[i][c] {
				t.Errorf("Mismatch at [%d][%d]: got %f, want %f", i, c, decoded[i][c], input[i][c])
			}
		}
	}
}

func TestRaw_ChannelPadding_Upmix(t *testing.T) {
	t.Parallel()
	codec, _ := NewRaw(4, 16) // 4 channels configured

	// Input has 2 channels, should be padded to 4
	input := [][]float32{
		{0.1, 0.2},
		{0.3, 0.4},
	}

	encoded := codec.Encode(input)
	decoded := codec.Decode(encoded, 4)

	if len(decoded[0]) != 4 {
		t.Errorf("Expected 4 channels, got %d", len(decoded[0]))
	}
	// Check padding: channels 2,3 should be copies of channel 1
	for i := range input {
		if decoded[i][2] != input[i][1] {
			t.Errorf("Frame %d ch 2: expected %f (copy of ch1), got %f", i, input[i][1], decoded[i][2])
		}
		if decoded[i][3] != input[i][1] {
			t.Errorf("Frame %d ch 3: expected %f (copy of ch1), got %f", i, input[i][1], decoded[i][3])
		}
	}
}

func TestRaw_ChannelPadding_Downmix(t *testing.T) {
	t.Parallel()
	codec, _ := NewRaw(2, 16) // 2 channels configured

	// Input has 4 channels, should be truncated to 2
	input := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}

	encoded := codec.Encode(input)
	decoded := codec.Decode(encoded, 2)

	if len(decoded[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(decoded[0]))
	}
	// Check truncation: first 2 channels preserved
	for i := range input {
		for c := 0; c < 2; c++ {
			if decoded[i][c] != input[i][c] {
				t.Errorf("Mismatch at [%d][%d]: got %f, want %f", i, c, decoded[i][c], input[i][c])
			}
		}
	}
}

func TestRaw_BitdepthMapping(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		bitdepth  int
		expectedD string
		expectedH int
	}{
		{16, "float16", BITDEPTH_16},
		{32, "float32", BITDEPTH_32},
		{48, "float32", BITDEPTH_32},
		{64, "float64", BITDEPTH_64},
		{96, "float64", BITDEPTH_64},
		{128, "float128", BITDEPTH_128},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedD, func(t *testing.T) {
			t.Parallel()
			codec, err := NewRaw(1, tc.bitdepth)
			if err != nil {
				t.Fatalf("NewRaw failed: %v", err)
			}
			if codec.dtype != tc.expectedD {
				t.Errorf("dtype: got %s, want %s", codec.dtype, tc.expectedD)
			}
			if codec.headerBD != tc.expectedH {
				t.Errorf("headerBD: got %d, want %d", codec.headerBD, tc.expectedH)
			}
		})
	}
}

func TestRaw_HeaderEncoding(t *testing.T) {
	t.Parallel()
	codec, _ := NewRaw(2, 32)

	// Encode a single frame
	input := [][]float32{{1.0, -1.0}}
	encoded := codec.Encode(input)

	// Header: bitdepth=BITDEPTH_32 (0x01) << 6 = 0x40, channels-1 = 1
	// 0x40 | 0x01 = 0x41
	if encoded[0] != 0x41 {
		t.Errorf("Header byte: got 0x%02x, want 0x41", encoded[0])
	}
}

func TestRaw_InvalidBitdepth(t *testing.T) {
	t.Parallel()
	_, err := NewRaw(1, 8)
	if err == nil {
		t.Error("Expected error for bitdepth < 16")
	}
	_, err = NewRaw(1, 256)
	if err == nil {
		t.Error("Expected error for bitdepth > 128")
	}
}

func TestRaw_InvalidChannels(t *testing.T) {
	t.Parallel()
	_, err := NewRaw(0, 16) // 0 is valid (auto-detect)
	if err != nil {
		t.Errorf("Unexpected error for 0 channels: %v", err)
	}
	_, err = NewRaw(-1, 16)
	if err == nil {
		t.Error("Expected error for negative channels")
	}
	_, err = NewRaw(33, 16)
	if err == nil {
		t.Error("Expected error for >32 channels")
	}
}



func TestRaw_Float16Encoding(t *testing.T) {
	t.Parallel()
	// Test that float16 values are handled (they're stored as float32 in Go)
	codec, _ := NewRaw(1, 16)
	input := [][]float32{{1.0}, {-1.0}, {0.0}, {0.5}}
	encoded := codec.Encode(input)
	decoded := codec.Decode(encoded, 1)

	for i := range input {
		// float16 has reduced precision, so allow small delta
		if math.Abs(float64(decoded[i][0]-input[i][0])) > 0.001 {
			t.Errorf("Frame %d: got %f, want %f", i, decoded[i][0], input[i][0])
		}
	}
}