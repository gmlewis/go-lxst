// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package common

import "testing"

func TestNop(t *testing.T) {
	t.Parallel()
	// Nop should not panic
	Nop()
}