// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package platforms

import (
	"testing"
)

// TestPortAudioBackend_LibraryName checks that the library name function
// returns a non-empty string for the current platform.
func TestPortAudioBackend_LibraryName(t *testing.T) {
	t.Parallel()

	name := portAudioLibraryName()
	if name == "" {
		t.Error("portAudioLibraryName returned empty string")
	}
	t.Logf("Library name: %s", name)
}

// TestPortAudioBackend_LibraryPaths checks that fallback library paths
// are provided for the current platform.
func TestPortAudioBackend_LibraryPaths(t *testing.T) {
	t.Parallel()

	paths := portAudioLibraryPaths()
	if len(paths) == 0 {
		t.Error("portAudioLibraryPaths returned empty list")
	}
	t.Logf("Library paths: %v", paths)
}

// TestPortAudioBackend_Creation verifies that the PortAudio backend can
// be created when libportaudio is available. On systems without
// libportaudio installed, this test is skipped. This test does NOT open
// any audio streams and is therefore safe to run in `go test ./...`.
func TestPortAudioBackend_Creation(t *testing.T) {
	t.Parallel()

	backend, err := NewPortAudioBackend(48000, 2, 32)
	if err != nil {
		t.Skipf("PortAudio unavailable: %v", err)
	}
	if backend == nil {
		t.Fatal("NewPortAudioBackend returned nil without error")
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

// TestPortAudioBackend_DeviceEnumeration checks that device lists are
// populated (or at least return ["default"] on headless systems). This
// test only queries device info and does NOT open any streams, so it is
// safe to run in `go test ./...`.
func TestPortAudioBackend_DeviceEnumeration(t *testing.T) {
	t.Parallel()

	backend, err := NewPortAudioBackend(48000, 2, 32)
	if err != nil {
		t.Skipf("PortAudio unavailable: %v", err)
	}

	mics := backend.AllMicrophones()
	if len(mics) == 0 {
		t.Error("AllMicrophones returned empty list")
	}

	speakers := backend.AllSpeakers()
	if len(speakers) == 0 {
		t.Error("AllSpeakers returned empty list")
	}

	if backend.DefaultMicrophone() == "" {
		t.Error("DefaultMicrophone returned empty string")
	}
	if backend.DefaultSpeaker() == "" {
		t.Error("DefaultSpeaker returned empty string")
	}

	t.Logf("Microphones: %v", mics)
	t.Logf("Speakers: %v", speakers)
}