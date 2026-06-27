// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

package platforms

import "log"

// tryMalgoBackend attempts to create a malgo (miniaudio) backend.
// Returns nil if initialization fails.
func tryMalgoBackend(sampleRate, channels, bitDepth int) AudioBackend {
	mb, err := NewMalgoBackend(sampleRate, channels, bitDepth)
	if err != nil {
		log.Printf("platforms: malgo backend unavailable (%v)", err)
		return nil
	}
	return mb
}
