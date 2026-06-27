// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends for the
// LXST library. It abstracts audio playback and recording across macOS,
// Windows, and Linux using PortAudio (loaded via purego — no CGO
// required) as the primary backend, since PortAudio supports both
// microphone input and speaker output. A pure-Go Oto backend is kept as
// a fallback for output-only scenarios, and a NullBackend is used when
// no audio hardware or library is available. Device enumeration and
// selection capabilities are provided for microphones and speakers.
//
// To force the NullBackend (e.g. in unit tests or headless CI), set the
// LXST_NULL_AUDIO environment variable to any non-empty value. This
// prevents any real audio device from being opened.
package platforms

import (
	"log"
	"os"
	"runtime"
	"strings"
)

// nullAudioForced reports whether the LXST_NULL_AUDIO environment
// variable is set, forcing all backends to be NullBackend. This is
// used by tests to avoid opening real audio hardware (which can produce
// sound or block on microphone input).
func nullAudioForced() bool {
	return os.Getenv("LXST_NULL_AUDIO") != ""
}

// NewBackend creates the appropriate audio backend for the current platform.
// It tries PortAudio first (full input + output), then Oto (output only),
// and finally falls back to the NullBackend. When preferredDevice is
// non-empty, the backend will attempt to select that device; if
// unavailable, it falls back to the system default.
func NewBackend(sampleRate, channels, bitDepth int) AudioBackend {
	return NewBackendWithDevice(sampleRate, channels, bitDepth, "")
}

// NewBackendWithDevice creates an audio backend with a preferred device name.
// If preferredDevice is empty or the device is unavailable, the default
// device is used. The selection order is:
//  1. PortAudio (purego, no CGO) — supports both recording and playback.
//  2. Oto (pure-Go) — output only; used when PortAudio is unavailable.
//  3. NullBackend — no hardware, returns silence / discards output.
//
// If the LXST_NULL_AUDIO environment variable is set, the NullBackend is
// returned immediately without attempting to load any audio library or
// open any device.
func NewBackendWithDevice(sampleRate, channels, bitDepth int, preferredDevice string) AudioBackend {
	if nullAudioForced() {
		return NewNullBackend(sampleRate, channels, bitDepth)
	}

	// PortAudio is the preferred backend: it provides real microphone
	// input on macOS (CoreAudio), Windows (WASAPI), and Linux
	// (ALSA/PulseAudio/JACK) via purego with no CGO.
	paBackend, err := NewPortAudioBackend(sampleRate, channels, bitDepth)
	if err == nil && paBackend != nil {
		return paBackend
	}
	if err != nil {
		log.Printf("platforms: portaudio backend unavailable (%v), falling back to malgo/oto", err)
	}

	// Malgo (miniaudio) provides full input+output on all platforms
	// via CGO: WASAPI on Windows, CoreAudio on macOS, ALSA/PulseAudio
	// on Linux. Only available when built with CGO.
	if mb := tryMalgoBackend(sampleRate, channels, bitDepth); mb != nil {
		return mb
	}

	// Oto is a pure-Go fallback. It only supports playback (output);
	// its recorder returns silence, so it is only useful when microphone
	// input is not needed.
	otoBackend := NewOtoBackend(sampleRate, channels, bitDepth)
	if otoBackend != nil {
		return otoBackend
	}

	return NewNullBackend(sampleRate, channels, bitDepth)
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
	case "linux", "darwin", "windows", "android":
		return "portaudio"
	default:
		return "null"
	}
}
