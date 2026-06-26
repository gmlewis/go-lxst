// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gmlewis/go-reticulum/rns"
)

func TestLoadOrCreateIdentity_New(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	identityPath := filepath.Join(dir, "identity")

	id, err := loadOrCreateIdentity(identityPath)
	if err != nil {
		t.Fatalf("loadOrCreateIdentity() error = %v", err)
	}
	if id == nil {
		t.Fatal("loadOrCreateIdentity() returned nil identity")
	}
	if len(id.Hash) == 0 {
		t.Error("identity hash is empty")
	}
	if id.HexHash == "" {
		t.Error("identity hex hash is empty")
	}

	info, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("identity file not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("identity file permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadOrCreateIdentity_Existing(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	identityPath := filepath.Join(dir, "identity")

	id1, err := loadOrCreateIdentity(identityPath)
	if err != nil {
		t.Fatalf("first loadOrCreateIdentity() error = %v", err)
	}

	id2, err := loadOrCreateIdentity(identityPath)
	if err != nil {
		t.Fatalf("second loadOrCreateIdentity() error = %v", err)
	}

	if id1.HexHash != id2.HexHash {
		t.Errorf("identity hash mismatch: %q != %q", id1.HexHash, id2.HexHash)
	}
}

func TestIdentityToFileRoundTrip(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	path1 := filepath.Join(dir, "id1")
	path2 := filepath.Join(dir, "id2")

	id1, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}

	if err := id1.ToFile(path1); err != nil {
		t.Fatalf("ToFile() error = %v", err)
	}

	id2, err := rns.FromFile(path1, nil)
	if err != nil {
		t.Fatalf("FromFile() error = %v", err)
	}

	if err := id2.ToFile(path2); err != nil {
		t.Fatalf("ToFile() error = %v", err)
	}

	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if len(data1) != len(data2) {
		t.Errorf("key data length mismatch: %v != %v", len(data1), len(data2))
	}

	for i := range data1 {
		if data1[i] != data2[i] {
			t.Errorf("key data mismatch at byte %v", i)
			break
		}
	}
}

func TestIdentityHashConsistency(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	path := filepath.Join(dir, "identity")

	id, err := loadOrCreateIdentity(path)
	if err != nil {
		t.Fatalf("loadOrCreateIdentity() error = %v", err)
	}

	hash1 := make([]byte, len(id.Hash))
	copy(hash1, id.Hash)

	id2, err := loadOrCreateIdentity(path)
	if err != nil {
		t.Fatalf("loadOrCreateIdentity() error = %v", err)
	}

	if len(hash1) != len(id2.Hash) {
		t.Fatalf("hash length mismatch: %v != %v", len(hash1), len(id2.Hash))
	}
	for i := range hash1 {
		if hash1[i] != id2.Hash[i] {
			t.Errorf("hash mismatch at byte %v: %x != %x", i, hash1[i], id2.Hash[i])
			break
		}
	}
}
