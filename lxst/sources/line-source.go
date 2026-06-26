// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package sources provides audio source implementations.
package sources

import (
	"errors"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/platforms"
)

var (
	ErrInvalidSampleRate = errors.New("invalid sample rate")
	ErrNoBackend         = errors.New("no audio backend available")
)

const (
	LineSourceMaxFrames      = 128
	LineSourceDefaultFrameMs = 80.0
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
	skipCompleted bool
	skipStartTime time.Time
}

type threadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

func NewLineSource(preferredDevice string, targetFrameMs float64, codec codecs.Codec, sink LocalSource, filterChain []filters.Filter, gain, easeIn, skip float64) *LineSource {
	log.Printf("LineSource.NewLineSink: preferredDevice=%v, targetFrameMs=%.1f, codec=%T, sink=%T", preferredDevice, targetFrameMs, codec, sink)
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

	// Get platform backend with preferred device
	ls.backend = platforms.NewBackendWithDevice(48000, 2, 32, ls.preferredDevice)
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

	thread := &threadInfo{
		done: make(chan struct{}),
	}
	ls.ingestThread = thread
	thread.wg.Add(1)
	go ls.ingestJobWithThread(thread)

	return nil
}

func (ls *LineSource) Stop() error {
	ls.mu.Lock()
	running := ls.shouldRun
	if !running {
		ls.mu.Unlock()
		return nil
	}

	ls.shouldRun = false

	var thread *threadInfo
	if ls.ingestThread != nil {
		thread = ls.ingestThread
		ls.ingestThread = nil
		close(thread.done)
	}

	recorder := ls.recorder
	ls.recorder = nil
	backend := ls.backend
	ls.mu.Unlock()

	// Close recorder to unblock any pending Record() calls in ingestJob,
	// so the goroutine can check the done channel and exit.
	if recorder != nil {
		if err := recorder.Close(); err != nil {
			log.Printf("LineSource.Stop: recorder.Close failed: %v", err)
		}
	}
	if backend != nil {
		if err := backend.ReleaseRecorder(); err != nil {
			log.Printf("LineSource.Stop: backend.ReleaseRecorder failed: %v", err)
		}
	}

	if thread != nil {
		thread.wg.Wait()
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

func (ls *LineSource) ingestJobWithThread(thread *threadInfo) {
	defer thread.wg.Done()

	ls.recordingLock.Lock()
	defer ls.recordingLock.Unlock()

	ls.mu.Lock()
	recorder := ls.recorder
	samplesPerFrame := ls.samplesPerFrame
	ls.mu.Unlock()

	for {
		select {
		case <-thread.done:
			return
		default:
		}

		ls.mu.Lock()
		shouldRun := ls.shouldRun
		ls.mu.Unlock()

		if !shouldRun {
			return
		}

		if recorder == nil {
			log.Printf("LineSource.ingestJob: recorder is nil, stopping")
			return
		}

		frame, err := recorder.Record(samplesPerFrame)
		if err != nil {
			log.Printf("LineSource.ingestJob: recorder.Record failed: %v", err)
			continue
		}

		ls.mu.Lock()
		skipCompleted := ls.skipCompleted
		skipStartTime := ls.skipStartTime
		skipDuration := ls.skip
		filterChain := ls.filterChain
		currentGain := ls.currentGain
		easeInCompleted := ls.easeInCompleted
		easeInDuration := ls.easeIn
		targetGain := ls.targetGain
		codec := ls.codec
		sink := ls.sink
		ls.mu.Unlock()

		if !skipCompleted {
			if time.Since(skipStartTime).Seconds() > skipDuration {
				ls.mu.Lock()
				ls.skipCompleted = true
				ls.skipStartTime = time.Now()
				ls.mu.Unlock()
			} else {
				continue
			}
		}

		// Apply filters
		for _, f := range filterChain {
			frame = f.HandleFrame(frame, ls.samplerate)
		}

		// Apply gain
		if currentGain != 1.0 {
			for i := range frame {
				for ch := range frame[i] {
					frame[i][ch] *= float32(currentGain)
				}
			}
		}

		// Apply ease-in
		if !easeInCompleted && easeInDuration > 0 {
			elapsed := time.Since(skipStartTime).Seconds()
			newGain := (elapsed / easeInDuration) * targetGain
			if newGain >= targetGain {
				newGain = targetGain
				ls.mu.Lock()
				ls.easeInCompleted = true
				ls.mu.Unlock()
			}
			ls.mu.Lock()
			ls.currentGain = newGain
			ls.mu.Unlock()
		}

		// Encode and send to sink
		if codec != nil && !codecs.IsNullCodec(codec) && sink != nil {
			encoded := codec.Encode(frame)
			if len(encoded) > 0 && sink.CanReceive(ls) {
				if err := sink.HandleEncodedFrame(encoded, ls); err != nil {
					log.Printf("LineSource.ingestJob: HandleEncodedFrame failed: %v", err)
				}
			}
		} else if sink != nil {
			if err := sink.HandleFrame(frame, ls); err != nil {
				log.Printf("LineSource.ingestJob: HandleFrame failed: %v", err)
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

// PreferredDevice returns the preferred audio input device name.
func (ls *LineSource) PreferredDevice() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.preferredDevice
}

// AvailableMicrophones returns the list of available microphone device names
// from the audio backend.
func (ls *LineSource) AvailableMicrophones() []string {
	ls.mu.Lock()
	backend := ls.backend
	ls.mu.Unlock()
	if backend == nil {
		return nil
	}
	return backend.AllMicrophones()
}

// HandleEncodedFrame is not used by LineSource in normal pipeline
// operation. LineSource is an audio input that generates frames from
// a microphone and does not receive incoming encoded data.
func (ls *LineSource) HandleEncodedFrame(data []byte, fromSource Source) error {
	return nil
}

// Sink returns the current output destination for this LineSource.
func (ls *LineSource) Sink() LocalSource {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.sink
}

// SetSink sets the output destination for this LineSource, matching
// the Python LocalSource.sink property setter.
func (ls *LineSource) SetSink(sink LocalSource) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.sink = sink
}
