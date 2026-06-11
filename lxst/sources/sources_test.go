// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sources

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
)

func TestLoopback_StartStop(t *testing.T) {
	t.Parallel()
	lb := NewLoopback(codecs.NullCodec{}, nil)

	// Initially not running
	if lb.Running() {
		t.Error("Loopback should not be running initially")
	}

	// Start
	err := lb.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !lb.Running() {
		t.Error("Loopback should be running after Start()")
	}

	// Start again should fail
	err = lb.Start()
	if err == nil {
		t.Error("Start() should fail when already running")
	}

	// Stop
	err = lb.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if lb.Running() {
		t.Error("Loopback should not be running after Stop()")
	}

	// Stop again should succeed (idempotent)
	err = lb.Stop()
	if err != nil {
		t.Fatalf("Stop() should be idempotent: %v", err)
	}
}

func TestLoopback_CanReceive(t *testing.T) {
	t.Parallel()
	lb := NewLoopback(codecs.NullCodec{}, nil)

	// With no sink, should always be able to receive
	if !lb.CanReceive(nil) {
		t.Error("Should be able to receive when no sink")
	}

	// With sink that can receive
	ms1 := &mockSink{canReceive: true}
	lb2 := NewLoopback(codecs.NullCodec{}, ms1)
	if !lb2.CanReceive(nil) {
		t.Error("Should be able to receive when sink allows")
	}

	// With sink that cannot receive
	ms2 := &mockSink{canReceive: false}
	lb3 := NewLoopback(codecs.NullCodec{}, ms2)
	if lb3.CanReceive(nil) {
		t.Error("Should NOT be able to receive when sink denies")
	}
}

type mockSink struct {
	canReceive bool
	running    bool
}

func (m *mockSink) Start() error {
	m.running = true
	return nil
}

func (m *mockSink) Stop() error {
	m.running = false
	return nil
}

func (m *mockSink) Running() bool {
	return m.running
}

func (m *mockSink) HandleFrame(frame [][]float32, fromSource Source) error {
	return nil
}

func (m *mockSink) CanReceive(fromSource Source) bool {
	return m.canReceive
}

// We need a minimal Source interface implementation for testing
type testSource struct {
	running bool
}

func (t *testSource) Start() error {
	t.running = true
	return nil
}

func (t *testSource) Stop() error {
	t.running = false
	return nil
}

func (t *testSource) Running() bool {
	return t.running
}

func TestLoopback_HandleFrame(t *testing.T) {
	t.Parallel()
	codec := codecs.NullCodec{}
	mockSink := &mockSink{canReceive: true}
	lb := NewLoopback(codec, mockSink)

	_ = lb.Start()

	// Create test frame data (already decoded float32)
	frame := [][]float32{
		{0.5, -0.3},
		{-0.7, 0.9},
	}

	// Handle frame (pass decoded frames)
	err := lb.HandleFrame(frame, &testSource{})
	if err != nil {
		t.Errorf("HandleFrame failed: %v", err)
	}
}

func TestLoopback_HandleFrame_NotRunning(t *testing.T) {
	t.Parallel()
	lb := NewLoopback(codecs.NullCodec{}, nil)

	frame := [][]float32{{1.0, 2.0}}
	err := lb.HandleFrame(frame, &testSource{})
	if err == nil {
		t.Error("HandleFrame should fail when not running")
	}
}
