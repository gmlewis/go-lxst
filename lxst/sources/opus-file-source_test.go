// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sources

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/testutils"
)

func createTestWavFile(t *testing.T, sampleRate, numChannels, sampleCount int, frequency float64) string {
	t.Helper()

	tmpDir := testutils.TempDir(t, "go-lxst-source-test-")
	path := filepath.Join(tmpDir, "test.wav")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer func() { _ = f.Close() }()

	dataSize := sampleCount * numChannels * 2
	byteRate := uint32(sampleRate) * uint32(numChannels) * 2
	blockAlign := uint16(numChannels) * 2

	if _, err := f.Write([]byte("RIFF")); err != nil {
		t.Fatalf("write RIFF header: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatalf("write file size: %v", err)
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		t.Fatalf("write WAVE header: %v", err)
	}

	if _, err := f.Write([]byte("fmt ")); err != nil {
		t.Fatalf("write fmt chunk: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatalf("write fmt size: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatalf("write audio format: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		t.Fatalf("write channels: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatalf("write sample rate: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		t.Fatalf("write byte rate: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		t.Fatalf("write block align: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(16)); err != nil {
		t.Fatalf("write bits per sample: %v", err)
	}

	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatalf("write data chunk: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatalf("write data size: %v", err)
	}

	for i := 0; i < sampleCount; i++ {
		for ch := 0; ch < numChannels; ch++ {
			phase := 2.0 * math.Pi * frequency * float64(i) / float64(sampleRate)
			sample := int16(16000.0 * math.Sin(phase))
			if err := binary.Write(f, binary.LittleEndian, sample); err != nil {
				t.Fatalf("write sample: %v", err)
			}
		}
	}

	return path
}

func TestOpusFileSource_Load(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	src, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if src.SampleRate() != 8000 {
		t.Errorf("Expected sample rate 8000, got %d", src.SampleRate())
	}
	if src.Channels() != 1 {
		t.Errorf("Expected 1 channel, got %d", src.Channels())
	}
	if src.SampleCount() <= 0 {
		t.Errorf("Expected positive sample count, got %d", src.SampleCount())
	}
}

func TestOpusFileSource_StereoLoad(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 48000, 2, 48000, 1000.0)

	src, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if src.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", src.SampleRate())
	}
	if src.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %d", src.Channels())
	}
}

func TestOpusFileSource_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewOpusFileSource("/nonexistent/path.wav", 20.0, false, nil, nil, false)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestOpusFileSource_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := NewOpusFileSource("", 20.0, false, nil, nil, false)
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestOpusFileSource_Loop(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	src, err := NewOpusFileSource(path, 20.0, true, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if !src.Loop() {
		t.Error("Expected loop=true")
	}

	src2, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}
	if src2.Loop() {
		t.Error("Expected loop=false")
	}
}

func TestOpusFileSource_SamplesPerFrame(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	src, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	expectedSPF := 160
	if src.SamplesPerFrame() != expectedSPF {
		t.Errorf("Expected %d samples per frame, got %d", expectedSPF, src.SamplesPerFrame())
	}
}

func TestOpusFileSource_CodecSetterConstraints(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	opus, err := opusPkg.NewOpus(opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	src, err := NewOpusFileSource(path, 21.0, false, opus, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource with codec failed: %v", err)
	}

	// 21.0 → quantized to 22.5 (quanta), then clamped to 20 (closest valid frame)
	if math.Abs(src.TargetFrameMs()-20.0) > 0.001 {
		t.Errorf("Expected frame time quantized to 20.0ms, got %f", src.TargetFrameMs())
	}
}

func TestOpusFileSource_FrameTime(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	src, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	expectedFrameTime := float64(src.SamplesPerFrame()) / float64(src.SampleRate())
	if math.Abs(src.FrameTime()-expectedFrameTime) > 0.0001 {
		t.Errorf("Expected frame time %f, got %f", expectedFrameTime, src.FrameTime())
	}
}

func TestOpusFileSource_LengthMs(t *testing.T) {
	t.Parallel()

	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	src, err := NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if src.LengthMs() <= 0 {
		t.Errorf("Expected positive length, got %f", src.LengthMs())
	}
}
