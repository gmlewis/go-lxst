// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

// Package platforms test: hardware-touching Oto backend tests.
// These tests open real audio output streams and may produce sound.
// They are gated behind the "integration" build tag so that
// `go test ./...` is safe and silent by default. Run with:
//
//	go test -tags integration ./lxst/platforms/
package platforms

import (
	"testing"
	"time"
)

func TestOtoBackend_GetPlayer(t *testing.T) {
	t.Parallel()

	backend := NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	player, err := backend.GetPlayer(960, false)
	if err != nil {
		t.Skipf("GetPlayer failed (expected in CI): %v", err)
	}
	if player == nil {
		t.Fatal("GetPlayer returned nil player")
	}
	defer func() { _ = player.Close() }()

	// Check if speakers are available
	speakers := backend.AllSpeakers()
	if len(speakers) == 0 {
		t.Skip("No speakers available (headless environment)")
	}

	// Create test frame (silence)
	frame := make([][]float32, 960)
	for i := range frame {
		frame[i] = make([]float32, 2)
	}

	// Use a goroutine with timeout to avoid blocking
	done := make(chan error, 1)
	go func() {
		done <- player.Play(frame)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Play failed (expected in CI): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Log("Play timed out (expected in headless environment)")
	}
}

func TestOtoBackend_ReleaseRecorderPlayer(t *testing.T) {
	t.Parallel()

	backend := NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	// Get and release recorder
	recorder, err := backend.GetRecorder(960)
	if err == nil && recorder != nil {
		_ = recorder.Close()
		err = backend.ReleaseRecorder()
		if err != nil {
			t.Errorf("ReleaseRecorder failed: %v", err)
		}
	}

	// Get and release player
	player, err := backend.GetPlayer(960, false)
	if err == nil && player != nil {
		_ = player.Close()
		err = backend.ReleasePlayer()
		if err != nil {
			t.Errorf("ReleasePlayer failed: %v", err)
		}
	}
}
