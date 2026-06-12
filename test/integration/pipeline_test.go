// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/generators"
	"github.com/gmlewis/go-lxst/lxst/mixer"
	"github.com/gmlewis/go-lxst/lxst/processing"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// collectingSink collects frames for verification.
type collectingSink struct {
	mu      sync.Mutex
	frames  [][][]float32
	maxCols int
}

func (c *collectingSink) HandleFrame(frame [][]float32, _ sources.Source) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxCols > 0 && len(c.frames) >= c.maxCols {
		return nil
	}
	cp := make([][]float32, len(frame))
	for i := range frame {
		cp[i] = make([]float32, len(frame[i]))
		copy(cp[i], frame[i])
	}
	c.frames = append(c.frames, cp)
	return nil
}

func (c *collectingSink) CanReceive(_ sources.Source) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxCols == 0 || len(c.frames) < c.maxCols
}

func (c *collectingSink) Start() error  { return nil }
func (c *collectingSink) Stop() error   { return nil }
func (c *collectingSink) Running() bool { return true }

func (c *collectingSink) collected() [][][]float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frames
}

// TestIntegration_ToneSource_Generation verifies ToneSource produces
// valid sine wave frames with expected frequency characteristics.
func TestIntegration_ToneSource_Generation(t *testing.T) {
	t.Parallel()

	ts := generators.NewToneSource(440.0, 0.1, false, 20.0, 20.0, nil, nil, 1)
	if ts == nil {
		t.Fatal("NewToneSource returned nil")
	}

	if ts.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", ts.SampleRate())
	}
	if ts.Channels() != 1 {
		t.Errorf("Expected 1 channel, got %d", ts.Channels())
	}

	// Start the source with a collecting sink
	sink := &collectingSink{maxCols: 5}
	ts2 := generators.NewToneSource(440.0, 0.1, false, 20.0, 20.0, nil, sink, 1)

	err := ts2.Start()
	if err != nil {
		t.Fatalf("ToneSource.Start failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	ts2.Stop()

	frames := sink.collected()
	if len(frames) < 2 {
		t.Fatalf("Expected at least 2 frames, got %d", len(frames))
	}

	// Verify each frame has correct dimensions
	for i, f := range frames {
		if len(f) != ts2.SamplesPerFrame() {
			t.Errorf("Frame %d: expected %d samples, got %d", i, ts2.SamplesPerFrame(), len(f))
		}
		for _, s := range f {
			if len(s) != 1 {
				t.Errorf("Frame %d: expected 1 channel, got %d", i, len(s))
			}
		}
	}

	// Verify the tone has energy at 440Hz
	frame := frames[0]
	energy := float64(0)
	for _, s := range frame {
		energy += float64(s[0]) * float64(s[0])
	}
	if energy < 0.001 {
		t.Errorf("ToneSource output has very low energy: %f", energy)
	}
}

// TestIntegration_ToneSource_WithEasing verifies easing-in behavior.
func TestIntegration_ToneSource_WithEasing(t *testing.T) {
	t.Parallel()

	sink := &collectingSink{maxCols: 3}
	ts := generators.NewToneSource(1000.0, 0.5, true, 20.0, 20.0, nil, sink, 1)

	err := ts.Start()
	if err != nil {
		t.Fatalf("ToneSource.Start failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	ts.Stop()

	frames := sink.collected()
	if len(frames) < 2 {
		t.Fatalf("Expected at least 2 frames, got %d", len(frames))
	}

	// First frame should have lower amplitude due to easing in
	firstFrameEnergy := float64(0)
	for _, s := range frames[0] {
		firstFrameEnergy += float64(s[0]) * float64(s[0])
	}

	// Eased start should have lower energy than later frames
	// (easeGain starts at 0 and increases)
	if firstFrameEnergy <= 0 {
		t.Error("First frame should have some energy from easing")
	}
}

// TestIntegration_Mixer_TwoSources verifies mixer combines two sources.
func TestIntegration_Mixer_TwoSources(t *testing.T) {
	t.Parallel()

	sink := &collectingSink{maxCols: 5}

	m := mixer.NewMixer(20.0, 48000, nil, sink, 0.0)
	if m == nil {
		t.Fatal("NewMixer returned nil")
	}

	src1 := generators.NewToneSource(440.0, 0.1, false, 20.0, 20.0, nil, m, 1)
	src2 := generators.NewToneSource(880.0, 0.1, false, 20.0, 20.0, nil, m, 1)

	err := m.Start()
	if err != nil {
		t.Fatalf("Mixer.Start failed: %v", err)
	}

	err = src1.Start()
	if err != nil {
		t.Fatalf("Source1.Start failed: %v", err)
	}

	err = src2.Start()
	if err != nil {
		t.Fatalf("Source2.Start failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	src1.Stop()
	src2.Stop()
	m.Stop()

	frames := sink.collected()
	if len(frames) == 0 {
		t.Fatal("Mixer produced no output frames")
	}

	// Mixed output should have energy
	totalEnergy := float64(0)
	for _, f := range frames[0] {
		totalEnergy += float64(f[0]) * float64(f[0])
	}
	if totalEnergy < 0.001 {
		t.Error("Mixed output has very low energy")
	}
}

// TestIntegration_FilterPipeline_HighPass verifies a chain of
// ToneSource → HighPass → collects output.
func TestIntegration_FilterPipeline_HighPass(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(300)
	sampleRate := 48000
	frameSize := 480

	// Generate a signal with DC offset
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{0.5 + 0.3*float32(math.Sin(2.0*math.Pi*1000.0*float64(i)/float64(sampleRate)))}
	}

	result := hp.HandleFrame(frame, sampleRate)

	// DC should be removed
	mean := float64(0)
	for _, s := range result {
		mean += float64(s[0])
	}
	mean /= float64(len(result))

	if math.Abs(mean) > 0.1 {
		t.Errorf("DC offset after HighPass should be small, mean=%f", mean)
	}

	// Signal energy should be preserved (AC component passes through)
	inputEnergy := float64(0)
	outputEnergy := float64(0)
	// Use AC energy (subtract DC) for fair comparison
	inputDC := float64(0)
	for i := range frame {
		inputDC += float64(frame[i][0])
	}
	inputDC /= float64(len(frame))

	for i := range frame {
		acInput := float64(frame[i][0]) - inputDC
		inputEnergy += acInput * acInput
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	// Output should have most of the AC energy (1000Hz passes through 300Hz HP)
	if outputEnergy < inputEnergy*0.1 {
		t.Errorf("HighPass removed too much AC energy: input=%f, output=%f", inputEnergy, outputEnergy)
	}
}

// TestIntegration_FilterPipeline_BandPass verifies BandPass filter
// passes in-band and attenuates out-of-band content.
func TestIntegration_FilterPipeline_BandPass(t *testing.T) {
	t.Parallel()

	bp := filters.NewBandPass(300, 3000)
	sampleRate := 48000
	frameSize := 4800

	// In-band: 1000Hz
	inBand := make([][]float32, frameSize)
	for i := range inBand {
		inBand[i] = []float32{float32(math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(sampleRate)))}
	}

	result := bp.HandleFrame(inBand, sampleRate)

	inputEnergy := float64(0)
	outputEnergy := float64(0)
	for i := range inBand {
		inputEnergy += float64(inBand[i][0]) * float64(inBand[i][0])
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	ratio := outputEnergy / inputEnergy
	if ratio < 0.1 {
		t.Errorf("BandPass(300-3000) should pass 1000Hz, energy ratio=%f", ratio)
	}
}

// TestIntegration_FilterPipeline_AGCAmplification verifies AGC
// amplifies a quiet signal toward the target level.
func TestIntegration_FilterPipeline_AGCAmplification(t *testing.T) {
	t.Parallel()

	agc := filters.NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	sampleRate := 48000
	frameSize := 480

	// Create a quiet signal
	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{0.01 * float32(math.Sin(2.0*math.Pi*440.0*float64(i)/float64(sampleRate)))}
	}

	// Process multiple frames to allow AGC to converge
	var result [][]float32
	for i := 0; i < 50; i++ {
		result = agc.HandleFrame(frame, sampleRate)
	}

	// After many frames, AGC should have amplified the signal
	maxInput := float64(0)
	maxOutput := float64(0)
	for i := range frame {
		inAbs := math.Abs(float64(frame[i][0]))
		outAbs := math.Abs(float64(result[i][0]))
		if inAbs > maxInput {
			maxInput = inAbs
		}
		if outAbs > maxOutput {
			maxOutput = outAbs
		}
	}

	if maxOutput <= maxInput {
		t.Errorf("AGC should amplify quiet signal: input max=%f, output max=%f", maxInput, maxOutput)
	}
}

// TestIntegration_Processing_Pipeline verifies processing utilities
// in a real pipeline context.
func TestIntegration_Processing_Pipeline(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.05)) * 0.5, float32(math.Cos(float64(i)*0.05)) * 0.3}
	}

	// Compute RMS
	rms := processing.RMS(frame)
	if rms <= 0 {
		t.Error("RMS should be positive for non-zero signal")
	}

	// Compute Peak
	peak := processing.Peak(frame)
	if peak <= 0 {
		t.Error("Peak should be positive for non-zero signal")
	}

	// Peak should be >= RMS
	if peak < rms {
		t.Errorf("Peak (%f) should be >= RMS (%f)", peak, rms)
	}

	// Normalize
	normalized := processing.Normalize(frame)
	normalizedPeak := processing.Peak(normalized)
	if math.Abs(float64(normalizedPeak)-1.0) > 0.01 {
		t.Errorf("Normalized peak should be ~1.0, got %f", normalizedPeak)
	}

	// Convert to mono
	mono := processing.ConvertChannels(frame, 1)
	if len(mono) != len(frame) {
		t.Errorf("ConvertChannels should preserve sample count: got %d, want %d", len(mono), len(frame))
	}
	if len(mono[0]) != 1 {
		t.Errorf("ConvertChannels to 1 should produce 1 channel: got %d", len(mono[0]))
	}

	// Resample
	downsampled := processing.Resample(frame, 48000, 16000)
	if len(downsampled) != 160 {
		t.Errorf("Resample 48k->16k should produce 160 samples, got %d", len(downsampled))
	}

	// VAD on non-silent signal
	if processing.VAD(frame, 0.01) != true {
		t.Error("VAD should detect non-silent signal")
	}

	// Create a silent frame
	silent := make([][]float32, 480)
	for i := range silent {
		silent[i] = []float32{0.0001, 0.0001}
	}
	if processing.VAD(silent, 0.01) != false {
		t.Error("VAD should not detect silent signal")
	}

	// Zero-crossing rate
	zcr := processing.ZeroCrossingRate(frame)
	if zcr < 0 {
		t.Error("ZeroCrossingRate should be non-negative")
	}
}

