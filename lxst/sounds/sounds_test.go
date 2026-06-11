// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sounds

import (
	"bytes"
	"io"
	"testing"
)

func TestGetSound_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{name: "ringer"},
		{name: "soft"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := GetSound(tc.name)
			if err != nil {
				t.Fatalf("GetSound(%q) returned error: %v", tc.name, err)
			}
			if reader == nil {
				t.Fatalf("GetSound(%q) returned nil reader", tc.name)
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			if len(data) == 0 {
				t.Errorf("GetSound(%q) returned empty data", tc.name)
			}

			if !bytes.HasPrefix(data, []byte("OggS")) {
				t.Errorf("GetSound(%q) data does not start with OggS header", tc.name)
			}
		})
	}
}

func TestGetSound_Invalid(t *testing.T) {
	t.Parallel()

	_, err := GetSound("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent sound, got nil")
	}
}

func TestListSounds(t *testing.T) {
	t.Parallel()

	names := ListSounds()
	if len(names) < 2 {
		t.Errorf("ListSounds returned %d names, expected at least 2", len(names))
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["ringer"] {
		t.Error("ListSounds missing 'ringer'")
	}
	if !found["soft"] {
		t.Error("ListSounds missing 'soft'")
	}
}

func TestGetSound_ReaderUsable(t *testing.T) {
	t.Parallel()

	reader, err := GetSound("ringer")
	if err != nil {
		t.Fatalf("GetSound returned error: %v", err)
	}

	buf := make([]byte, 512)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n == 0 {
		t.Error("Read returned 0 bytes")
	}
}

func TestGetSound_MultipleReads(t *testing.T) {
	t.Parallel()

	r1, err := GetSound("soft")
	if err != nil {
		t.Fatalf("GetSound returned error: %v", err)
	}
	r2, err := GetSound("soft")
	if err != nil {
		t.Fatalf("GetSound returned error: %v", err)
	}

	d1, _ := io.ReadAll(r1)
	d2, _ := io.ReadAll(r2)

	if !bytes.Equal(d1, d2) {
		t.Error("Multiple reads of same sound returned different data")
	}
	if len(d1) == 0 {
		t.Error("Sound data is empty")
	}
}
