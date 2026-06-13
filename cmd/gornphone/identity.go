// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/gmlewis/go-reticulum/rns"
)

// loadOrCreateIdentity loads an identity from a file path, or creates a
// new one if the file does not exist. The new identity is saved to disk.
func loadOrCreateIdentity(path string) (*rns.Identity, error) {
	id, err := rns.FromFile(path, nil)
	if err == nil && id != nil {
		return id, nil
	}

	id, err = rns.NewIdentity(true, nil)
	if err != nil {
		return nil, fmt.Errorf("creating identity: %w", err)
	}

	if err := id.ToFile(path); err != nil {
		return nil, fmt.Errorf("saving identity: %w", err)
	}

	return id, nil
}
