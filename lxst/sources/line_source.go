// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package sources provides audio source implementations.
package sources

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/platforms"
)

var (
	ErrInvalidSampleRate    = errors.New("invalid sample rate")
	ErrNoBackend            = errors.New("no audio backend available")
)

const (
	LineSourceMaxFrames       = 128
	LineSourceDefaultFrameMs  = 80.0
)

// LineSource implements audio input from a microphone/line input.
type LineSource struct {
	mu              sync.Mutex
	preferredDevice string
	targetFrameMs   float64
	samplerate      int
	channels        int
	bitdepth        int
	shouldRun       bool
	ingestThread    *threadInfo
	recordingLock   sync.Mutex
	codec           codecs.Codec
	sink            LocalSource
	filterChain     []filters.Filter
	easeIn          float64
	gain            float64
	skip            float64
	backend         platforms.AudioBackend
	recorder        platforms.AudioRecorder
	samplesPerFrame int
	frameTime       float64
	
	// Ease-in state
	easeInCompleted bool
	currentGain     float64
	targetGain      float64
	
	// Skip state
	skipCompleted   bool
	skipStartTime   time.Time
}

type threadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

func NewLineSource(preferredDevice string, targetFrameMs float64, codec codecs.Codec, sink LocalSource, filterChain []filters.Filter, gain, easeIn, skip float64) *LineSource {
	if targetFrameMs <= 0 {
		targetFrameMs = LineSourceDefaultFrameMs
	}
	
	ls := &LineSource{
		preferredDevice: preferredDevice,
		targetFrameMs:   targetFrameMs,
		codec:           codec,
		sink:            sink,
		filterChain:     filterChain,
		gain:            gain,
		easeIn:          easeIn,
		skip:            skip,
		targetGain:      math.Pow(10, gain/10.0),
		currentGain:     1.0,
	}
	
	if easeIn > 0 {
		ls.currentGain = 0.0
	}
	
	if skip > 0 {
		ls.skipCompleted = false
	} else {
		ls.skipCompleted = true
	}
	
	return ls
}

func (ls *LineSource) Start() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	if ls.shouldRun {
		return ErrSourceAlreadyRunning
	}
	
	// Get platform backend
	ls.backend = platforms.NewBackend(48000, 2, 32)
	if ls.backend == nil {
		return ErrNoBackend
	}
	
	// Apply codec frame constraints
	if ls.codec != nil {
		ls.applyCodecConstraints()
	}
	
	ls.samplerate = ls.backend.SampleRate()
	ls.channels = ls.backend.Channels()
	ls.bitdepth = 32 // Default to float32 internal
	ls.samplesPerFrame = int(math.Ceil((ls.targetFrameMs / 1000.0) * float64(ls.samplerate)))
	ls.frameTime = float64(ls.samplesPerFrame) / float64(ls.samplerate)
	
	// Get recorder
	var err error
	backendSamplesPerFrame := ls.samplesPerFrame
	// On Darwin, let the backend choose block size
	if platforms.GetBackend() == "darwin" {
		backendSamplesPerFrame = 0
	}
	ls.recorder, err = ls.backend.GetRecorder(backendSamplesPerFrame)
	if err != nil {
		return err
	}
	
	ls.shouldRun = true
	ls.easeInCompleted = (ls.easeIn <= 0)
	ls.skipCompleted = (ls.skip <= 0)
	ls.skipStartTime = time.Now()
	
	ls.ingestThread = &threadInfo{
		done: make(chan struct{}),
	}
	ls.ingestThread.wg.Add(1)
	go ls.ingestJob()
	
	return nil
}

func (ls *LineSource) Stop() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	running := ls.shouldRun
	if !running {
		return nil
	}
	
	ls.shouldRun = false
	
	if ls.ingestThread != nil {
		close(ls.ingestThread.done)
		ls.ingestThread.wg.Wait()
		ls.ingestThread = nil
	}
	
	if ls.recorder != nil {
		ls.recorder.Close()
		ls.recorder = nil
	}
	
	if ls.backend != nil {
		ls.backend.ReleaseRecorder()
	}
	
	return nil
}

