// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package vorbis

import (
	"embed"
	"io"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/test.ogg
var testVorbis embed.FS

func TestVorbisFileSource_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewVorbisFileSource("/nonexistent/file.ogg", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestVorbisFileSource_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := NewVorbisFileSource("", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for empty path")
	}
}

func TestDecodeVorbis_InvalidData(t *testing.T) {
	t.Parallel()

	_, _, _, err := DecodeVorbis(io.NopCloser(newZeroReader(100)))
	if err == nil {
		t.Fatal("Expected error for invalid Vorbis data")
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
	dir, err := os.MkdirTemp("/tmp", "go-lxst-vorbis-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func writeTestVorbis(t *testing.T) string {
	t.Helper()
	tmpDir := tempDir(t)
	oggPath := filepath.Join(tmpDir, "test.ogg")

	data, err := testVorbis.ReadFile("testdata/test.ogg")
	if err != nil {
		t.Fatalf("Failed to read embedded test Vorbis: %v", err)
	}

	err = os.WriteFile(oggPath, data, 0o644)
	if err != nil {
		t.Fatalf("Failed to write test Vorbis: %v", err)
	}

	return oggPath
}

func TestVorbisFileSource_ValidFile(t *testing.T) {
	t.Parallel()

	oggPath := writeTestVorbis(t)

	src, err := NewVorbisFileSource(oggPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewVorbisFileSource failed: %v", err)
	}

	if src.SampleRate() != 44100 {
		t.Errorf("Expected sample rate 44100, got %d", src.SampleRate())
	}
	if src.Channels() <= 0 {
		t.Errorf("Expected positive channels, got %d", src.Channels())
	}
	if src.SampleCount() <= 0 {
		t.Errorf("Expected positive sample count, got %d", src.SampleCount())
	}
	if src.LengthMs() <= 0 {
		t.Errorf("Expected positive length, got %f ms", src.LengthMs())
	}
}

func TestVorbisFileSource_StartStop(t *testing.T) {
	t.Parallel()

	oggPath := writeTestVorbis(t)

	src, err := NewVorbisFileSource(oggPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewVorbisFileSource failed: %v", err)
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

func TestVorbisFileSource_Samples(t *testing.T) {
	t.Parallel()

	oggPath := writeTestVorbis(t)

	src, err := NewVorbisFileSource(oggPath, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewVorbisFileSource failed: %v", err)
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
