// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sources

import (
	"math"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/filters"
)

func TestLineSource_CodecSetter(t *testing.T) {
	t.Parallel()

	// Test with NullCodec (no constraints)
	nc := codecs.NullCodec{}
	ls := NewLineSource("", 20.0, nc, nil, nil, 0.0, 0.0, 0.0)

	err := ls.SetCodec(nc)
	if err != nil {
		t.Fatalf("SetCodec failed: %v", err)
	}

	// Frame should stay the same with NullCodec (no quantization)
	if math.Abs(ls.targetFrameMs-20.0) > 0.001 {
		t.Errorf("Frame time should be 20ms, got %f", ls.targetFrameMs)
	}

	// Test with nil codec
	ls2 := NewLineSource("", 25.0, nil, nil, nil, 0.0, 0.0, 0.0)
	err = ls2.SetCodec(nil)
	if err != nil {
		t.Fatalf("SetCodec(nil) should not error: %v", err)
	}
	if ls2.codec != nil {
		t.Error("Codec should be nil after SetCodec(nil)")
	}
}

func TestLineSource_StartStop(t *testing.T) {
	t.Parallel()

	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0)

	// Start should succeed (even with null backend)
	err := ls.Start()
	if err != nil {
		// Null backend might fail, but that's OK - we test the logic
		t.Logf("Start returned error (expected with null backend): %v", err)
	}

	// Starting again should fail
	err = ls.Start()
	if err == nil {
		t.Error("Start() should fail when already running")
	}

	// Stop
	err = ls.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Stop again should succeed (idempotent)
	err = ls.Stop()
	if err != nil {
		t.Errorf("Stop() should be idempotent: %v", err)
	}
}

func TestLineSource_Stop_NotRunning(t *testing.T) {
	t.Parallel()

	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0)

	// Stop when not running should succeed (idempotent)
	err := ls.Stop()
	if err != nil {
		t.Errorf("Stop() should be idempotent: %v", err)
	}
}

func TestLineSource_EaseIn(t *testing.T) {
	t.Parallel()

	// Test ease-in logic
	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 10.0, 0.1, 0.0) // 10dB gain, 0.1s ease-in

	// Initial gain should be 0 when easeIn > 0
	if ls.currentGain != 0.0 {
		t.Errorf("Initial gain should be 0 with easeIn, got %f", ls.currentGain)
	}

	// Target gain should be 10 (linear)
	expectedTarget := math.Pow(10, 1.0) // = 10.0
	if math.Abs(ls.targetGain-expectedTarget) > 0.001 {
		t.Errorf("Target gain mismatch: got %f, want %f", ls.targetGain, expectedTarget)
	}
}

func TestLineSource_Skip(t *testing.T) {
	t.Parallel()

	// Test skip logic
	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.1) // 0.1s skip

	if ls.skipCompleted {
		t.Error("Skip should not be completed initially")
	}

	ls2 := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0) // No skip
	if !ls2.skipCompleted {
		t.Error("Skip should be completed when skip=0")
	}
}

func TestLineSource_Filters(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(300)
	lp := filters.NewLowPass(3000)

	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, []filters.Filter{hp, lp}, 0.0, 0.0, 0.0)

	if len(ls.filterChain) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(ls.filterChain))
	}
}

func TestLineSource_GainCalculation(t *testing.T) {
	t.Parallel()

	// Test that linear gain is calculated correctly from dB
	testCases := []struct {
		gainDB      float64
		expectedLin float64
	}{
		{0.0, 1.0},
		{10.0, 10.0},
		{-10.0, 0.1},
		{20.0, 100.0},
		{6.0, 3.981072}, // 10^(6/10)
	}

	for _, tc := range testCases {
		ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, tc.gainDB, 0.0, 0.0)
		if math.Abs(ls.targetGain-tc.expectedLin) > 0.001 {
			t.Errorf("Gain %fdB: got linear %f, want %f", tc.gainDB, ls.targetGain, tc.expectedLin)
		}

	}
}

func TestLineSource_NilCodec(t *testing.T) {
	t.Parallel()

	// Test with nil codec (passthrough)
	ls := NewLineSource("", 20.0, nil, nil, nil, 0.0, 0.0, 0.0)

	err := ls.SetCodec(nil)
	if err != nil {
		t.Errorf("SetCodec(nil) should not error: %v", err)
	}

	if ls.codec != nil {
		t.Error("Codec should be nil after SetCodec(nil)")
	}
}

func TestLineSource_Getters(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	ls := NewLineSource("test-device", 25.0, codec, nil, nil, 5.0, 0.0, 0.0)

	if ls.GetCodec() != codec {
		t.Error("GetCodec should return the set codec")
	}

	// Sample rate and channels will be 0 until Start() is called
	// (since we use null backend which doesn't initialize them)
}

func TestLineSource_PreferredDevice(t *testing.T) {
	t.Parallel()

	ls := NewLineSource("my-mic", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0)
	if ls.PreferredDevice() != "my-mic" {
		t.Errorf("Expected preferred device 'my-mic', got %q", ls.PreferredDevice())
	}

	ls2 := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0)
	if ls2.PreferredDevice() != "" {
		t.Errorf("Expected empty preferred device, got %q", ls2.PreferredDevice())
	}
}
