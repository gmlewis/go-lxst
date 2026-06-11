// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package flac

import (
	"embed"
	"io"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/test.flac
var testFLAC embed.FS

func TestFLACFileSource_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewFLACFileSource("/nonexistent/file.flac", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestFLACFileSource_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := NewFLACFileSource("", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for empty path")
	}
}

func TestDecodeFLAC_InvalidData(t *testing.T) {
	t.Parallel()

	_, _, _, err := DecodeFLAC(io.NopCloser(newZeroReader(100)))
	if err == nil {
		t.Fatal("Expected error for invalid FLAC data")
	}
}

type zeroReader struct {
	remaining int
}

func newZeroReader(n int) *zeroReader {
	return &zeroReader{remaining: n}
}

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > z.remaining {
		n = z.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 0
	}
	z.remaining -= n
	return n, nil
}

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "go-lxst-flac-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeTestFLAC(t *testing.T) string {
	t.Helper()
	tmpDir := tempDir(t)
	flacPath := filepath.Join(tmpDir, "test.flac")

	data, err := testFLAC.ReadFile("testdata/test.flac")
	if err != nil {
		t.Fatalf("Failed to read embedded test FLAC: %v", err)
	}

	err = os.WriteFile(flacPath, data, 0o644)
	if err != nil {
		t.Fatalf("Failed to write test FLAC: %v", err)
	}

	return flacPath
}

func TestFLACFileSource_ValidFile(t *testing.T) {
	t.Parallel()

	flacPath := writeTestFLAC(t)

	src, err := NewFLACFileSource(flacPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewFLACFileSource failed: %v", err)
	}

	if src.SampleRate() != 44100 {
		t.Errorf("Expected sample rate 44100, got %d", src.SampleRate())
	}
	if src.Channels() != 1 {
		t.Errorf("Expected 1 channel, got %d", src.Channels())
	}
	if src.SampleCount() <= 0 {
		t.Errorf("Expected positive sample count, got %d", src.SampleCount())
	}
	if src.LengthMs() <= 0 {
		t.Errorf("Expected positive length, got %f ms", src.LengthMs())
	}
}

func TestFLACFileSource_StartStop(t *testing.T) {
	t.Parallel()

	flacPath := writeTestFLAC(t)

	src, err := NewFLACFileSource(flacPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewFLACFileSource failed: %v", err)
	}

	err = src.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = src.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestFLACFileSource_Samples(t *testing.T) {
	t.Parallel()

	flacPath := writeTestFLAC(t)

	src, err := NewFLACFileSource(flacPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewFLACFileSource failed: %v", err)
	}

	samples := src.Samples()
	if len(samples) == 0 {
		t.Fatal("Expected non-empty samples")
	}

	for i, s := range samples {
		for ch, v := range s {
			if v > 1.0 || v < -1.0 {
				t.Errorf("Sample [%d][%d] out of range: %f", i, ch, v)
			}
		}
	}
}

func TestFLACFileSource_SamplesPerFrame(t *testing.T) {
	t.Parallel()

	flacPath := writeTestFLAC(t)

	src, err := NewFLACFileSource(flacPath, 40.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewFLACFileSource failed: %v", err)
	}

	spf := src.SamplesPerFrame()
	if spf <= 0 {
		t.Errorf("Expected positive samples per frame, got %d", spf)
	}
}
