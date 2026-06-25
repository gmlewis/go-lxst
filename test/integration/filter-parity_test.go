// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package integration

import (
	"math"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/filters"
)

// TestFilter_HighPass_Parity verifies the Go HighPass filter produces
// results consistent with the expected mathematical behavior.
// The Python LXST HighPass uses a simple first-order IIR filter.
func TestFilter_HighPass_Parity(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(300)

	// Test with a known input: DC component should be removed
	// and high-frequency content should pass through.
	frameSize := 480
	sampleRate := 48000

	// Generate a 1000Hz sine wave (should pass through)
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(sampleRate)))}
	}

	result := hp.HandleFrame(frame, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("Expected %d samples, got %d", frameSize, len(result))
	}

	// Result should have similar amplitude to input for 1000Hz (well above 300Hz cutoff)
	inputEnergy := float64(0)
	outputEnergy := float64(0)
	for i := range frame {
		inputEnergy += float64(frame[i][0]) * float64(frame[i][0])
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	// 1000Hz is well above 300Hz, so output should have most of the energy
	// Allow for some initial transient loss
	if outputEnergy < inputEnergy*0.3 {
		t.Errorf("HighPass attenuated 1000Hz too much: input=%f, output=%f", inputEnergy, outputEnergy)
	}
}

// TestFilter_LowPass_Parity verifies LowPass filter behavior.
func TestFilter_LowPass_Parity(t *testing.T) {
	t.Parallel()

	lp := filters.NewLowPass(3000)

	frameSize := 480
	sampleRate := 48000

	// Generate a 500Hz sine wave (should pass through)
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0 * math.Pi * 500.0 * float64(i) / float64(sampleRate)))}
	}

	result := lp.HandleFrame(frame, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("Expected %d samples, got %d", frameSize, len(result))
	}

	inputEnergy := float64(0)
	outputEnergy := float64(0)
	for i := range frame {
		inputEnergy += float64(frame[i][0]) * float64(frame[i][0])
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	// 500Hz is well below 3000Hz, so output should have most of the energy
	if outputEnergy < inputEnergy*0.3 {
		t.Errorf("LowPass attenuated 500Hz too much: input=%f, output=%f", inputEnergy, outputEnergy)
	}
}

// TestFilter_BandPass_Parity verifies BandPass filter behavior.
func TestFilter_BandPass_Parity(t *testing.T) {
	t.Parallel()

	bp := filters.NewBandPass(300, 3000)

	frameSize := 480
	sampleRate := 48000

	// Generate a 1000Hz sine wave (within band)
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(sampleRate)))}
	}

	result := bp.HandleFrame(frame, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("Expected %d samples, got %d", frameSize, len(result))
	}

	inputEnergy := float64(0)
	outputEnergy := float64(0)
	for i := range frame {
		inputEnergy += float64(frame[i][0]) * float64(frame[i][0])
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	// 1000Hz is within 300-3000Hz bandpass, should pass through
	if outputEnergy < inputEnergy*0.2 {
		t.Errorf("BandPass attenuated 1000Hz too much: input=%f, output=%f", inputEnergy, outputEnergy)
	}
}

// TestFilter_AGC_Parity verifies AGC filter behavior.
func TestFilter_AGC_Parity(t *testing.T) {
	t.Parallel()

	agc := filters.NewAGC(1.0, 30.0, 0.01, 0.1, 0.0) // target level of 1.0

	frameSize := 480
	sampleRate := 48000

	// Generate a quiet sine wave
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{0.1 * float32(math.Sin(2.0*math.Pi*1000.0*float64(i)/float64(sampleRate)))}
	}

	result := agc.HandleFrame(frame, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("Expected %d samples, got %d", frameSize, len(result))
	}

	// After processing many frames, AGC should amplify quiet signals
	// toward the target level. Process multiple frames.
	for i := 0; i < 100; i++ {
		result = agc.HandleFrame(frame, sampleRate)
	}

	// Check that output is louder than input
	maxOutput := float32(0)
	for i := range result {
		abs := float32(math.Abs(float64(result[i][0])))
		if abs > maxOutput {
			maxOutput = abs
		}
	}

	if maxOutput <= 0.1 {
		t.Errorf("AGC should amplify quiet signals, max output=%f", maxOutput)
	}
}

// TestFilter_HighPass_DC_Removal verifies that DC offset is removed.
func TestFilter_HighPass_DC_Removal(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(100)

	frameSize := 480
	sampleRate := 48000

	// Create a signal with DC offset
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{0.5 + 0.3*float32(math.Sin(2.0*math.Pi*1000.0*float64(i)/float64(sampleRate)))}
	}

	result := hp.HandleFrame(frame, sampleRate)

	// Calculate mean of output - should be close to zero (DC removed)
	mean := float64(0)
	for i := range result {
		mean += float64(result[i][0])
	}
	mean /= float64(len(result))

	if math.Abs(mean) > 0.1 {
		t.Errorf("HighPass should remove DC offset, mean=%f", mean)
	}
}