func (ls *LineSource) Running() bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.shouldRun
}

func (ls *LineSource) applyCodecConstraints() {
	if ls.codec == nil {
		return
	}
	
	// Frame quanta
	if quanta := ls.codec.FrameQuantumMs(); quanta > 0 {
		if math.Mod(ls.targetFrameMs, quanta) != 0 {
			ls.targetFrameMs = math.Ceil(ls.targetFrameMs/quanta) * quanta
		}
	}
	
	// Frame max
	if maxMs := ls.codec.FrameMaxMs(); maxMs > 0 && ls.targetFrameMs > maxMs {
		ls.targetFrameMs = maxMs
	}
	
	// Valid frame times
	if valid := ls.codec.ValidFrameMs(); len(valid) > 0 {
		closest := valid[0]
		minDiff := math.Abs(ls.targetFrameMs - valid[0])
		for _, v := range valid[1:] {
			diff := math.Abs(ls.targetFrameMs - v)
			if diff < minDiff {
				minDiff = diff
				closest = v
			}
		}
		ls.targetFrameMs = closest
	}
	
	// Preferred sample rate
	if pref := ls.codec.PreferredSampleRate(); pref > 0 {
		ls.samplerate = pref
	}
}

func (ls *LineSource) ingestJob() {
	defer ls.ingestThread.wg.Done()
	
	ls.recordingLock.Lock()
	defer ls.recordingLock.Unlock()
	
	for {
		select {
		case <-ls.ingestThread.done:
			return
		default:
			if !ls.shouldRun {
				return
			}
			
			frame, err := ls.recorder.Record(ls.samplesPerFrame)
			if err != nil {
				continue
			}
			
			if !ls.skipCompleted {
				if time.Since(ls.skipStartTime).Seconds() > ls.skip {
					ls.skipCompleted = true
					ls.skipStartTime = time.Now()
				} else {
					continue
				}
			}
			
			// Apply filters
			for _, f := range ls.filterChain {
				frame = f.HandleFrame(frame, ls.samplerate)
			}
			
			// Apply gain
			if ls.currentGain != 1.0 {
				for i := range frame {
					for ch := range frame[i] {
						frame[i][ch] *= float32(ls.currentGain)
					}
				}
			}
			
			// Apply ease-in
			if !ls.easeInCompleted && ls.easeIn > 0 {
				elapsed := time.Since(ls.skipStartTime).Seconds()
				ls.currentGain = (elapsed / ls.easeIn) * ls.targetGain
				if ls.currentGain >= ls.targetGain {
					ls.currentGain = ls.targetGain
					ls.easeInCompleted = true
				}
			}
			
			// Encode and send to sink
			if ls.codec != nil && ls.sink != nil {
				encoded := ls.codec.Encode(frame)
				if len(encoded) > 0 && ls.sink.CanReceive(ls) {
					ls.sink.HandleFrame(frame, ls)
				}
			} else if ls.sink != nil {
				ls.sink.HandleFrame(frame, ls)
			}
		}
	}
}

// SetCodec sets the codec and applies constraints
func (ls *LineSource) SetCodec(codec codecs.Codec) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	if codec == nil {
		ls.codec = nil
		return nil
	}
	
	ls.codec = codec
	ls.applyCodecConstraints()
	
	// Recalculate samples per frame
	ls.samplesPerFrame = int(math.Ceil((ls.targetFrameMs / 1000.0) * float64(ls.samplerate)))
	ls.frameTime = float64(ls.samplesPerFrame) / float64(ls.samplerate)
	
	return nil
}

// GetCodec returns the current codec
func (ls *LineSource) GetCodec() codecs.Codec {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.codec
}

// GetSampleRate returns the sample rate
func (ls *LineSource) GetSampleRate() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.samplerate
}

// GetChannels returns the number of channels
func (ls *LineSource) GetChannels() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.channels
}