// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

package main

import (
	"math"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
)

// TestEchoRoundTrip simulates the full audio path:
//
//	mic (48kHz) → encode (gornphone) → decode (echo) → delay →
//	encode (echo) → decode (gornphone) → speaker (24kHz)
//
// Without hardware, using the same Opus codec profile as the real
// gornphone/gornphone-echo (PROFILE_VOICE_MEDIUM = 24kHz mono).
// The test verifies that a sine wave survives the round trip with
// only a small margin of error, catching sample-rate mismatches,
// channel count bugs, and truncation issues.
func TestEchoRoundTrip(t *testing.T) {
	t.Parallel()

	// --- Setup: simulate the gornphone caller's transmit codec ---
	callerCodec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		t.Fatalf("NewOpus(caller) failed: %v", err)
	}
	nativeRate := callerCodec.PreferredSampleRate() // 24000
	channels := callerCodec.Channels()              // 1

	// Simulate the caller's LineSource capturing at 48kHz (the
	// backend default on macOS). The mixer should call
	// SetSourceSampleRate so the codec resamples before encoding.
	captureRate := 48000
	callerCodec.SetSourceSampleRate(captureRate)

	// --- Setup: simulate the echo's codec ---
	echoCodec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		t.Fatalf("NewOpus(echo) failed: %v", err)
	}

	// --- Generate a test sine wave at the capture rate ---
	frameMs := 60.0
	captureSamplesPerFrame := int(float64(captureRate) * frameMs / 1000.0) // 2880

	// Generate 1 second of audio (~17 frames at 60ms).
	numFrames := 17
	freq := 440.0
	phase := 0.0
	var allInput [][]float32
	for f := 0; f < numFrames; f++ {
		frame := make([][]float32, captureSamplesPerFrame)
		for i := range frame {
			sample := 0.5 * math.Sin(2.0*math.Pi*freq*phase/float64(captureRate))
			frame[i] = []float32{float32(sample)}
			phase++
		}
		allInput = append(allInput, frame...)
	}
	totalInputSamples := len(allInput)

	// --- Simulate the transmit path: encode at 48k, decode at 24k ---
	// (The Opus encoder resamples from sourceSampleRate to nativeRate.)
	var allDecodedAtEcho [][]float32
	for f := 0; f < numFrames; f++ {
		frame := allInput[f*captureSamplesPerFrame : (f+1)*captureSamplesPerFrame]
		encoded := callerCodec.Encode(frame)
		if len(encoded) == 0 {
			t.Fatalf("Encode returned empty at frame %v", f)
		}

		decoded := echoCodec.Decode(encoded, channels)
		if len(decoded) == 0 {
			t.Fatalf("Decode returned empty at frame %v", f)
		}

		// After resampling, the decoded frame should be at the
		// native rate: 60ms * 24000 = 1440 samples.
		expected := int(float64(nativeRate) * frameMs / 1000.0)
		if len(decoded) != expected {
			t.Errorf("Frame %v: decoded %v samples, expected %v (resampling not working?)",
				f, len(decoded), expected)
		}
		allDecodedAtEcho = append(allDecodedAtEcho, decoded...)
	}

	// --- Simulate the echo delay + re-encode + re-decode ---
	// The echo sends back the decoded audio. The caller decodes it.
	echoFrameSamples := int(float64(nativeRate) * frameMs / 1000.0) // 1440

	// Pad the decoded audio with silence for the delay, then feed
	// it back through the echo's encoder and the caller's decoder.
	var allOutput [][]float32

	// Simulate processing echo frames: encode each 1440-sample
	// frame with the echo codec, then decode with a fresh caller
	// decoder.
	callerDecodeCodec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		t.Fatalf("NewOpus(caller decode) failed: %v", err)
	}

	for i := 0; i < len(allDecodedAtEcho); i += echoFrameSamples {
		end := i + echoFrameSamples
		if end > len(allDecodedAtEcho) {
			end = len(allDecodedAtEcho)
		}
		frame := allDecodedAtEcho[i:end]
		if len(frame) < echoFrameSamples {
			// Pad last frame with silence.
			padded := make([][]float32, echoFrameSamples)
			copy(padded, frame)
			for j := len(frame); j < echoFrameSamples; j++ {
				padded[j] = []float32{0.0}
			}
			frame = padded
		}

		encoded := echoCodec.Encode(frame)
		if len(encoded) == 0 {
			t.Fatalf("Echo encode returned empty at frame %v", i/echoFrameSamples)
		}

		decoded := callerDecodeCodec.Decode(encoded, channels)
		if len(decoded) == 0 {
			t.Fatalf("Caller decode returned empty at frame %v", i/echoFrameSamples)
		}
		allOutput = append(allOutput, decoded...)
	}

	// The round trip introduces codec lossy compression (twice),
	// so we compare correlation rather than exact sample matching.
	// We check that the dominant frequency is still ~440Hz.

	totalOutputSamples := len(allOutput)

	// Check that we got a reasonable amount of output (the full
	// echoed audio minus codec latency). The output should contain
	// all the decoded echo frames.
	expectedOutputMin := totalInputSamples / 3 // allow for codec latency
	if totalOutputSamples < expectedOutputMin {
		t.Fatalf("Not enough output: got %v samples, need at least %v",
			totalOutputSamples, expectedOutputMin)
	}

	// Check that the echoed audio has energy (not silence or noise).
	var maxVal float32
	for _, s := range allOutput {
		v := float64(s[0])
		if v < 0 {
			v = -v
		}
		if v > float64(maxVal) {
			maxVal = float32(v)
		}
	}
	if maxVal < 0.01 {
		t.Errorf("Echoed audio is near-silent: max=%v, expected > 0.01", maxVal)
	}

	// Verify the frequency is preserved by counting zero crossings.
	// At 440Hz over 24000Hz sample rate, there should be ~880 zero
	// crossings per second, or ~0.037 per sample.
	zeroCrossings := 0
	for i := 1; i < len(allOutput); i++ {
		if (allOutput[i-1][0] >= 0) != (allOutput[i][0] >= 0) {
			zeroCrossings++
		}
	}

	analyzedSamples := len(allOutput)

	// Expected zero crossings: 2 * 440 * (analyzedSamples / 24000)
	expectedCrossings := 2.0 * freq * float64(analyzedSamples) / float64(nativeRate)
	actualCrossings := float64(zeroCrossings)

	// Allow 20% tolerance for codec lossy compression.
	ratio := actualCrossings / expectedCrossings
	if ratio < 0.8 || ratio > 1.2 {
		t.Errorf("Zero crossing count: got %v, expected ~%.0f (ratio %.2f). Audio may be garbled.",
			zeroCrossings, expectedCrossings, ratio)
	} else {
		t.Logf("Zero crossings: got %v, expected ~%.0f (ratio %.2f) — OK",
			zeroCrossings, expectedCrossings, ratio)
	}
}

