// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends.
package platforms

import (
	"errors"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/oto/v3"
)

var (
	ErrOtoNotInitialized = errors.New("oto context not initialized")
	ErrOtoNoDevice       = errors.New("no audio device available")
)

const (
	otoDefaultSampleRate   = 48000
	otoDefaultChannels     = 2
	otoDefaultFormat       = oto.FormatFloat32LE
	otoDefaultBufferFrames = 1024
)

// OtoBackend implements AudioBackend using github.com/ebitengine/oto/v3
type OtoBackend struct {
	sampleRate int
	channels   int
	bitDepth   int

	mu         sync.Mutex
	ctx        *oto.Context
	ready      chan struct{}
	recorder   *otoRecorder
	player     *otoPlayer
	recorderMu sync.Mutex
	playerMu   sync.Mutex

	// Device info cache
	micNames     []string
	speakerNames []string
}

// otoRecorder wraps oto for recording (input)
// Uses a pipe to capture audio data from oto's input stream
type otoRecorder struct {
	sampleRate    int
	channels      int
	framesPerRead int
	closed        bool
	mu            sync.Mutex
	pipeReader    *io.PipeReader
	pipeWriter    *io.PipeWriter
	readBuf       []byte
}

// otoPlayer wraps oto for playback (output)
// Uses a pipe to feed audio data to oto's output stream
type otoPlayer struct {
	player     *oto.Player
	closed     bool
	mu         sync.Mutex
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

// NewOtoBackend creates a new Oto-based audio backend.
func NewOtoBackend(sampleRate, channels, bitDepth int) AudioBackend {
	if sampleRate <= 0 {
		sampleRate = otoDefaultSampleRate
	}
	if channels <= 0 {
		channels = otoDefaultChannels
	}
	if channels > 2 {
		channels = 2 // Oto only supports 1 or 2 channels
	}
	// Bit depth is informational; oto uses float32 internally

	ob := &OtoBackend{
		sampleRate: sampleRate,
		channels:   channels,
		bitDepth:   bitDepth,
		ready:      make(chan struct{}),
	}

	// Initialize oto context in background
	go ob.initContext()

	return ob
}

func (ob *OtoBackend) initContext() {
	op := &oto.NewContextOptions{
		SampleRate:   ob.sampleRate,
		ChannelCount: ob.channels,
		Format:       otoDefaultFormat,
	}

	ctx, readyChan, err := oto.NewContext(op)
	if err != nil {
		close(ob.ready)
		return
	}

	ob.mu.Lock()
	ob.ctx = ctx
	ob.mu.Unlock()

	// Wait for context to be ready
	<-readyChan
	close(ob.ready)

	// Cache device names (oto doesn't provide enumeration directly,
	// so we use generic names)
	ob.mu.Lock()
	ob.micNames = []string{"default"}
	ob.speakerNames = []string{"default"}
	ob.mu.Unlock()
}

// waitReady waits for the oto context to be initialized.
func (ob *OtoBackend) waitReady() error {
	select {
	case <-ob.ready:
		ob.mu.Lock()
		defer ob.mu.Unlock()
		if ob.ctx == nil {
			return ErrOtoNotInitialized
		}
		return nil
	case <-time.After(5 * time.Second):
		return ErrOtoNotInitialized
	}
}

func (ob *OtoBackend) SampleRate() int { return ob.sampleRate }
func (ob *OtoBackend) Channels() int   { return ob.channels }
func (ob *OtoBackend) BitDepth() int   { return ob.bitDepth }

func (ob *OtoBackend) AllMicrophones() []string {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.micNames
}

func (ob *OtoBackend) DefaultMicrophone() string {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if len(ob.micNames) > 0 {
		return ob.micNames[0]
	}
	return "default"
}

func (ob *OtoBackend) AllSpeakers() []string {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.speakerNames
}

func (ob *OtoBackend) DefaultSpeaker() string {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if len(ob.speakerNames) > 0 {
		return ob.speakerNames[0]
	}
	return "default"
}

func (ob *OtoBackend) Flush() error {
	// Oto doesn't have explicit flush; players handle buffering
	return nil
}

func (ob *OtoBackend) ReleaseRecorder() error {
	ob.recorderMu.Lock()
	defer ob.recorderMu.Unlock()

	if ob.recorder != nil {
		ob.recorder.closed = true
		ob.recorder = nil
	}
	return nil
}

func (ob *OtoBackend) ReleasePlayer() error {
	ob.playerMu.Lock()
	defer ob.playerMu.Unlock()

	if ob.player != nil {
		_ = ob.player.Close()
		ob.player = nil
	}
	return nil
}

func (ob *OtoBackend) GetRecorder(samplesPerFrame int) (AudioRecorder, error) {
	if err := ob.waitReady(); err != nil {
		return nil, err
	}

	ob.recorderMu.Lock()
	defer ob.recorderMu.Unlock()

	if ob.recorder != nil {
		return nil, errors.New("recorder already in use")
	}

	// Create pipe for recording
	pr, pw := io.Pipe()

	ob.recorder = &otoRecorder{
		sampleRate:    ob.sampleRate,
		channels:      ob.channels,
		framesPerRead: samplesPerFrame,
		pipeReader:    pr,
		pipeWriter:    pw,
		readBuf:       make([]byte, samplesPerFrame*2*4), // stereo float32
	}

	return ob.recorder, nil
}

func (ob *OtoBackend) GetPlayer(samplesPerFrame int, lowLatency bool) (AudioPlayer, error) {
	if err := ob.waitReady(); err != nil {
		return nil, err
	}

	ob.playerMu.Lock()
	defer ob.playerMu.Unlock()

	if ob.player != nil {
		return nil, errors.New("player already in use")
	}

	ob.mu.Lock()
	ctx := ob.ctx
	ob.mu.Unlock()

	if ctx == nil {
		return nil, ErrOtoNotInitialized
	}

	// Create pipe for playback
	pr, pw := io.Pipe()

	// Create player with pipe reader
	player := ctx.NewPlayer(pr)

	ob.player = &otoPlayer{
		player:     player,
		pipeReader: pr,
		pipeWriter: pw,
	}

	return ob.player, nil
}

// otoRecorder implementation

func (or *otoRecorder) Record(numFrames int) ([][]float32, error) {
	or.mu.Lock()
	if or.closed || or.pipeReader == nil {
		or.mu.Unlock()
		return nil, errors.New("recorder closed")
	}
	// Take a local copy of pipeReader under the lock
	pipeReader := or.pipeReader
	or.mu.Unlock()

	// Read audio data from pipe (using local copy to avoid race with Close)
	bytesPerSample := 4 // float32
	bytesPerFrame := or.channels * bytesPerSample
	totalBytes := numFrames * bytesPerFrame

	// Read data with timeout protection
	n, err := io.ReadFull(pipeReader, or.readBuf[:totalBytes])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// If the pipe was closed, return silence
		frame := make([][]float32, numFrames)
		for i := range frame {
			frame[i] = make([]float32, or.channels)
		}
		return frame, nil
	}

	// If we got no data, return silence
	if n == 0 {
		frame := make([][]float32, numFrames)
		for i := range frame {
			frame[i] = make([]float32, or.channels)
		}
		return frame, nil
	}

	// Convert bytes to float32 frames
	frame := make([][]float32, numFrames)
	frameCount := n / bytesPerFrame
	for i := 0; i < frameCount; i++ {
		frame[i] = make([]float32, or.channels)
		for ch := 0; ch < or.channels; ch++ {
			offset := i*bytesPerFrame + ch*bytesPerSample
			if offset+3 < len(or.readBuf) {
				// Convert little-endian bytes to float32
				bits := uint32(or.readBuf[offset]) |
					uint32(or.readBuf[offset+1])<<8 |
					uint32(or.readBuf[offset+2])<<16 |
					uint32(or.readBuf[offset+3])<<24
				frame[i][ch] = float32frombits(bits)
			}
		}
	}

	// Pad remaining frames with silence
	for i := frameCount; i < numFrames; i++ {
		frame[i] = make([]float32, or.channels)
	}

	return frame, nil
}

