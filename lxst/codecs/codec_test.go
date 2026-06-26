// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package codecs

import "testing"

func TestNullCodec_EncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	codec := NullCodec{}

	// Test data: 2 samples, 2 channels
	input := [][]float32{
		{0.5, -0.3},
		{-0.7, 0.9},
	}

	encoded := codec.Encode(input)
	if len(encoded) != 16 { // 2 samples * 2 channels * 4 bytes
		t.Errorf("Expected 16 bytes, got %v", len(encoded))
	}

	decoded := codec.Decode(encoded, 2)
	if len(decoded) != 2 {
		t.Errorf("Expected 2 samples, got %v", len(decoded))
	}
	if len(decoded[0]) != 2 {
		t.Errorf("Expected 2 channels, got %v", len(decoded[0]))
	}

	// Check values match (within float32 precision)
	for i := range input {
		for c := range input[i] {
			if decoded[i][c] != input[i][c] {
				t.Errorf("Mismatch at [%v][%v]: got %f, want %f", i, c, decoded[i][c], input[i][c])
			}
		}
	}
}

func TestNullCodec_EmptyFrame(t *testing.T) {
	t.Parallel()
	codec := NullCodec{}

	encoded := codec.Encode([][]float32{})
	if len(encoded) != 0 {
		t.Errorf("Expected empty bytes, got %v", len(encoded))
	}

	decoded := codec.Decode([]byte{}, 1)
	if len(decoded) != 0 {
		t.Errorf("Expected empty frames, got %v", len(decoded))
	}
}

func TestNullCodec_SingleChannel(t *testing.T) {
	t.Parallel()
	codec := NullCodec{}

	input := [][]float32{
		{0.1},
		{0.2},
		{0.3},
	}

	encoded := codec.Encode(input)
	decoded := codec.Decode(encoded, 1)

	if len(decoded) != 3 {
		t.Errorf("Expected 3 samples, got %v", len(decoded))
	}
	for i := range input {
		if decoded[i][0] != input[i][0] {
			t.Errorf("Mismatch at [%v]: got %f, want %f", i, decoded[i][0], input[i][0])
		}
	}
}

func TestResampleBytes_SameRate(t *testing.T) {
	t.Parallel()
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	result := ResampleBytes(data, 16, 2, 48000, 48000, false)
	if string(result) != string(data) {
		t.Error("Expected same bytes when sample rates match")
	}
}

func TestResample_SameRate(t *testing.T) {
	t.Parallel()
	input := [][]float32{{0.1, 0.2}, {0.3, 0.4}}
	result := Resample(input, 16, 2, 48000, 48000, false)
	if len(result) != 2 || result[0][0] != 0.1 {
		t.Error("Expected same samples when sample rates match")
	}
}

func TestCodecError_Exists(t *testing.T) {
	t.Parallel()
	if CodecError == nil {
		t.Error("CodecError should not be nil")
	}
	err := CodecError
	if err.Error() != "codec error" {
		t.Errorf("Expected 'codec error', got '%s'", err.Error())
	}
}

func TestIsNullCodec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		codec Codec
		want  bool
	}{
		{"nil", nil, false},
		{"NullCodec value", NullCodec{}, true},
		{"NullCodec pointer", &NullCodec{}, true},
		{"NullCodecBuffered pointer", &NullCodecBuffered{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNullCodec(tc.codec); got != tc.want {
				t.Errorf("IsNullCodec(%v) = %v, want %v", tc.codec, got, tc.want)
			}
		})
	}
}

func TestNullCodec_Channels(t *testing.T) {
	t.Parallel()

	// NullCodec.Channels() returns 0 to indicate "unknown", matching
	// the Python Null codec which has no channels attribute.
	if got := (NullCodec{}).Channels(); got != 0 {
		t.Errorf("NullCodec.Channels() = %v, want 0", got)
	}
	if got := (&NullCodecBuffered{}).Channels(); got != 0 {
		t.Errorf("NullCodecBuffered.Channels() = %v, want 0", got)
	}
}