// TestEchoRoundTrip_NoResampling verifies that when the source rate
// matches the codec rate (no resampling needed), the round trip is
// also clean. This catches regressions in the non-resampling path.
func TestEchoRoundTrip_NoResampling(t *testing.T) {
	t.Parallel()

	codec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		t.Fatalf("NewOpus failed: %v", err)
	}
	nativeRate := codec.PreferredSampleRate() // 24000
	channels := codec.Channels()              // 1

	// Source rate = native rate (no resampling).
	codec.SetSourceSampleRate(nativeRate)

	frameMs := 60.0
	spf := int(float64(nativeRate) * frameMs / 1000.0) // 1440
	numFrames := 10
	freq := 440.0
	phase := 0.0

	// Encode and decode each frame, check sample count is preserved.
	for f := 0; f < numFrames; f++ {
		frame := make([][]float32, spf)
		for i := range frame {
			sample := 0.5 * math.Sin(2.0*math.Pi*freq*phase/float64(nativeRate))
			frame[i] = []float32{float32(sample)}
			phase++
		}

		encoded := codec.Encode(frame)
		if len(encoded) == 0 {
			t.Fatalf("Encode returned empty at frame %v", f)
		}

		decoded := codec.Decode(encoded, channels)
		if len(decoded) == 0 {
			t.Fatalf("Decode returned empty at frame %v", f)
		}

		if len(decoded) != spf {
			t.Errorf("Frame %v: decoded %v samples, expected %v", f, len(decoded), spf)
		}
	}
}

// TestEchoRoundTrip_WithResampling verifies that resampling from 48k
// to 24k produces the correct number of output samples.
func TestEchoRoundTrip_WithResampling(t *testing.T) {
	t.Parallel()

	codec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		t.Fatalf("NewOpus failed: %v", err)
	}
	nativeRate := codec.PreferredSampleRate() // 24000
	channels := codec.Channels()              // 1

	// Set source rate to 48kHz — the encoder should resample to 24k.
	codec.SetSourceSampleRate(48000)

	frameMs := 60.0
	captureSPF := int(float64(48000) * frameMs / 1000.0)     // 2880
	nativeSPF := int(float64(nativeRate) * frameMs / 1000.0) // 1440

	freq := 440.0
	phase := 0.0

	for f := 0; f < 5; f++ {
		frame := make([][]float32, captureSPF)
		for i := range frame {
			sample := 0.5 * math.Sin(2.0*math.Pi*freq*phase/48000.0)
			frame[i] = []float32{float32(sample)}
			phase++
		}

		encoded := codec.Encode(frame)
		if len(encoded) == 0 {
			t.Fatalf("Encode returned empty at frame %v", f)
		}

		decoded := codec.Decode(encoded, channels)
		if len(decoded) == 0 {
			t.Fatalf("Decode returned empty at frame %v", f)
		}

		// After resampling 48k→24k, the decoded frame should be
		// 1440 samples (60ms at 24kHz), NOT 2880.
		if len(decoded) != nativeSPF {
			t.Errorf("Frame %v: decoded %v samples, expected %v (resampling 48k→24k failed)",
				f, len(decoded), nativeSPF)
		}
	}
}
