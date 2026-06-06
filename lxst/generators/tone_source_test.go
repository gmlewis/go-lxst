// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package generators

import (
	"math"
	"testing"

	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
)

func TestToneSource_New(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)
	if ts == nil {
		t.Fatal("NewToneSource returned nil")
	}
	if ts.Frequency() != 440.0 {
		t.Errorf("Expected frequency 440.0, got %f", ts.Frequency())
	}
	if ts.Gain() != 0.1 {
		t.Errorf("Expected gain 0.1, got %f", ts.Gain())
	}
}

func TestToneSource_DefaultValues(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(0, 0, true, 0, 0, nil, nil, 0)
	if ts.Frequency() != ToneSourceDefaultFrequency {
		t.Errorf("Expected default frequency %f, got %f", ToneSourceDefaultFrequency, ts.Frequency())
	}
	if ts.TargetFrameMs() != ToneSourceDefaultFrameMs {
		t.Errorf("Expected default frame ms %f, got %f", ToneSourceDefaultFrameMs, ts.TargetFrameMs())
	}
	if ts.Channels() != 1 {
		t.Errorf("Expected default 1 channel, got %d", ts.Channels())
	}
}

func TestToneSource_EaseIn(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)

	_ = ts.Start()

	ts.mu.Lock()
	easeGain := ts.easeGain
	ts.mu.Unlock()

	if easeGain != 0.0 {
		t.Errorf("Expected initial ease_gain 0.0 with ease, got %f", easeGain)
	}

	_ = ts.Stop()
}

func TestToneSource_NoEaseIn(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, false, 20.0, 80.0, nil, nil, 1)

	_ = ts.Start()

	ts.mu.Lock()
	easeGain := ts.easeGain
	ts.mu.Unlock()

	if easeGain != 1.0 {
		t.Errorf("Expected initial ease_gain 1.0 without ease, got %f", easeGain)
	}

	_ = ts.Stop()
}

func TestToneSource_SamplesPerFrame(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 20.0, nil, nil, 1)

	expectedSPF := int(math.Ceil(20.0 / 1000.0 * float64(ToneSourceDefaultSampleRate)))
	if ts.SamplesPerFrame() != expectedSPF {
		t.Errorf("Expected %d samples per frame, got %d", expectedSPF, ts.SamplesPerFrame())
	}
}

func TestToneSource_MultiChannel(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 2)
	if ts.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", ts.Channels())
	}
}

func TestToneSource_CodecConstraints(t *testing.T) {
	t.Parallel()

	opus, err := opusPkg.NewOpus(opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	ts := NewToneSource(440.0, 0.1, true, 20.0, 21.0, opus, nil, 1)

	if math.Abs(ts.TargetFrameMs()-20.0) > 0.001 {
		t.Errorf("Expected frame time 20.0ms (closest valid), got %f", ts.TargetFrameMs())
	}

	if ts.SampleRate() != 8000 {
		t.Errorf("Expected sample rate 8000 (Opus VOICE_LOW), got %d", ts.SampleRate())
	}
}

func TestToneSource_Generate(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.5, false, 20.0, 20.0, nil, nil, 1)

	ts.easeGain = 1.0
	frame := ts.generate()

	if len(frame) != ts.SamplesPerFrame() {
		t.Errorf("Expected %d samples, got %d", ts.SamplesPerFrame(), len(frame))
	}

	for _, s := range frame {
		if len(s) != 1 {
			t.Errorf("Expected 1 channel per sample, got %d", len(s))
		}
		if math.Abs(float64(s[0])) > 1.0 {
			t.Errorf("Sample amplitude %f exceeds 1.0", s[0])
		}
	}
}

func TestToneSource_StartStop(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)

	err := ts.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ts.mu.Lock()
	running := ts.shouldRun
	ts.mu.Unlock()
	if !running {
		t.Error("ToneSource should be running after Start()")
	}

	err = ts.Start()
	if err == nil {
		t.Error("Start() should fail when already running")
	}

	_ = ts.Stop()
}

func TestToneSource_SetFrequency(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)

	ts.SetFrequency(880.0)
	if ts.Frequency() != 880.0 {
		t.Errorf("Expected frequency 880.0, got %f", ts.Frequency())
	}
}

func TestToneSource_SetGain(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)

	ts.SetGain(0.5)
	if ts.Gain() != 0.5 {
		t.Errorf("Expected gain 0.5, got %f", ts.Gain())
	}
}