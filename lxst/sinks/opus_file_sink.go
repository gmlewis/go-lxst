// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
	"errors"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

var (
	ErrNoOutputPath       = errors.New("no recording file path configured")
	ErrOpusSinkNotRunning = errors.New("opus file sink not running")
)

const (
	OpusFileSinkMaxFrames      = 64
	OpusFileSinkAutostartMin   = 1
	OpusFileSinkFinalizeTimeout = 2.0
	TypeMapFactor              = 32767.0
)

type OpusFileSink struct {
	mu                sync.Mutex
	insertLock        sync.Mutex
	digestLock        sync.Mutex
	shouldRun         bool
	digestThread      *digestThreadInfo
	frameDeque        [][][]float32
	autodigest        bool
	autostartMin      int
	bufferMaxHeight   int
	profile           int
	bitdepth          int
	samplerate        int
	outputSamplerate  int
	channels          int
	maxBytesPerFrame  int
	samplesPerFrame   int
	frameTime         float64
	outputLatency     float64
	maxLatency        float64
	samplesWritten    int
	recordingStopped  bool
	finalized         bool
	outputPath        string
	opusEncoder       *opusPkg.Opus
	preferredSamplerate int
}

func NewOpusFileSink(path string, autodigest bool, profile int) (*OpusFileSink, error) {
	opus, err := opusPkg.NewOpus(profile)
	if err != nil {
		return nil, err
	}

	sr, ch, _, _ := opusPkg.ProfileConfig(profile)

	fs := &OpusFileSink{
		outputPath:       path,
		autodigest:       autodigest,
		autostartMin:     OpusFileSinkAutostartMin,
		bufferMaxHeight:  OpusFileSinkMaxFrames,
		profile:          profile,
		bitdepth:         32,
		channels:         ch,
		outputSamplerate: sr,
		opusEncoder:       opus,
		frameDeque:       make([][][]float32, 0, OpusFileSinkMaxFrames),
	}

	return fs, nil
}

func (fs *OpusFileSink) FramesWaiting() int {
	fs.insertLock.Lock()
	defer fs.insertLock.Unlock()
	return len(fs.frameDeque)
}

func (fs *OpusFileSink) CanReceive(fromSource sources.Source) bool {
	fs.insertLock.Lock()
	defer fs.insertLock.Unlock()
	if fs.recordingStopped {
		return false
	}
	return len(fs.frameDeque) < fs.bufferMaxHeight
}

func (fs *OpusFileSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	fs.insertLock.Lock()
	fs.frameDeque = append(fs.frameDeque, frame)

	if fs.samplesPerFrame == 0 && len(frame) > 0 {
		if src, ok := fromSource.(interface{ SampleRate() int }); ok {
			fs.samplerate = src.SampleRate()
		}
		fs.samplesPerFrame = len(frame)
		fs.frameTime = float64(fs.samplesPerFrame) / float64(fs.samplerate)
		if len(frame) > 0 && len(frame[0]) > fs.channels {
			fs.channels = len(frame[0])
		}
	}

	dequeLen := len(fs.frameDeque)
	fs.insertLock.Unlock()

	fs.mu.Lock()
	shouldStart := fs.autodigest && !fs.shouldRun && dequeLen >= fs.autostartMin
	fs.mu.Unlock()

	if shouldStart {
		_ = fs.Start()
	}

	return nil
}

func (fs *OpusFileSink) Start() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.shouldRun {
		return nil
	}

	fs.shouldRun = true

	fs.digestThread = &digestThreadInfo{
		done: make(chan struct{}),
	}
	fs.digestThread.wg.Add(1)
	go fs.digestJob()

	return nil
}

func (fs *OpusFileSink) Stop() error {
	fs.mu.Lock()
	if fs.shouldRun {
		fs.recordingStopped = true
	}
	fs.mu.Unlock()

	timeout := time.Now().Add(time.Duration(OpusFileSinkFinalizeTimeout * float64(time.Second)))
	for time.Now().Before(timeout) {
		fs.insertLock.Lock()
		dequeLen := len(fs.frameDeque)
		fs.insertLock.Unlock()
		if dequeLen == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	fs.mu.Lock()
	fs.shouldRun = false
	fs.mu.Unlock()

	return nil
}

func (fs *OpusFileSink) Running() bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.shouldRun
}

func (fs *OpusFileSink) Profile() int {
	return fs.profile
}

func (fs *OpusFileSink) SampleRate() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.samplerate
}

func (fs *OpusFileSink) OutputSamplerate() int {
	return fs.outputSamplerate
}

func (fs *OpusFileSink) Channels() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.channels
}

func (fs *OpusFileSink) SamplesPerFrame() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.samplesPerFrame
}

func (fs *OpusFileSink) digestJob() {
	defer fs.digestThread.wg.Done()

	fs.digestLock.Lock()
	defer fs.digestLock.Unlock()

	finalSilenceFrames := 10

	for {
		select {
		case <-fs.digestThread.done:
			return
		default:
		}

		if !fs.shouldRun && finalSilenceFrames <= 0 {
			fs.finalized = true
			return
		}

		fs.insertLock.Lock()
		framesReady := len(fs.frameDeque)
		fs.insertLock.Unlock()

		processFrame := false
		var frame [][]float32

		if fs.shouldRun && framesReady > 0 {
			fs.insertLock.Lock()
			fs.outputLatency = float64(len(fs.frameDeque)) * fs.frameTime
			fs.maxLatency = float64(fs.bufferMaxHeight) * fs.frameTime

			if len(fs.frameDeque) > 0 {
				frame = fs.frameDeque[0]
				fs.frameDeque = fs.frameDeque[1:]
			}
			fs.insertLock.Unlock()
			processFrame = true
		} else if !fs.shouldRun && finalSilenceFrames > 0 {
			finalSilenceFrames--
			frame = make([][]float32, fs.samplesPerFrame)
			for i := range frame {
				frame[i] = make([]float32, fs.channels)
			}
			processFrame = true
		}

		if processFrame && len(frame) > 0 {
			if len(frame[0]) > fs.channels {
				for i := range frame {
					frame[i] = frame[i][:fs.channels]
				}
			} else if len(frame[0]) < fs.channels {
				for i := range frame {
					for j := len(frame[i]); j < fs.channels; j++ {
						frame[i] = append(frame[i], frame[i][len(frame[i])-1])
					}
				}
			}

			if len(frame) < fs.samplesPerFrame {
				for i := len(frame); i < fs.samplesPerFrame; i++ {
					frame = append(frame, make([]float32, fs.channels))
				}
			}

			fs.samplesWritten += len(frame)

			if fs.samplerate != 0 && fs.samplerate != fs.outputSamplerate {
				frame = codecs.Resample(frame, fs.bitdepth, fs.channels, fs.samplerate, fs.outputSamplerate, false)
			}

			if fs.opusEncoder != nil {
				_ = fs.opusEncoder.Encode(frame)
			}
		} else {
			time.Sleep(time.Duration(fs.frameTime * float64(time.Second) * 0.1))
		}

		fs.insertLock.Lock()
		if len(fs.frameDeque) > fs.bufferMaxHeight {
			fs.frameDeque = fs.frameDeque[1:]
		}
		fs.insertLock.Unlock()
	}
}

// Ensure OpusFileSink implements sources.LocalSource
var _ sources.LocalSource = (*OpusFileSink)(nil)