// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends.
package platforms

import (
	"runtime"
)

// NewBackend creates the appropriate audio backend for the current platform.
// Uses the null backend by default - real implementations should be provided per-platform.
func NewBackend(sampleRate, channels, bitDepth int) AudioBackend {
	// For now, always return null backend
	// Real implementations would use build tags to select platform-specific backends
	return NewNullBackend(sampleRate, channels, bitDepth)
}

// GetBackend returns the backend type name for the current platform.
func GetBackend() string {
	switch runtime.GOOS {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	case "windows":
		return "windows"
	case "android":
		return "android"
	default:
		return "null"
	}
}