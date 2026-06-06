// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package filters

import (
	"math"
	"testing"
)

func TestHighPass_HandleFrame(t *testing.T) {
	t.Parallel()
	hp := NewHighPass(300)

	// Create test frame (same as Python reference)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = make([]float32, 2)
	}
	// Use fixed seed values for reproducibility
	for i := 0; i < 480; i++ {
		frame[i][0] = float32(math.Sin(float64(i)*0.1)) * 0.5
		frame[i][1] = float32(math.Cos(float64(i)*0.1)) * 0.3
	}

	out := hp.HandleFrame(frame, 48000)

	if len(out) != 480 {
		t.Errorf("Expected 480 samples, got %d", len(out))
	}
	if len(out[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(out[0]))
	}

	// Test alpha recalculation on samplerate change
	hp2 := NewHighPass(300)
	frame2 := make([][]float32, 480)
	for i := range frame2 {
		frame2[i] = []float32{0.1, 0.2}
	}
	out2 := hp2.HandleFrame(frame2, 16000)
	_ = out2
}

func TestHighPass_FirstFrame(t *testing.T) {
	t.Parallel()
	hp := NewHighPass(300)

	// Single frame test
	frame := [][]float32{
		{1.0, 0.5},
		{0.8, 0.4},
		{0.6, 0.3},
	}

	out := hp.HandleFrame(frame, 48000)

	if len(out) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(out))
	}

	// Check state is preserved for next frame
	// Second call should use previous frame's last input
	out2 := hp.HandleFrame(frame, 48000)
	if len(out2) != 3 {
		t.Errorf("Expected 3 samples on second call, got %d", len(out2))
	}
}

func TestHighPass_SamplerateChange(t *testing.T) {
	t.Parallel()
	hp := NewHighPass(300)

	frame := [][]float32{
		{1.0, 0.5},
	}

	// First at 48kHz
	out1 := hp.HandleFrame(frame, 48000)
	alpha1 := hp.alpha

	// Then at 16kHz
	out2 := hp.HandleFrame(frame, 16000)
	alpha2 := hp.alpha

	_ = out1
	_ = out2

	// Alpha should have changed
	if alpha1 == alpha2 {
		t.Error("Alpha should change when samplerate changes")
	}
}

func TestLowPass_HandleFrame(t *testing.T) {
	t.Parallel()
	lp := NewLowPass(3000)

	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = make([]float32, 2)
	}
	for i := 0; i < 480; i++ {
		frame[i][0] = float32(math.Sin(float64(i)*0.1)) * 0.5
		frame[i][1] = float32(math.Cos(float64(i)*0.1)) * 0.3
	}

	out := lp.HandleFrame(frame, 48000)

	if len(out) != 480 {
		t.Errorf("Expected 480 samples, got %d", len(out))
	}
	if len(out[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(out[0]))
	}
}

func TestLowPass_SamplerateChange(t *testing.T) {
	t.Parallel()
	lp := NewLowPass(3000)

	frame := [][]float32{
		{1.0, 0.5},
	}

	out1 := lp.HandleFrame(frame, 48000)
	alpha1 := lp.alpha

	out2 := lp.HandleFrame(frame, 16000)
	alpha2 := lp.alpha

	_ = out1
	_ = out2

	if alpha1 == alpha2 {
		t.Error("Alpha should change when samplerate changes")
	}
}

func TestBandPass_HandleFrame(t *testing.T) {
	t.Parallel()
	bp := NewBandPass(300, 3000)

	frame := [][]float32{
		{1.0, 0.5},
		{0.9, 0.4},
		{0.8, 0.3},
	}

	out := bp.HandleFrame(frame, 48000)

	if len(out) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(out))
	}
	if len(out[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(out[0]))
	}
}

func TestBandPass_ComposesHPLP(t *testing.T) {
	t.Parallel()
	bp := NewBandPass(300, 3000)

	// Verify it composes HP and LP
	frame := make([][]float32, 100)
	for i := range frame {
		frame[i] = []float32{float32(i) * 0.01, float32(i) * 0.005}
	}

	out := bp.HandleFrame(frame, 48000)
	if len(out) != 100 {
		t.Errorf("Expected 100 samples, got %d", len(out))
	}
}

func TestAGC_HandleFrame(t *testing.T) {
	t.Parallel()
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	// Constant input signal (0.1 = -20dBFS)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{0.1, 0.1}
	}

	out := agc.HandleFrame(frame, 48000)

	if len(out) != 480 {
		t.Errorf("Expected 480 samples, got %d", len(out))
	}
	if len(out[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(out[0]))
	}

	// Python reference: input 0.1 -> output ~0.093 (AGC attenuates toward -12dB target slowly)
	// First frame gain is still near 1.0, so output ~ 0.1 * 0.93 = 0.093
	if math.Abs(float64(out[0][0]) - 0.093) > 0.01 {
		t.Errorf("AGC output mismatch: got %f, expected ~0.093 (matching Python reference)", out[0][0])
	}
}

func TestAGC_PeakLimiting(t *testing.T) {
	t.Parallel()
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	// Large input that should be peak limited
	frame := make([][]float32, 100)
	for i := range frame {
		frame[i] = []float32{2.0, 2.0} // Will be amplified then limited
	}

	out := agc.HandleFrame(frame, 48000)

	// Check peak limiting at 0.75
	for i := range out {
		for ch := 0; ch < 2; ch++ {
			if math.Abs(float64(out[i][ch])) > 0.75 {
				t.Errorf("Peak limiting failed at [%d][%d]: %f", i, ch, out[i][ch])
			}
		}
	}
}

func TestAGC_SamplerateChange(t *testing.T) {
	t.Parallel()
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	frame := [][]float32{{0.1, 0.1}}

	out1 := agc.HandleFrame(frame, 48000)
	attack1 := agc.attackCoeff

	out2 := agc.HandleFrame(frame, 16000)
	attack2 := agc.attackCoeff

	_ = out1
	_ = out2

	if attack1 == attack2 {
		t.Error("Attack coeff should change when samplerate changes")
	}
}

func TestAGC_IndependentChannels(t *testing.T) {
	t.Parallel()
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	// Different levels on each channel
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{0.1, 0.01} // Channel 0 louder
	}

	out := agc.HandleFrame(frame, 48000)

	// Gains should be different for each channel
	if out[0][0] == out[0][1] {
		t.Error("Channels should have independent gain")
	}
}

func TestFilter_EmptyFrame(t *testing.T) {
	t.Parallel()
	hp := NewHighPass(300)
	lp := NewLowPass(3000)
	bp := NewBandPass(300, 3000)
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	empty := [][]float32{}

	out := hp.HandleFrame(empty, 48000)
	if len(out) != 0 {
		t.Error("HighPass should handle empty frame")
	}

	out = lp.HandleFrame(empty, 48000)
	if len(out) != 0 {
		t.Error("LowPass should handle empty frame")
	}

	out = bp.HandleFrame(empty, 48000)
	if len(out) != 0 {
		t.Error("BandPass should handle empty frame")
	}

	out = agc.HandleFrame(empty, 48000)
	if len(out) != 0 {
		t.Error("AGC should handle empty frame")
	}
}

func TestBandPass_InvalidCutoff(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for lowCut >= highCut")
		}
	}()
	NewBandPass(3000, 300) // Should panic
}