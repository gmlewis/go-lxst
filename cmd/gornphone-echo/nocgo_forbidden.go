// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build !cgo

package main

// gornphone-echo requires CGO for the Opus audio codec (libopus).
// Without CGO, echo calls connect but transfer no audio — a silent failure.
//
// Build with CGO enabled:
//
//	CGO_ENABLED=1 go install github.com/gmlewis/go-lxst/cmd/gornphone-echo@latest
//
// Install libopus first:
//
//	brew install opus          # macOS
//	sudo apt install libopus-dev  # Debian/Ubuntu
var _ = cgoRequired
