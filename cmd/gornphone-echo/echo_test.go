// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"
)

func absFloat(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

func TestEchoSource_StartStop(t *testing.T) {
	t.Parallel()

	es := NewEchoSource(440.0, 0.15, 100*time.Millisecond, 60.0, nil)

	if es.Running() {
		t.Error("should not be running before Start")
	}

	if err := es.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !es.Running() {
		t.Error("should be running after Start")
	}

	// Starting twice should be safe.
	if err := es.Start(); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	if err := es.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if es.Running() {
		t.Error("should not be running after Stop")
	}
}

func TestEchoSource_GenerateToneFrame(t *testing.T) {
	t.Parallel()

	es := NewEchoSource(440.0, 0.5, 0, 60.0, nil)
	es.channels = 1
	es.sampleRate = 48000

	frame := es.generateToneFrame(48000 * 60 / 1000) // 60ms at 48kHz

	// frame is [samples][channels]
	if len(frame) != 2880 {
		t.Fatalf("expected 2880 samples, got %d", len(frame))
	}
	if len(frame[0]) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(frame[0]))
	}

	// Check that the tone is non-zero (sine wave).
	nonZero := false
	for i := range frame {
		if frame[i][0] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("tone frame should have non-zero samples")
	}

	// Check amplitude is bounded by gain.
	for i := range frame {
		if frame[i][0] > 0.51 || frame[i][0] < -0.51 {
			t.Errorf("sample %v exceeds gain 0.5", frame[i][0])
			break
		}
	}
}

func TestEchoSource_HandleFrame_BuffersForDelay(t *testing.T) {
	t.Parallel()

	es := NewEchoSource(440.0, 0.15, 200*time.Millisecond, 60.0, nil)
	es.channels = 1
	es.sampleRate = 48000

	// Mark as running so HandleFrame stores the frame.
	_ = es.Start()

	// Create a test frame [samples][channels].
	frame := make([][]float32, 100)
	for i := range frame {
		frame[i] = make([]float32, 1)
		frame[i][0] = 0.5
	}

	// Feed it to the echo source.
	if err := es.HandleFrame(frame, nil); err != nil {
		t.Fatalf("HandleFrame failed: %v", err)
	}

	// Stop the generate loop so it doesn't consume the echo frame.
	_ = es.Stop()

	// Immediately, no echo frames should be ready (delay=200ms).
	echoFrames := getReadyEchoFramesDirect(es)
	if len(echoFrames) > 0 {
		t.Errorf("expected no ready echo frames immediately, got %d", len(echoFrames))
	}

	// Wait for the delay to pass.
	time.Sleep(250 * time.Millisecond)

	// Now the echo frame should be ready.
	echoFrames = getReadyEchoFramesDirect(es)
	// frame was [100 samples][1 channel], so spreading gives 100 []float32 slices
	if len(echoFrames) != 100 {
		t.Fatalf("expected 100 ready echo frame slices, got %d", len(echoFrames))
	}
	if len(echoFrames[0]) != 1 {
		t.Errorf("expected 1 channel per sample, got %d", len(echoFrames[0]))
	}
}

// getReadyEchoFramesDirect accesses the echo buffer directly without
// interference from the generate loop.
func getReadyEchoFramesDirect(es *EchoSource) [][]float32 {
	now := time.Now()
	var result [][]float32

	es.echoBufferMu.Lock()
	remaining := es.echoBuffer[:0]
	for _, tf := range es.echoBuffer {
		if !tf.emitTime.After(now) {
			result = append(result, tf.frame...)
		} else {
			remaining = append(remaining, tf)
		}
	}
	es.echoBuffer = remaining
	es.echoBufferMu.Unlock()

	return result
}

func TestEchoSource_CanReceive(t *testing.T) {
	t.Parallel()

	es := NewEchoSource(440.0, 0.15, 100*time.Millisecond, 60.0, nil)

	if !es.CanReceive(nil) {
		t.Error("CanReceive should return true")
	}
}

func TestMixFrames(t *testing.T) {
	t.Parallel()

	// [samples][channels] format
	tone := [][]float32{{0.1}, {0.2}, {0.3}}
	echo := [][]float32{{0.2}, {0.3}, {0.4}}

	mixed := mixFrames(tone, echo)

	if len(mixed) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(mixed))
	}
	if len(mixed[0]) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(mixed[0]))
	}

	expected := []float32{0.3, 0.5, 0.7}
	for i, v := range mixed {
		if absFloat(v[0]-expected[i]) > 1e-5 {
			t.Errorf("sample %d: expected %v, got %v", i, expected[i], v[0])
		}
	}
}

func TestMixFrames_Clamping(t *testing.T) {
	t.Parallel()

	tone := [][]float32{{0.8}, {-0.8}}
	echo := [][]float32{{0.8}, {-0.8}}

	mixed := mixFrames(tone, echo)

	if mixed[0][0] != 1.0 {
		t.Errorf("expected clamped to 1.0, got %v", mixed[0][0])
	}
	if mixed[1][0] != -1.0 {
		t.Errorf("expected clamped to -1.0, got %v", mixed[1][0])
	}
}

func TestMixFrames_DifferentLengths(t *testing.T) {
	t.Parallel()

	tone := [][]float32{{0.1}, {0.2}, {0.3}, {0.4}}
	echo := [][]float32{{0.2}, {0.3}}

	mixed := mixFrames(tone, echo)

	if len(mixed) != 4 {
		t.Fatalf("expected 4 samples (max length), got %d", len(mixed))
	}

	expected := []float32{0.3, 0.5, 0.3, 0.4}
	for i, v := range mixed {
		if absFloat(v[0]-expected[i]) > 1e-5 {
			t.Errorf("sample %d: expected %v, got %v", i, expected[i], v[0])
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
	}{
		{int(5), 5},
		{int8(3), 3},
		{int16(300), 300},
		{int32(70000), 70000},
		{int64(200000), 200000},
		{uint8(200), 200},
		{uint16(500), 500},
		{uint32(100000), 100000},
		{uint64(999999), 999999},
		{float64(1.5), 0},
		{nil, 0},
	}

	for _, tt := range tests {
		got := toInt(tt.input)
		if got != tt.want {
			t.Errorf("toInt(%v %T) = %d, want %d", tt.input, tt.input, got, tt.want)
		}
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		input       string
		wantHost    string
		wantPort    int
		defaultHost string
		defaultPort int
	}{
		{"localhost:4242", "localhost", 4242, "localhost", 4242},
		{":8080", "localhost", 8080, "localhost", 4242},
		{"noport", "localhost", 4242, "localhost", 4242},
		{"example.com:9999", "example.com", 9999, "localhost", 4242},
	}

	for _, tt := range tests {
		host, port := parseHostPort(tt.input, tt.defaultHost, tt.defaultPort)
		if host != tt.wantHost {
			t.Errorf("parseHostPort(%q) host = %q, want %q", tt.input, host, tt.wantHost)
		}
		if port != tt.wantPort {
			t.Errorf("parseHostPort(%q) port = %d, want %d", tt.input, port, tt.wantPort)
		}
	}
}

func TestSetRNSConfigDirective(t *testing.T) {
	content := `[reticulum]
  share_instance = Yes

[interfaces]
  [[Default Interface]]
    type = AutoInterface
`
	result := setRNSConfigDirective(content, "share_instance", "No")

	if !contains(result, "share_instance = No") {
		t.Error("expected share_instance = No in result")
	}
	if contains(result, "share_instance = Yes") {
		t.Error("should not contain old share_instance = Yes")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}