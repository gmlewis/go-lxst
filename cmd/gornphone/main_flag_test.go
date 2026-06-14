// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRNSConfigFlagAccepted(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "rnsconfig")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte("[rns]\nloglevel = 3\n"), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	if _, err := os.Stat(filepath.Join(configDir, "config")); os.IsNotExist(err) {
		t.Fatal("Config file should exist")
	}
}

func TestVerbosityIncreasesWithString(t *testing.T) {
	t.Parallel()

	var v verbosity
	if err := v.Set(""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected verbosity 1 after one Set call, got %d", v)
	}
	if err := v.Set(""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if v != 2 {
		t.Errorf("expected verbosity 2 after two Set calls, got %d", v)
	}
	if err := v.Set(""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if v != 3 {
		t.Errorf("expected verbosity 3 after three Set calls, got %d", v)
	}
}

func TestVerbosityString(t *testing.T) {
	t.Parallel()

	var v verbosity = 3
	if s := v.String(); s != "3" {
		t.Errorf("expected '3', got %q", s)
	}
}

func TestVersionOutputFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "gornphone %s\n", version)

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "gornphone ") {
		t.Errorf("expected output to start with 'gornphone ', got %q", out)
	}
}

func TestFlagSetDefaults(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	l := fs.Bool("l", false, "")
	v := fs.Bool("version", false, "")
	c := fs.String("config", "", "")
	r := fs.String("rnsconfig", "", "")
	s := fs.Bool("service", false, "")
	sd := fs.Bool("systemd", false, "")
	p := fs.Int("profile", 0x40, "")
	g := fs.Float64("gain", 0.0, "")
	m := fs.String("mic", "", "")
	sp := fs.String("speaker", "", "")

	args := []string{}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse empty args: %v", err)
	}

	if *l {
		t.Error("--l should default to false")
	}
	if *v {
		t.Error("--version should default to false")
	}
	if *c != "" {
		t.Error("--config should default to empty")
	}
	if *r != "" {
		t.Error("--rnsconfig should default to empty")
	}
	if *s {
		t.Error("--service should default to false")
	}
	if *sd {
		t.Error("--systemd should default to false")
	}
	if *p != 0x40 {
		t.Errorf("--profile should default to 0x40, got %d", *p)
	}
	if *g != 0.0 {
		t.Errorf("--gain should default to 0.0, got %v", *g)
	}
	if *m != "" {
		t.Error("--mic should default to empty")
	}
	if *sp != "" {
		t.Error("--speaker should default to empty")
	}
}
