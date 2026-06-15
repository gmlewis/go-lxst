// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
	"os"
	"path/filepath"
	"testing"

	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func TestOpusFileSink_New(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Fatalf("NewOpusFileSink failed: %v", err)
	}

	if fs.Running() {
		t.Error("OpusFileSink should not be running initially")
	}
}

func TestOpusFileSink_CanReceive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	if !fs.CanReceive(nil) {
		t.Error("Should be able to receive when buffer is empty")
	}

	for i := 0; i < fs.bufferMaxHeight; i++ {
		fs.frameDeque = append(fs.frameDeque, [][]float32{{0.5, 0.5}})
	}

	if fs.CanReceive(nil) {
		t.Error("Should not be able to receive when buffer is at max height")
	}
}

func TestOpusFileSink_CanReceive_Stopped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	fs.recordingStopped = true
	if fs.CanReceive(nil) {
		t.Error("Should not receive when recording is stopped")
	}
}

func TestOpusFileSink_HandleFrame_SetsSamplesPerFrame(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	if fs.SamplesPerFrame() != 0 {
		t.Errorf("Expected initial samples_per_frame=0, got %d", fs.SamplesPerFrame())
	}

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.5, 0.5}
	}

	mockSrc := &mockSourceForSink{sampleRate: 8000}
	err = fs.HandleFrame(frame, mockSrc)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}

	if fs.SamplesPerFrame() != 160 {
		t.Errorf("Expected samples_per_frame=160, got %d", fs.SamplesPerFrame())
	}
}

func TestOpusFileSink_FramesWaiting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	if fs.FramesWaiting() != 0 {
		t.Errorf("Expected 0 frames waiting, got %d", fs.FramesWaiting())
	}

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.5, 0.5}
	}

	_ = fs.HandleFrame(frame, nil)

	if fs.FramesWaiting() != 1 {
		t.Errorf("Expected 1 frame waiting, got %d", fs.FramesWaiting())
	}
}

func TestOpusFileSink_Profile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	if fs.Profile() != opusPkg.PROFILE_VOICE_LOW {
		t.Errorf("Expected profile %d, got %d", opusPkg.PROFILE_VOICE_LOW, fs.Profile())
	}
}

func TestOpusFileSink_OutputSamplerate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	if fs.OutputSamplerate() != 8000 {
		t.Errorf("Expected output sample rate 8000 for VOICE_LOW, got %d", fs.OutputSamplerate())
	}
}

func TestOpusFileSink_ChannelAdjustment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	// VOICE_LOW has 1 channel
	if fs.Channels() != 1 {
		t.Errorf("Expected 1 channel for VOICE_LOW, got %d", fs.Channels())
	}
}

func TestOpusFileSink_Stop(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	fs, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	_ = fs.Stop()

	if fs.Running() {
		t.Error("OpusFileSink should not be running after Stop()")
	}
}

func TestOpusFileSink_FileCreation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.opus")

	_, err := NewOpusFileSink(path, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	// File should not exist yet until we write data
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should not exist until data is written")
	}
}

type mockSourceForSink struct {
	sampleRate int
}

func (m *mockSourceForSink) Start() error                              { return nil }
func (m *mockSourceForSink) Stop() error                               { return nil }
func (m *mockSourceForSink) Running() bool                             { return true }
func (m *mockSourceForSink) SampleRate() int                           { return m.sampleRate }
func (m *mockSourceForSink) CanReceive(fromSource sources.Source) bool { return true }
func (m *mockSourceForSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}
func (m *mockSourceForSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}
