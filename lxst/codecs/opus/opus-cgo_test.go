// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

package opus

import (
	"math"
	"testing"
)

func TestOpus_EncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	o, err := NewOpus(PROFILE_VOICE_LOW)
	if err != nil {
		t.Fatalf("NewOpus failed: %v", err)
	}

	samplesPerFrame := int(float64(o.sourceSampleRate) * 20.0 / 1000.0)
	input := make([][]float32, samplesPerFrame)
	for i := range input {
		input[i] = make([]float32, o.channels)
		for c := 0; c < o.channels; c++ {
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

	if len(decoded) < samplesPerFrame/2 {
		t.Errorf("Decoded too few frames: got %v, expected ~%v", len(decoded), samplesPerFrame)
	}
}