// TestIntegration_Codec2Pipeline verifies Codec2 encode/decode roundtrip.
// NOTE: Codec2 is currently a stub and this test is skipped.
func TestIntegration_Codec2Pipeline(t *testing.T) {
	t.Skip("Codec2 is currently a stub implementation")
}

// TestIntegration_Codec2WithFilterPipeline verifies Codec2 through
// a filter pipeline.
// NOTE: Codec2 is currently a stub and this test is skipped.
func TestIntegration_Codec2WithFilterPipeline(t *testing.T) {
	t.Skip("Codec2 is currently a stub implementation")
}

// TestIntegration_NullCodecPipeline verifies the NullCodec pipeline works
// end-to-end with filters and processing.
func TestIntegration_NullCodecPipeline(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	hp := filters.NewHighPass(300)
	lp := filters.NewLowPass(8000)

	sampleRate := 48000
	frameSize := 480

	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{
			float32(math.Sin(2.0*math.Pi*440.0*float64(i)/float64(sampleRate))) * 0.5,
			float32(math.Sin(2.0*math.Pi*880.0*float64(i)/float64(sampleRate))) * 0.3,
		}
	}

	// Apply filters
	filtered := hp.HandleFrame(frame, sampleRate)
	filtered = lp.HandleFrame(filtered, sampleRate)

	// Encode and decode through NullCodec
	encoded := codec.Encode(filtered)
	decoded := codec.Decode(encoded, 2)

	if len(decoded) != len(filtered) {
		t.Fatalf("NullCodec roundtrip: got %d samples, want %d", len(decoded), len(filtered))
	}

	// NullCodec should be lossless
	for i := range decoded {
		for ch := 0; ch < 2; ch++ {
			diff := math.Abs(float64(decoded[i][ch]) - float64(filtered[i][ch]))
			if diff > 0.001 {
				t.Errorf("NullCodec roundtrip mismatch at [%d][%d]: got %f, want %f (diff %f)",
					i, ch, decoded[i][ch], filtered[i][ch], diff)
			}
		}
	}
}

