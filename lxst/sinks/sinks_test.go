// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
	"testing"
)

func TestLocalSink_CanReceive(t *testing.T) {
	t.Parallel()

	ls := &LocalSink{}
	if !ls.CanReceive(nil) {
		t.Error("LocalSink should always be able to receive")
	}
}

func TestLocalSink_HandleFrame(t *testing.T) {
	t.Parallel()

	ls := &LocalSink{}
	frame := [][]float32{{0.5, -0.3}, {-0.7, 0.9}}
	err := ls.HandleFrame(frame, nil)
	if err != nil {
		t.Errorf("LocalSink HandleFrame should not error: %v", err)
	}
}

func TestRemoteSink_CanReceive(t *testing.T) {
	t.Parallel()

	rs := &RemoteSink{}
	if !rs.CanReceive(nil) {
		t.Error("RemoteSink should always be able to receive")
	}
}

func TestLineSink_New(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", true, false, 0)
	if ls == nil {
		t.Fatal("NewLineSink returned nil")
	}
	if ls.Running() {
		t.Error("LineSink should not be running initially")
	}
	if ls.SampleRate() <= 0 {
		t.Errorf("Expected positive sample rate, got %v", ls.SampleRate())
	}
}

func TestLineSink_CanReceive(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false, 0)

	for i := 0; i < ls.bufferMaxHeight; i++ {
		if !ls.CanReceive(nil) {
			t.Errorf("Should be able to receive at buffer height %v/%v", i, ls.bufferMaxHeight)
		}
		ls.frameDeque = append(ls.frameDeque, [][]float32{{0.5, 0.5}})
	}

	if ls.CanReceive(nil) {
		t.Error("Should not be able to receive when buffer is at max height")
	}
}

func TestLineSink_HandleFrame(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false, 0)

	frame := [][]float32{
		{0.5, -0.3},
		{-0.7, 0.9},
	}

	err := ls.HandleFrame(frame, nil)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	if ls.SamplesPerFrame() != 2 {
		t.Errorf("Expected samples_per_frame=2, got %v", ls.SamplesPerFrame())
	}

	ls.insertLock.Lock()
	if len(ls.frameDeque) != 1 {
		t.Errorf("Expected 1 frame in deque, got %v", len(ls.frameDeque))
	}
	ls.insertLock.Unlock()
}

func TestLineSink_HandleFrame_SetsSamplesPerFrame(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false, 0)
	if ls.SamplesPerFrame() != 0 {
		t.Errorf("Expected initial samples_per_frame=0, got %v", ls.SamplesPerFrame())
	}

	// Frame format is [samples][channels]: 160 samples, 1 channel.
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = make([]float32, 1)
		frame[i][0] = 0.5
	}

	err := ls.HandleFrame(frame, nil)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	if ls.SamplesPerFrame() != 160 {
		t.Errorf("Expected samples_per_frame=160, got %v", ls.SamplesPerFrame())
	}
}

func TestLineSink_Autostart(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", true, false, 0)

	frame := make([][]float32, 2)
	for i := range frame {
		frame[i] = make([]float32, 160)
	}

	err := ls.HandleFrame(frame, &testSource{})
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	ls.mu.Lock()
	running := ls.shouldRun
	ls.mu.Unlock()

	if !running {
		t.Error("LineSink should auto-start when buffer reaches autostart_min")
	}

	_ = ls.Stop()
}

func TestLineSink_EnableLowLatency(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false, 0)
	ls.EnableLowLatency()

	ls.mu.Lock()
	wantsLow := ls.wantsLowLatency
	ls.mu.Unlock()

	if !wantsLow {
		t.Error("EnableLowLatency should set wantsLowLatency")
	}
}

func TestLineSink_ChannelReduction(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false, 0)
	ls.channels = 2

	if ls.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %v", ls.Channels())
	}
}

type testSource struct{}

func (t *testSource) Start() error  { return nil }
func (t *testSource) Stop() error   { return nil }
func (t *testSource) Running() bool { return true }

func TestLineSink_PreferredDevice(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("my-speaker", true, false, 0)
	if ls.PreferredDevice() != "my-speaker" {
		t.Errorf("Expected preferred device 'my-speaker', got %q", ls.PreferredDevice())
	}

	ls2 := NewLineSink("", true, false, 0)
	if ls2.PreferredDevice() != "" {
		t.Errorf("Expected empty preferred device, got %q", ls2.PreferredDevice())
	}
}

