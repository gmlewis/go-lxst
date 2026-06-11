// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package platforms

import (
	"testing"
	"time"
)

func TestOtoBackend_Creation(t *testing.T) {
	t.Parallel()

	backend := NewOtoBackend(48000, 2, 32)
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

	backend := NewOtoBackend(48000, 2, 32)
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

	backend := NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend returned nil")
	}

	recorder, err := backend.GetRecorder(960)
	if err != nil {
		// May fail in headless environments
		t.Logf("GetRecorder failed (expected in CI): %v", err)
		backend.ReleaseRecorder()
		return
	}
	defer func() {
		recorder.Close()
		backend.ReleaseRecorder()
	}()

	// Record with a short timeout to avoid blocking in headless environments
	done := make(chan struct{})
	var frames [][]float32
	var recordErr error
	go func() {
		frames, recordErr = recorder.Record(960)
		close(done)
	}()

	select {
	case <-done:
		if recordErr != nil {
			t.Logf("Record failed (expected in CI): %v", recordErr)
			return
		}
		if len(frames) != 960 {
			t.Errorf("Expected 960 frames, got %d", len(frames))
		}
		if len(frames) > 0 && len(frames[0]) != 2 {
			t.Errorf("Expected 2 channels, got %d", len(frames[0]))
		}
	case <-time.After(2 * time.Second):
		// Close recorder to unblock the goroutine
		recorder.Close()
		<-done
		t.Log("Record timed out (expected in headless environment)")
	}
}

func TestOtoBackend_GetPlayer(t *testing.T) {
	t.Parallel()

	backend := NewOtoBackend(48000, 2, 32)
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

	backend := NewOtoBackend(48000, 2, 32)
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

	backend := NewOtoBackend(48000, 2, 32)
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
		backend := NewOtoBackend(sr, 1, 16)
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
	backend := NewOtoBackend(48000, 2, 32)
	if backend == nil {
		t.Fatal("NewOtoBackend failed for stereo")
	}
	if backend.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", backend.Channels())
	}
}

func TestNewBackendWithDevice(t *testing.T) {
	t.Parallel()

	backend := NewBackendWithDevice(48000, 2, 32, "")
	if backend == nil {
		t.Fatal("NewBackendWithDevice returned nil")
	}

	// With empty preferred device, should work normally
	if backend.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", backend.SampleRate())
	}
}

func TestNewBackendWithDevice_PreferredDevice(t *testing.T) {
	t.Parallel()

	// With a non-existent device, should fall back to default
	backend := NewBackendWithDevice(48000, 2, 32, "nonexistent-device")
	if backend == nil {
		t.Fatal("NewBackendWithDevice returned nil")
	}
}

func TestDeviceInList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		device   string
		devices  []string
		expected bool
	}{
		{"exact match", "default", []string{"default", "external"}, true},
		{"case insensitive", "Default", []string{"default"}, true},
		{"no match", "missing", []string{"default", "external"}, false},
		{"empty list", "default", []string{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deviceInList(tc.device, tc.devices)
			if got != tc.expected {
				t.Errorf("deviceInList(%q, %v) = %v, want %v", tc.device, tc.devices, got, tc.expected)
			}
		})
	}
}

func TestNullBackend_DeviceEnumeration(t *testing.T) {
	t.Parallel()

	backend := NewNullBackend(48000, 2, 32)

	mics := backend.AllMicrophones()
	if len(mics) != 1 || mics[0] != "null-mic" {
		t.Errorf("Expected ['null-mic'], got %v", mics)
	}

	speakers := backend.AllSpeakers()
	if len(speakers) != 1 || speakers[0] != "null-speaker" {
		t.Errorf("Expected ['null-speaker'], got %v", speakers)
	}

	if backend.DefaultMicrophone() != "null-mic" {
		t.Errorf("Expected 'null-mic', got %q", backend.DefaultMicrophone())
	}
	if backend.DefaultSpeaker() != "null-speaker" {
		t.Errorf("Expected 'null-speaker', got %q", backend.DefaultSpeaker())
	}
}
