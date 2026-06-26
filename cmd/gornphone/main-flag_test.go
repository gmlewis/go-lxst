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

	"github.com/gmlewis/go-lxst/testutils"
)

func TestRNSConfigFlagAccepted(t *testing.T) {
	t.Parallel()

	tmpDir := testutils.TempDir(t, "gornphone-test-")
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
		t.Errorf("expected verbosity 1 after one Set call, got %v", v)
	}
	if err := v.Set(""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if v != 2 {
		t.Errorf("expected verbosity 2 after two Set calls, got %v", v)
	}
	if err := v.Set(""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if v != 3 {
		t.Errorf("expected verbosity 3 after three Set calls, got %v", v)
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
	fmt.Fprintf(&buf, "gornphone %v\n", version)

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
		t.Errorf("--profile should default to 0x40, got %v", *p)
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

func TestExpandVerboseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "single -v unchanged",
			in:   []string{"-v"},
			want: []string{"-v"},
		},
		{
			name: "-vv expands to two -v",
			in:   []string{"-vv"},
			want: []string{"-v", "-v"},
		},
		{
			name: "-vvv expands to three -v",
			in:   []string{"-vvv"},
			want: []string{"-v", "-v", "-v"},
		},
		{
			name: "-vvvv expands to four -v",
			in:   []string{"-vvvv"},
			want: []string{"-v", "-v", "-v", "-v"},
		},
		{
			name: "mixed flags",
			in:   []string{"-vvv", "-l", "--config", "/tmp/x"},
			want: []string{"-v", "-v", "-v", "-l", "--config", "/tmp/x"},
		},
		{
			name: "passthrough non-v flags",
			in:   []string{"-profile", "0x40", "-gain", "3.5"},
			want: []string{"-profile", "0x40", "-gain", "3.5"},
		},
		{
			name: "empty args",
			in:   []string{},
			want: []string{},
		},
		{
			name: "-vvvv with other args",
			in:   []string{"gornphone", "-vvvv", "--version"},
			want: []string{"gornphone", "-v", "-v", "-v", "-v", "--version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandVerboseArgs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v args, want %v: got=%v want=%v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%v]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestVerbosityWithFlagParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no flags", []string{}, 0},
		{"-v", []string{"-v"}, 1},
		{"-vv", []string{"-vv"}, 2},
		{"-vvv", []string{"-vvv"}, 3},
		{"-vvvv", []string{"-vvvv"}, 4},
		{"-v -v -v", []string{"-v", "-v", "-v"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := expandVerboseArgs(tt.args)
			var v verbosity
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.Var(&v, "v", "verbosity")
			if err := fs.Parse(args); err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if int(v) != tt.want {
				t.Errorf("verbosity = %v, want %v", v, tt.want)
			}
		})
	}
}
