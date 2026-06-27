// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build !cgo

package platforms

// tryMalgoBackend returns nil when CGO is not available.
func tryMalgoBackend(sampleRate, channels, bitDepth int) AudioBackend {
	return nil
}
