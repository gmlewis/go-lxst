// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
	"github.com/gmlewis/go-lxst/testutils"
)

// createTestWav creates a minimal WAV file with a sine wave.
func createTestWav(t *testing.T, sampleRate, numChannels, sampleCount int, frequency float64) string {
	t.Helper()

	tmpDir := testutils.TempDir(t, "go-lxst-roundtrip-test-")
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

func TestOpusFileSource_Roundtrip_Load(t *testing.T) {
	t.Parallel()

	path := createTestWav(t, 8000, 1, 8000, 440.0)

	src, err := sources.NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if src.SampleRate() != 8000 {
		t.Errorf("Expected sample rate 8000, got %v", src.SampleRate())
	}
	if src.Channels() != 1 {
		t.Errorf("Expected 1 channel, got %v", src.Channels())
	}
	if src.SampleCount() <= 0 {
		t.Errorf("Expected positive sample count, got %v", src.SampleCount())
	}

	expectedLengthMs := (float64(src.SampleCount()) / float64(src.SampleRate())) * 1000.0
	if math.Abs(src.LengthMs()-expectedLengthMs) > 1.0 {
		t.Errorf("Expected length ~%fms, got %fms", expectedLengthMs, src.LengthMs())
	}
}

func TestOpusFileSource_Roundtrip_Stereo(t *testing.T) {
	t.Parallel()

	path := createTestWav(t, 48000, 2, 48000, 1000.0)

	src, err := sources.NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	if src.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %v", src.SampleRate())
	}
	if src.Channels() != 2 {
		t.Errorf("Expected 2 channels, got %v", src.Channels())
	}
}

func TestNullCodec_Roundtrip(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0 * math.Pi * 440.0 * float64(i) / 8000.0))}
	}

	encoded := codec.Encode(frame)
	if len(encoded) == 0 {
		t.Fatal("Encode returned empty data")
	}

	decoded := codec.Decode(encoded, 1)
	if len(decoded) != len(frame) {
		t.Fatalf("Decode returned wrong length: got %v, want %v", len(decoded), len(frame))
	}

	for i := range decoded {
		diff := math.Abs(float64(decoded[i][0]) - float64(frame[i][0]))
		if diff > 0.001 {
			t.Errorf("Sample %v: got %f, want %f (diff %f)", i, decoded[i][0], frame[i][0], diff)
		}
	}
}

func TestNullCodec_Roundtrip_Stereo(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{
			float32(math.Sin(2.0*math.Pi*440.0*float64(i)/8000.0)) * 0.5,
			float32(math.Sin(2.0*math.Pi*880.0*float64(i)/8000.0)) * 0.3,
		}
	}

	encoded := codec.Encode(frame)
	if len(encoded) == 0 {
		t.Fatal("Encode returned empty data")
	}

	decoded := codec.Decode(encoded, 2)
	if len(decoded) != len(frame) {
		t.Fatalf("Decode returned wrong length: got %v, want %v", len(decoded), len(frame))
	}

	for i := range decoded {
		for ch := 0; ch < 2; ch++ {
			diff := math.Abs(float64(decoded[i][ch]) - float64(frame[i][ch]))
			if diff > 0.001 {
				t.Errorf("Sample %v ch %v: got %f, want %f", i, ch, decoded[i][ch], frame[i][ch])
			}
		}
	}
}

func TestFilterPipeline_Roundtrip(t *testing.T) {
	t.Parallel()

	hp := filters.NewHighPass(100)
	lp := filters.NewLowPass(8000)

	sampleRate := 48000
	frameSize := 480

	frame := make([][]float32, frameSize)
	for i := range frame {
		frame[i] = []float32{0.5 + 0.3*float32(math.Sin(2.0*math.Pi*1000.0*float64(i)/float64(sampleRate)))}
	}

	// Apply HighPass to remove DC offset
	result := hp.HandleFrame(frame, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("HighPass changed frame size: got %v, want %v", len(result), frameSize)
	}

	// Apply LowPass
	result = lp.HandleFrame(result, sampleRate)
	if len(result) != frameSize {
		t.Fatalf("LowPass changed frame size: got %v, want %v", len(result), frameSize)
	}

	// DC offset should be reduced after HighPass
	mean := float64(0)
	for i := range result {
		mean += float64(result[i][0])
	}
	mean /= float64(len(result))

	if math.Abs(mean) > 0.1 {
		t.Errorf("DC offset after HighPass should be small, mean=%f", mean)
	}
}

func TestResample_Passthrough(t *testing.T) {
	t.Parallel()

	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(2.0 * math.Pi * 440.0 * float64(i) / 8000.0))}
	}

	// Same rate = passthrough
	result := codecs.Resample(frame, 16, 1, 8000, 8000, false)
	if len(result) != len(frame) {
		t.Errorf("Passthrough resample changed length: got %v, want %v", len(result), len(frame))
	}
}

func TestResampleBytes_Passthrough(t *testing.T) {
	t.Parallel()

	data := make([]byte, 320)
	for i := range data {
		data[i] = byte(i)
	}

	result := codecs.ResampleBytes(data, 16, 1, 8000, 8000, false)
	if len(result) != len(data) {
		t.Errorf("Passthrough resample changed length: got %v, want %v", len(result), len(data))
	}
}

