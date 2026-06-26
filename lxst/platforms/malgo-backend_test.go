// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

package platforms

import (
	"testing"
)

func TestMalgoBackend_Creation(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
	}
	if backend == nil {
		t.Fatal("NewMalgoBackend returned nil")
	}

	if backend.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %v", backend.SampleRate())
	}
	if backend.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %v", backend.Channels())
	}
	if backend.BitDepth() != 32 {
		t.Errorf("Expected bit depth 32, got %v", backend.BitDepth())
	}
}

func TestMalgoBackend_DeviceEnumeration(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
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

func TestMalgoBackend_GetRecorder(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
	}

	recorder, err := backend.GetRecorder(960)
	if err != nil {
		t.Logf("GetRecorder failed (expected in CI): %v", err)
		return
	}
	if recorder == nil {
		t.Fatal("GetRecorder returned nil recorder")
	}
	defer func() { _ = recorder.Close() }()

	frames, err := recorder.Record(960)
	if err != nil {
		t.Logf("Record failed (expected in CI): %v", err)
		return
	}

	if len(frames) != 960 {
		t.Errorf("Expected 960 frames, got %v", len(frames))
	}
	if len(frames) > 0 && len(frames[0]) != 2 {
		t.Errorf("Expected 2 channels, got %v", len(frames[0]))
	}
}

func TestMalgoBackend_GetPlayer(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
	}

	player, err := backend.GetPlayer(960, false)
	if err != nil {
		t.Logf("GetPlayer failed (expected in CI): %v", err)
		return
	}
	if player == nil {
		t.Fatal("GetPlayer returned nil player")
	}
	defer func() { _ = player.Close() }()

	frame := make([][]float32, 960)
	for i := range frame {
		frame[i] = make([]float32, 2)
	}

	err = player.Play(frame)
	if err != nil {
		t.Logf("Play failed (expected in CI): %v", err)
		return
	}
}

func TestMalgoBackend_Flush(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestMalgoBackend_ReleaseRecorderPlayer(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(48000, 2, 32)
	if err != nil {
		t.Fatalf("NewMalgoBackend returned error: %v", err)
	}

	recorder, err := backend.GetRecorder(960)
	if err == nil && recorder != nil {
		_ = recorder.Close()
		err = backend.ReleaseRecorder()
		if err != nil {
			t.Errorf("ReleaseRecorder failed: %v", err)
		}
	}

	player, err := backend.GetPlayer(960, false)
	if err == nil && player != nil {
		_ = player.Close()
		err = backend.ReleasePlayer()
		if err != nil {
			t.Errorf("ReleasePlayer failed: %v", err)
		}
	}
}

func TestMalgoBackend_Defaults(t *testing.T) {
	t.Parallel()

	backend, err := NewMalgoBackend(0, 0, 0)
	if err != nil {
		t.Fatalf("NewMalgoBackend with zero values returned error: %v", err)
	}
	if backend.SampleRate() != 48000 {
		t.Errorf("Expected default sample rate 48000, got %v", backend.SampleRate())
	}
	if backend.Channels() != 2 {
		t.Errorf("Expected default 2 channels, got %v", backend.Channels())
	}
}