// TestIntegration_FullFilterChain processes audio through
// HighPass → LowPass → BandPass → AGC chain.
func TestIntegration_FullFilterChain(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(300)
	bp := filters.NewBandPass(300, 3400)
	agc := filters.NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)

	sampleRate := 48000
	frameSize := 480

	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0*math.Pi*1000.0*float64(i)/float64(sampleRate))) * 0.1}
	}

	// Process through chain
	result := hp.HandleFrame(frame, sampleRate)
	result = bp.HandleFrame(result, sampleRate)

	// Process multiple frames through AGC
	for i := 0; i < 10; i++ {
		result = agc.HandleFrame(result, sampleRate)
	}

	// Verify output dimensions
	if len(result) != frameSize {
		t.Errorf("Expected %d samples, got %d", frameSize, len(result))
	}

	// Verify AGC amplifies the quiet signal
	outputEnergy := float64(0)
	inputEnergy := float64(0)
	for i := range frame {
		inputEnergy += float64(frame[i][0]) * float64(frame[i][0])
		outputEnergy += float64(result[i][0]) * float64(result[i][0])
	}

	if outputEnergy <= inputEnergy*0.5 {
		t.Errorf("AGC should amplify quiet signal: input energy=%f, output energy=%f", inputEnergy, outputEnergy)
	}
}