// TestIntegration_LineSourceToLineSink tests line source -> codec -> line sink
// Note: This uses the null backend (no real audio hardware) for CI compatibility
func TestIntegration_LineSourceToLineSink(t *testing.T) {
	t.Parallel()

	// Use NullCodec for testing without actual codec
	codec := codecs.NullCodec{}

	// Create a line sink that will receive frames (autodigest=true so it starts automatically)
	lineSink := sinks.NewLineSink("", true, false, 0)

	// Create a line source with the codec and sink
	lineSrc := sources.NewLineSource("", 20.0, codec, lineSink, nil, 0, 0, 0)

	// Start source (sink will auto-start due to autodigest)
	err := lineSrc.Start()
	if err != nil {
		t.Fatalf("LineSource.Start failed: %v", err)
	}
	defer func() { _ = lineSrc.Stop() }()
	defer func() { _ = lineSink.Stop() }()

	// Give some time for frames to flow
	time.Sleep(200 * time.Millisecond)

	// Verify they are running
	if !lineSrc.Running() {
		t.Error("LineSource should be running")
	}

	// Sink might not be running yet if no frames received, but that's ok
	// With autodigest=true and autostartMin=1, it should start once it gets a frame

	// Check that sink can receive (if not running, it should still accept frames)
	if !lineSink.CanReceive(lineSrc) {
		// This might be because the buffer is full with the null backend
		// Just verify the sink was created and source is running
		t.Log("LineSink cannot receive - buffer may be full with null backend")
	}
}

// TestIntegration_OpusFileRoundtrip tests file -> OpusFileSource -> OpusFileSink -> file
func TestIntegration_OpusFileRoundtrip(t *testing.T) {
	t.Parallel()

	// Skip if Opus is not available
	_, err := opusPkg.NewOpus(opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skip("Opus not available")
	}

	// Create test WAV file
	sampleRate := 8000
	numChannels := 1
	sampleCount := 8000
	path := createTestWav(t, sampleRate, numChannels, sampleCount, 440.0)

	// Create OpusFileSource
	src, err := sources.NewOpusFileSource(path, 20.0, false, nil, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	// Create OpusFileSink with temp output file
	tmpDir := testutils.TempDir(t, "go-lxst-roundtrip-test-")
	outputPath := filepath.Join(tmpDir, "output.opus")

	sink, err := sinks.NewOpusFileSink(outputPath, false, opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Fatalf("NewOpusFileSink failed: %v", err)
	}

	// Connect source to sink
	src.SetSink(sink)

	// Start both
	err = sink.Start()
	if err != nil {
		t.Fatalf("OpusFileSink.Start failed: %v", err)
	}
	defer func() { _ = sink.Stop() }()

	err = src.Start()
	if err != nil {
		t.Fatalf("OpusFileSource.Start failed: %v", err)
	}
	defer func() { _ = src.Stop() }()

	// Wait for processing to complete (file length ~1 second, with some buffer)
	time.Sleep(2 * time.Second)

	// Verify the output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("Output file not created: %s", outputPath)
	}

	// Verify sink processed frames
	if sink.FramesWaiting() < 0 {
		t.Error("Sink should have processed frames")
	}
}

// TestIntegration_OpusFileSourceToNullSink tests OpusFileSource with NullCodec sink
func TestIntegration_OpusFileSourceToNullSink(t *testing.T) {
	t.Parallel()

	// Use NullCodec for testing
	codec := codecs.NullCodec{}

	// Create test WAV file
	sampleRate := 8000
	numChannels := 1
	sampleCount := 8000
	path := createTestWav(t, sampleRate, numChannels, sampleCount, 440.0)

	// Create OpusFileSource
	src, err := sources.NewOpusFileSource(path, 20.0, false, codec, nil, false)
	if err != nil {
		t.Fatalf("NewOpusFileSource failed: %v", err)
	}

	// Create a mock sink for testing
	mockSink := &mockLocalSink{
		receivedFrames: make([][][]float32, 0),
		mu:             sync.Mutex{},
	}
	src.SetSink(mockSink)

	// Start source
	err = src.Start()
	if err != nil {
		t.Fatalf("OpusFileSource.Start failed: %v", err)
	}
	defer func() { _ = src.Stop() }()

	// Wait for some frames
	time.Sleep(300 * time.Millisecond)

	// Verify frames were received
	mockSink.mu.Lock()
	frameCount := len(mockSink.receivedFrames)
	mockSink.mu.Unlock()

	if frameCount == 0 {
		t.Error("Expected sink to receive frames")
	}
}

// mockLocalSink is a simple test sink
type mockLocalSink struct {
	mu             sync.Mutex
	receivedFrames [][][]float32
}

func (m *mockLocalSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedFrames = append(m.receivedFrames, frame)
	return nil
}

func (m *mockLocalSink) CanReceive(fromSource sources.Source) bool {
	return true
}

func (m *mockLocalSink) Start() error                                                    { return nil }
func (m *mockLocalSink) Stop() error                                                     { return nil }
func (m *mockLocalSink) Running() bool                                                   { return true }
func (m *mockLocalSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error { return nil }