func TestLineSink_AvailableSpeakers(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", true, false, 0)
	speakers := ls.AvailableSpeakers()
	// With null backend, should at least have "null-speaker"
	if len(speakers) == 0 {
		t.Log("No speakers available (may be headless environment)")
	}
}

func TestAdaptChannels_NoOp(t *testing.T) {
	t.Parallel()

	// When source and target channels match, the frame is unchanged.
	frame := [][]float32{{0.1, 0.2}, {0.3, 0.4}}
	result := adaptChannels(frame, 2)
	if len(result) != 2 || len(result[0]) != 2 {
		t.Errorf("Expected 2x2 frame, got %vx%v", len(result), len(result[0]))
	}
	if result[0][0] != 0.1 || result[1][1] != 0.4 {
		t.Error("adaptChannels modified a matching frame")
	}
}

func TestAdaptChannels_Trim(t *testing.T) {
	t.Parallel()

	// When source has more channels than the target, extra channels
	// are trimmed (downmix by truncation, matching Python LineSink).
	frame := [][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}}
	result := adaptChannels(frame, 2)
	if len(result) != 2 {
		t.Fatalf("Expected 2 samples, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 2 {
			t.Fatalf("Expected 2 channels at sample %v, got %v", i, len(result[i]))
		}
	}
	if result[0][0] != 0.1 || result[0][1] != 0.2 {
		t.Errorf("Trim produced wrong values: %v", result[0])
	}
	if result[1][0] != 0.4 || result[1][1] != 0.5 {
		t.Errorf("Trim produced wrong values: %v", result[1])
	}
}

func TestAdaptChannels_UpmixMonoToStereo(t *testing.T) {
	t.Parallel()

	// The primary bug: mono frames (1 channel) fed to a stereo
	// player (2 channels) must be upmixed by duplicating the mono
	// channel, matching Python soundcard player.play().
	frame := [][]float32{{0.1}, {0.2}, {0.3}}
	result := adaptChannels(frame, 2)
	if len(result) != 3 {
		t.Fatalf("Expected 3 samples, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 2 {
			t.Fatalf("Expected 2 channels at sample %v, got %v", i, len(result[i]))
		}
		if result[i][0] != result[i][1] {
			t.Errorf("Expected L==R at sample %v: got L=%v R=%v", i, result[i][0], result[i][1])
		}
	}
	if result[0][0] != 0.1 || result[1][0] != 0.2 || result[2][0] != 0.3 {
		t.Error("Upmix altered the mono source values")
	}
}

func TestAdaptChannels_UpmixMonoToThree(t *testing.T) {
	t.Parallel()

	// Mono to 3 channels: the single channel fills all 3.
	frame := [][]float32{{0.5}, {-0.5}}
	result := adaptChannels(frame, 3)
	if len(result) != 2 {
		t.Fatalf("Expected 2 samples, got %v", len(result))
	}
	for i := range result {
		if len(result[i]) != 3 {
			t.Fatalf("Expected 3 channels at sample %v, got %v", i, len(result[i]))
		}
		for c := range result[i] {
			if result[i][c] != frame[i][0] {
				t.Errorf("Expected channel %v = %v, got %v", c, frame[i][0], result[i][c])
			}
		}
	}
}

func TestAdaptChannels_EmptyFrame(t *testing.T) {
	t.Parallel()

	result := adaptChannels(nil, 2)
	if result != nil {
		t.Error("Expected nil for nil frame")
	}

	result = adaptChannels([][]float32{}, 2)
	if len(result) != 0 {
		t.Errorf("Expected empty frame, got %v", len(result))
	}
}

func TestAdaptChannels_ZeroTargetChannels(t *testing.T) {
	t.Parallel()

	frame := [][]float32{{0.1}, {0.2}}
	result := adaptChannels(frame, 0)
	if len(result) != 2 || len(result[0]) != 1 {
		t.Error("Expected unchanged frame for 0 target channels")
	}
}

func TestAdaptChannels_UpmixStereoToThree(t *testing.T) {
	t.Parallel()

	// 2 channels to 3: cycle through available channels.
	frame := [][]float32{{0.1, 0.2}}
	result := adaptChannels(frame, 3)
	if len(result) != 1 {
		t.Fatalf("Expected 1 sample, got %v", len(result))
	}
	if len(result[0]) != 3 {
		t.Fatalf("Expected 3 channels, got %v", len(result[0]))
	}
	if result[0][0] != 0.1 || result[0][1] != 0.2 || result[0][2] != 0.1 {
		t.Errorf("Expected [0.1, 0.2, 0.1], got %v", result[0])
	}
}
