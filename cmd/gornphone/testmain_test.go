// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"os"
	"testing"

	"github.com/gmlewis/go-lxst/testutils"
)

// TestMain forces the NullBackend so that AudioPipeline and Telephone
// tests do not open real audio hardware (which would produce sound or
// block on microphone input). The gornphone-echo manual test scenario
// uses real audio via the --listen/--connect flags, not via these unit
// tests.
func TestMain(m *testing.M) {
	testutils.ForceNullAudio()
	os.Exit(m.Run())
}
