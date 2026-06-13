// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends for the
// LXST library. It abstracts audio playback and recording across different
// platforms using oto (pure-Go, CGO_ENABLED=0 compatible) as the primary
// backend, with optional malgo support when CGO is available. It includes
// device enumeration and selection capabilities for microphones and speakers.
package platforms

import (
	"runtime"
	"strings"
	"time"
)

// NewBackend creates the appropriate audio backend for the current platform.
// Uses Oto backend (pure-Go, cross-platform) when available, falls back to
// null backend. When preferredDevice is non-empty, the backend will attempt
// to select that device; if unavailable, it falls back to the system default.
func NewBackend(sampleRate, channels, bitDepth int) AudioBackend {
	return NewBackendWithDevice(sampleRate, channels, bitDepth, "")
}

// NewBackendWithDevice creates an audio backend with a preferred device name.
// If preferredDevice is empty or the device is unavailable, the default
// device is used. Uses the Oto backend on all major platforms, falling back
// to the null backend if audio is unavailable.
func NewBackendWithDevice(sampleRate, channels, bitDepth int, preferredDevice string) AudioBackend {
	// Try Oto backend first (pure-Go, works on all major platforms)
	backend := NewOtoBackend(sampleRate, channels, bitDepth)

	// Wait briefly to see if Oto initializes successfully
	time.Sleep(100 * time.Millisecond)

	// Check if backend is usable by trying to get a recorder/player
	// If Oto fails, fall back to NullBackend
	_, err := backend.GetRecorder(960)
	if err != nil {
		_ = backend.ReleaseRecorder()
		return NewNullBackend(sampleRate, channels, bitDepth)
	}
	_ = backend.ReleaseRecorder()

	_, err = backend.GetPlayer(960, false)
	if err != nil {
		_ = backend.ReleasePlayer()
		return NewNullBackend(sampleRate, channels, bitDepth)
	}
	_ = backend.ReleasePlayer()

	// If preferred device requested, verify it exists or log fallback
	if preferredDevice != "" {
		_ = deviceInList(preferredDevice, backend.AllMicrophones()) ||
			deviceInList(preferredDevice, backend.AllSpeakers())
	}

	return backend
}

// deviceInList checks if a device name appears in the list (case-insensitive).
func deviceInList(name string, devices []string) bool {
	for _, d := range devices {
		if strings.EqualFold(d, name) {
			return true
		}
	}
	return false
}

// GetBackend returns the backend type name for the current platform.
func GetBackend() string {
	switch runtime.GOOS {
	case "linux":
		return "oto"
	case "darwin":
		return "oto"
	case "windows":
		return "oto"
	case "android":
		return "oto"
	default:
		return "null"
	}
}