func (or *otoRecorder) Close() error {
	or.mu.Lock()
	defer or.mu.Unlock()

	or.closed = true

	// Close pipe writer first to unblock any pending Read
	if or.pipeWriter != nil {
		_ = or.pipeWriter.Close()
		or.pipeWriter = nil
	}
	if or.pipeReader != nil {
		_ = or.pipeReader.Close()
		or.pipeReader = nil
	}
	return nil
}

// otoPlayer implementation

func (op *otoPlayer) Play(frame [][]float32) error {
	op.mu.Lock()
	if op.closed || op.pipeWriter == nil {
		op.mu.Unlock()
		return errors.New("player closed")
	}
	pw := op.pipeWriter
	op.mu.Unlock()

	if len(frame) == 0 || len(frame[0]) == 0 {
		return nil
	}

	// Convert float32 frames to interleaved little-endian bytes
	numFrames := len(frame)
	numChannels := len(frame[0])
	bytesPerSample := 4 // float32
	bytesPerFrame := numChannels * bytesPerSample

	buf := make([]byte, numFrames*bytesPerFrame)
	for i := 0; i < numFrames; i++ {
		for ch := 0; ch < numChannels; ch++ {
			val := frame[i][ch]
			bits := float32bits(val)
			offset := i*bytesPerFrame + ch*bytesPerSample
			buf[offset] = byte(bits)
			buf[offset+1] = byte(bits >> 8)
			buf[offset+2] = byte(bits >> 16)
			buf[offset+3] = byte(bits >> 24)
		}
	}

	_, err := pw.Write(buf)
	return err
}

func (op *otoPlayer) Close() error {
	// Close pipeWriter first to unblock any pending Write call
	op.mu.Lock()
	pw := op.pipeWriter
	op.pipeWriter = nil
	op.mu.Unlock()

	if pw != nil {
		_ = pw.Close()
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	if op.player != nil {
		op.player.Pause()
		op.player = nil
	}
	if op.pipeReader != nil {
		_ = op.pipeReader.Close()
		op.pipeReader = nil
	}
	op.closed = true
	return nil
}

func (op *otoPlayer) EnableLowLatency() error {
	// Oto handles latency via buffer size in context options
	return nil
}

// float32bits converts float32 to its bit representation
func float32bits(f float32) uint32 {
	return *(*uint32)(unsafe.Pointer(&f))
}

// float32frombits converts bit representation to float32
func float32frombits(b uint32) float32 {
	return *(*float32)(unsafe.Pointer(&b))
}
