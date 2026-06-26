// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

// Package platforms test: hardware-touching PortAudio backend tests.
// These tests open real audio streams and may produce sound or block on
// microphone input. They are gated behind the "integration" build tag
// so that `go test ./...` is safe and silent by default. Run with:
//
//	go test -tags integration ./lxst/platforms/
package platforms

import (
	"testing"
)

// TestPortAudioBackend_GetRecorderPlayer verifies that a recorder and
// player can be opened on the default devices. This requires audio
// hardware; it is skipped on headless systems or when libportaudio is
// missing. It may produce sound on the default output device.
func TestPortAudioBackend_GetRecorderPlayer(t *testing.T) {
	t.Parallel()

	backend, err := NewPortAudioBackend(48000, 1, 32)
	if err != nil {
		t.Skipf("PortAudio unavailable: %v", err)
	}

	recorder, err := backend.GetRecorder(480)
	if err != nil {
		t.Skipf("Cannot open recorder (no input device?): %v", err)
	}
	if recorder == nil {
		t.Fatal("GetRecorder returned nil without error")
	}
	defer func() {
		_ = recorder.Close()
		_ = backend.ReleaseRecorder()
	}()

	// Read a single frame to confirm the stream is live. We don't
	// assert on the content (it may be silence on some devices).
	frame, err := recorder.Record(480)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if len(frame) != 480 {
		t.Errorf("Expected 480 frames, got %v", len(frame))
	}
}

// TestPortAudioBackend_GetPlayer verifies that a player can be opened
// and accepts a silence frame. This may produce a brief click on the
// default output device but should not produce loud audio since only
// silence is written.
func TestPortAudioBackend_GetPlayer(t *testing.T) {
	t.Parallel()

	backend, err := NewPortAudioBackend(48000, 1, 32)
	if err != nil {
		t.Skipf("PortAudio unavailable: %v", err)
	}

	player, err := backend.GetPlayer(480, false)
	if err != nil {
		t.Skipf("Cannot open player (no output device?): %v", err)
	}
	if player == nil {
		t.Fatal("GetPlayer returned nil without error")
	}
	defer func() {
		_ = player.Close()
		_ = backend.ReleasePlayer()
	}()

	// Play a short silence frame to confirm the stream accepts writes.
	silence := make([][]float32, 480)
	for i := range silence {
		silence[i] = make([]float32, 1)
	}
	if err := player.Play(silence); err != nil {
		t.Fatalf("Play failed: %v", err)
	}
}

// TestPortAudioBackend_GetRecorderNotInUse verifies that GetRecorder
// returns an error when called twice (recorder already in use). Opens a
// real input stream but does not read from it.
func TestPortAudioBackend_GetRecorderNotInUse(t *testing.T) {
	t.Parallel()

	backend, err := NewPortAudioBackend(48000, 1, 32)
	if err != nil {
		t.Skipf("PortAudio unavailable: %v", err)
	}

	rec1, err := backend.GetRecorder(480)
	if err != nil {
		t.Skipf("Cannot open recorder: %v", err)
	}
	defer func() {
		_ = rec1.Close()
		_ = backend.ReleaseRecorder()
	}()

	// A second recorder should fail because the first is still held.
	if _, err := backend.GetRecorder(480); err == nil {
		t.Error("Expected error when opening second recorder, got nil")
	}
}
