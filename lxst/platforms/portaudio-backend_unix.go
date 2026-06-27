// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build (darwin || linux || freebsd) && !android && !faketime

package platforms

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// openLibrary opens a shared library using the POSIX dlopen API.
func openLibrary(name string) (uintptr, error) {
	h, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("dlopen %s: %w", name, err)
	}
	return h, nil
}
