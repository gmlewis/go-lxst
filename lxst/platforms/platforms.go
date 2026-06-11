// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends.
package platforms

import (
	"runtime"
	"time"
)

// NewBackend creates the appropriate audio backend for the current platform.
// Uses Oto backend (pure-Go, cross-platform) when available, falls back to null backend.
func NewBackend(sampleRate, channels, bitDepth int) AudioBackend {
	// Try Oto backend first (pure-Go, works on all major platforms)
	backend := NewOtoBackend(sampleRate, channels, bitDepth)
	
	// Wait briefly to see if Oto initializes successfully
	time.Sleep(100 * time.Millisecond)
	
	// Check if backend is usable by trying to get a recorder/player
	// If Oto fails, fall back to NullBackend
	_, err := backend.GetRecorder(960)
	if err != nil {
		backend.ReleaseRecorder()
		return NewNullBackend(sampleRate, channels, bitDepth)
	}
	backend.ReleaseRecorder()
	
	_, err = backend.GetPlayer(960, false)
	if err != nil {
		backend.ReleasePlayer()
		return NewNullBackend(sampleRate, channels, bitDepth)
	}
	backend.ReleasePlayer()
	
	return backend
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