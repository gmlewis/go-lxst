// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
	"os"
	"testing"

	"github.com/gmlewis/go-lxst/testutils"
)

// TestMain forces the NullBackend so that LineSink construction does
// not open real audio hardware during tests.
func TestMain(m *testing.M) {
	testutils.ForceNullAudio()
	os.Exit(m.Run())
}