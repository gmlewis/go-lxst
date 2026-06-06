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

	ls := NewLineSink("", true, false)
	if ls == nil {
		t.Fatal("NewLineSink returned nil")
	}
	if ls.Running() {
		t.Error("LineSink should not be running initially")
	}
	if ls.SampleRate() <= 0 {
		t.Errorf("Expected positive sample rate, got %d", ls.SampleRate())
	}
}

func TestLineSink_CanReceive(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false)

	for i := 0; i < ls.bufferMaxHeight; i++ {
		if !ls.CanReceive(nil) {
			t.Errorf("Should be able to receive at buffer height %d/%d", i, ls.bufferMaxHeight)
		}
		ls.frameDeque = append(ls.frameDeque, [][]float32{{0.5, 0.5}})
	}

	if ls.CanReceive(nil) {
		t.Error("Should not be able to receive when buffer is at max height")
	}
}

func TestLineSink_HandleFrame(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false)

	frame := [][]float32{
		{0.5, -0.3},
		{-0.7, 0.9},
	}

	err := ls.HandleFrame(frame, nil)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	if ls.SamplesPerFrame() != 2 {
		t.Errorf("Expected samples_per_frame=2, got %d", ls.SamplesPerFrame())
	}

	ls.insertLock.Lock()
	if len(ls.frameDeque) != 1 {
		t.Errorf("Expected 1 frame in deque, got %d", len(ls.frameDeque))
	}
	ls.insertLock.Unlock()
}

func TestLineSink_HandleFrame_SetsSamplesPerFrame(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", false, false)
	if ls.SamplesPerFrame() != 0 {
		t.Errorf("Expected initial samples_per_frame=0, got %d", ls.SamplesPerFrame())
	}

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.5, 0.5}
	}

	err := ls.HandleFrame(frame, nil)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	if ls.SamplesPerFrame() != 160 {
		t.Errorf("Expected samples_per_frame=160, got %d", ls.SamplesPerFrame())
	}
}

func TestLineSink_Autostart(t *testing.T) {
	t.Parallel()

	ls := NewLineSink("", true, false)

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.0, 0.0}
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

	ls := NewLineSink("", false, false)
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

	ls := NewLineSink("", false, false)
	ls.channels = 2

	if ls.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", ls.Channels())
	}
}

type testSource struct{}

func (t *testSource) Start() error     { return nil }
func (t *testSource) Stop() error      { return nil }
func (t *testSource) Running() bool    { return true }