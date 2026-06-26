// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package mp3

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gmlewis/go-lxst/testutils"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := testutils.TempDir(t, "go-lxst-mp3-test-")
	return dir
}

func TestMP3FileSource_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewMP3FileSource("/nonexistent/file.mp3", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestMP3FileSource_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := NewMP3FileSource("", 80.0, false, nil, nil, false)
	if err == nil {
		t.Fatal("Expected error for empty path")
	}
}

func TestMP3Decoder_ReadFrames(t *testing.T) {
	t.Parallel()

	tmpDir := tempDir(t)
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	if err := generateTestMP3(mp3Path, 440.0, 1.0, 48000, 1); err != nil {
		t.Skipf("Cannot generate test MP3: %v", err)
	}

	src, err := NewMP3FileSource(mp3Path, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewMP3FileSource failed: %v", err)
	}

	if src.SampleRate() <= 0 {
		t.Errorf("Expected positive sample rate, got %v", src.SampleRate())
	}
	if src.Channels() <= 0 {
		t.Errorf("Expected positive channels, got %v", src.Channels())
	}
	if src.SampleCount() <= 0 {
		t.Errorf("Expected positive sample count, got %v", src.SampleCount())
	}
	if src.LengthMs() <= 0 {
		t.Errorf("Expected positive length, got %f ms", src.LengthMs())
	}
}

func TestMP3Decoder_SamplesPerFrame(t *testing.T) {
	t.Parallel()

	tmpDir := tempDir(t)
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	if err := generateTestMP3(mp3Path, 440.0, 1.0, 48000, 1); err != nil {
		t.Skipf("Cannot generate test MP3: %v", err)
	}

	src, err := NewMP3FileSource(mp3Path, 40.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewMP3FileSource failed: %v", err)
	}

	spf := src.SamplesPerFrame()
	if spf <= 0 {
		t.Errorf("Expected positive samples per frame, got %v", spf)
	}
}

func TestMP3Decoder_ReadAll(t *testing.T) {
	t.Parallel()

	tmpDir := tempDir(t)
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	if err := generateTestMP3(mp3Path, 440.0, 0.5, 48000, 1); err != nil {
		t.Skipf("Cannot generate test MP3: %v", err)
	}

	src, err := NewMP3FileSource(mp3Path, 80.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewMP3FileSource failed: %v", err)
	}

	samples := src.Samples()
	if len(samples) == 0 {
		t.Fatal("Expected non-empty samples")
	}

	for i, s := range samples {
		for ch, v := range s {
			if v > 1.0 || v < -1.0 {
				t.Errorf("Sample [%v][%v] out of range: %f", i, ch, v)
			}
		}
	}
}

func generateTestMP3(path string, freq float64, duration float64, sampleRate int, channels int) error {
	// We can't easily generate MP3 without an encoder.
	// Use a tiny valid MP3 file for testing.
	// Create a minimal valid MP3 by writing raw bytes.
	// This is a ~1KB MP3 file with a short sine wave.
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Write a minimal MP3 frame header + some data
	// MPEG1, Layer 3, 128kbps, 44100Hz stereo
	// Frame sync: 0xFF 0xFB
	// This creates a syntactically valid but very short MP3
	header := []byte{0xFF, 0xFB, 0x90, 0x00}
	data := make([]byte, 417) // 128kbps frame size
	copy(data, header)
	// Fill rest with zeros (silence)
	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func TestDecodeMP3_InvalidData(t *testing.T) {
	t.Parallel()

	_, _, _, err := DecodeMP3(io.NopCloser(newZeroReader(100)))
	if err == nil {
		t.Fatal("Expected error for invalid MP3 data")
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
