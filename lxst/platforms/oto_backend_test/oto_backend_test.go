// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package platforms_oto_backend_test

import (
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/platforms"
)

func TestOtoBackend_Creation(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	if backend.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", backend.SampleRate())
	}
	if backend.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", backend.Channels())
	}
	if backend.BitDepth() != 32 {
		t.Errorf("Expected bit depth 32, got %d", backend.BitDepth())
	}
}

func TestOtoBackend_DeviceEnumeration(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	mics := backend.AllMicrophones()
	if len(mics) == 0 {
		t.Log("No microphones found (may be headless environment)")
	} else {
		t.Logf("Microphones: %v", mics)
	}

	speakers := backend.AllSpeakers()
	if len(speakers) == 0 {
		t.Log("No speakers found (may be headless environment)")
	} else {
		t.Logf("Speakers: %v", speakers)
	}

	defaultMic := backend.DefaultMicrophone()
	if defaultMic == "" && len(mics) > 0 {
		t.Error("Default microphone should be set when microphones exist")
	}

	defaultSpeaker := backend.DefaultSpeaker()
	if defaultSpeaker == "" && len(speakers) > 0 {
		t.Error("Default speaker should be set when speakers exist")
	}
}

func TestOtoBackend_GetRecorder(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	recorder, err := backend.GetRecorder(960) // 20ms at 48kHz
	if err != nil {
		// May fail in headless environments - that's OK for unit tests
		t.Logf("GetRecorder failed (expected in CI): %v", err)
		return
	}
	if recorder == nil {
		t.Fatal("GetRecorder returned nil recorder")
	}
	defer recorder.Close()

	// Try to record a few frames
	frames, err := recorder.Record(960)
	if err != nil {
		t.Logf("Record failed (expected in CI): %v", err)
		return
	}

	if len(frames) != 960 {
		t.Errorf("Expected 960 frames, got %d", len(frames))
	}
	if len(frames) > 0 && len(frames[0]) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(frames[0]))
	}
}

func TestOtoBackend_GetPlayer(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	player, err := backend.GetPlayer(960, false)
	if err != nil {
		t.Logf("GetPlayer failed (expected in CI): %v", err)
		return
	}
	if player == nil {
		t.Fatal("GetPlayer returned nil player")
	}
	defer player.Close()

	// Create test frame (silence)
	frame := make([][]float32, 960)
	for i := range frame {
		frame[i] = make([]float32, 2)
	}

	err = player.Play(frame)
	if err != nil {
		t.Logf("Play failed (expected in CI): %v", err)
		return
	}

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
}

func TestOtoBackend_Flush(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	err := backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestOtoBackend_ReleaseRecorderPlayer(t *testing.T) {
	t.Parallel()

	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	// Get and release recorder
	recorder, err := backend.GetRecorder(960)
	if err == nil && recorder != nil {
		recorder.Close()
		err = backend.ReleaseRecorder()
		if err != nil {
			t.Errorf("ReleaseRecorder failed: %v", err)
		}
	}

	// Get and release player
	player, err := backend.GetPlayer(960, false)
	if err == nil && player != nil {
		player.Close()
		err = backend.ReleasePlayer()
		if err != nil {
			t.Errorf("ReleasePlayer failed: %v", err)
		}
	}
}

func TestOtoBackend_FormatConversion(t *testing.T) {
	t.Parallel()

	// Test with different sample rates
	for _, sr := range []int{8000, 16000, 44100, 48000} {
		backend := platforms.NewOtoBackend(sr, 1, 16)
		if backend == nil {
			t.Fatalf("NewOtoBackend failed for sample rate %d", sr)
		}
		if backend.SampleRate() != sr {
			t.Errorf("Sample rate mismatch for %d: got %d", sr, backend.SampleRate())
		}
		if backend.Channels() != 1 {
			t.Errorf("Expected 1 channel for %d Hz, got %d", sr, backend.Channels())
		}
	}

	// Test stereo
	backend := platforms.NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend failed for stereo")
	}
	if backend.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", backend.Channels())
	}
}