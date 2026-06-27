// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build windows

package platforms

import (
	"fmt"
	"syscall"
)

// openLibrary opens a shared library on Windows using LoadLibrary.
func openLibrary(name string) (uintptr, error) {
	h, err := syscall.LoadLibrary(name)
	if err != nil {
		return 0, fmt.Errorf("LoadLibrary %s: %w", name, err)
	}
	return uintptr(h), nil
}
