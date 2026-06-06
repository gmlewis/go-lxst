// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package players

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func createTestWavForPlayer(t *testing.T, sampleRate, numChannels, sampleCount int, frequency float64) string {
	t.Helper()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.wav")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer f.Close()

	dataSize := sampleCount * numChannels * 2
	byteRate := uint32(sampleRate) * uint32(numChannels) * 2
	blockAlign := uint16(numChannels) * 2

	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(36+dataSize))
	f.Write([]byte("WAVE"))

	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(numChannels))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))
	binary.Write(f, binary.LittleEndian, uint32(byteRate))
	binary.Write(f, binary.LittleEndian, uint16(blockAlign))
	binary.Write(f, binary.LittleEndian, uint16(16))

	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))

	for i := 0; i < sampleCount; i++ {
		for ch := 0; ch < numChannels; ch++ {
			phase := 2.0 * math.Pi * frequency * float64(i) / float64(sampleRate)
			sample := int16(16000.0 * math.Sin(phase))
			binary.Write(f, binary.LittleEndian, sample)
		}
	}

	return path
}

func TestFilePlayer_New(t *testing.T) {
	t.Parallel()

	path := createTestWavForPlayer(t, 8000, 1, 8000, 440.0)

	fp, err := NewFilePlayer(path, "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}
	if fp == nil {
		t.Fatal("NewFilePlayer returned nil")
	}
}

func TestFilePlayer_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewFilePlayer("/nonexistent/path.wav", "", false)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFilePlayer_EmptyPath(t *testing.T) {
	t.Parallel()

	fp, err := NewFilePlayer("", "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer with empty path should succeed: %v", err)
	}
	if fp.Running() {
		t.Error("FilePlayer should not be running initially")
	}
}

func TestFilePlayer_SetSource(t *testing.T) {
	t.Parallel()

	fp, err := NewFilePlayer("", "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}

	path := createTestWavForPlayer(t, 8000, 1, 8000, 440.0)

	err = fp.SetSource(path)
	if err != nil {
		t.Fatalf("SetSource failed: %v", err)
	}
}

func TestFilePlayer_SetSource_Invalid(t *testing.T) {
	t.Parallel()

	fp, err := NewFilePlayer("", "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}

	err = fp.SetSource("/nonexistent/path.wav")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFilePlayer_SetLoop(t *testing.T) {
	t.Parallel()

	path := createTestWavForPlayer(t, 8000, 1, 8000, 440.0)

	fp, err := NewFilePlayer(path, "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}

	fp.SetLoop(true)
	if !fp.loop {
		t.Error("Expected loop=true after SetLoop(true)")
	}

	fp.SetLoop(false)
	if fp.loop {
		t.Error("Expected loop=false after SetLoop(false)")
	}
}

func TestFilePlayer_FinishedCallback(t *testing.T) {
	t.Parallel()

	path := createTestWavForPlayer(t, 8000, 1, 8000, 440.0)

	fp, err := NewFilePlayer(path, "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}

	var callbackCalled bool
	err = fp.SetFinishedCallback(func(p *FilePlayer) {
		callbackCalled = true
	})
	if err != nil {
		t.Fatalf("SetFinishedCallback failed: %v", err)
	}
	if fp.finishedCallback == nil {
		t.Error("Callback should be set")
	}
	_ = callbackCalled
}

func TestFilePlayer_FilePath(t *testing.T) {
	t.Parallel()

	path := createTestWavForPlayer(t, 8000, 1, 8000, 440.0)

	fp, err := NewFilePlayer(path, "", false)
	if err != nil {
		t.Fatalf("NewFilePlayer failed: %v", err)
	}

	if fp.FilePath() != path {
		t.Errorf("Expected filePath %s, got %s", path, fp.FilePath())
	}
}