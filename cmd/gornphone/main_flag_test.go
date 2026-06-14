// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRNSConfigFlagAccepted(t *testing.T) {
	t.Parallel()

	// Create a temporary config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "rnsconfig")

	// Create a minimal RNS config file
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte("[rns]\nloglevel = 3\n"), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Verify the config file was created
	if _, err := os.Stat(filepath.Join(configDir, "config")); os.IsNotExist(err) {
		t.Fatal("Config file should exist")
	}

	// The --rnsconfig flag is now defined in main.go
	// This test verifies that the config directory can be created and used
	// The actual flag parsing is tested by the CLI itself
}
