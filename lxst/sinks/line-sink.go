// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
	"log"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/platforms"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

const (
	LineSinkMaxFrames    = 6
	LineSinkAutostartMin = 1
	LineSinkFrameTimeout = 8
)

type LineSink struct {
	mu                  sync.Mutex
	insertLock          sync.Mutex
	digestLock          sync.Mutex
	preferredDevice     string
	shouldRun           bool
	digestThread        *digestThreadInfo
	frameDeque          [][][]float32
	underrunAt          *time.Time
	frameTimeout        int
	autodigest          bool
	autostartMin        int
	bufferMaxHeight     int
	lowLatency          bool
	preferredSamplerate int
	backend             platforms.AudioBackend
	player              platforms.AudioPlayer
	samplerate          int
	channels            int
	samplesPerFrame     int
	frameTime           float64
	outputLatency       float64
	maxLatency          float64
	wantsLowLatency     bool
}

type digestThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

func NewLineSink(preferredDevice string, autodigest bool, lowLatency bool, sampleRate int) *LineSink {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	backend := platforms.NewBackendWithDevice(sampleRate, 2, 32, preferredDevice)
	log.Printf("LineSink.NewLineSink: preferredDevice=%v, autodigest=%v, sampleRate=%d, backend=%T (%p)", preferredDevice, autodigest, sampleRate, backend, backend)

	ls := &LineSink{
		preferredDevice:     preferredDevice,
		shouldRun:           false,
		frameDeque:          make([][][]float32, 0, LineSinkMaxFrames),
		frameTimeout:        LineSinkFrameTimeout,
		autodigest:          autodigest,
		autostartMin:        LineSinkAutostartMin,
		bufferMaxHeight:     LineSinkMaxFrames - 3,
		lowLatency:          lowLatency,
		preferredSamplerate: sampleRate,
		backend:             backend,
		channels:            2,
	}

	if backend != nil {
		ls.samplerate = backend.SampleRate()
		if speaker := backend.DefaultSpeaker(); speaker != "" {
			ls.channels = 2
		}
	}

	return ls
}

func (ls *LineSink) CanReceive(fromSource sources.Source) bool {
	ls.insertLock.Lock()
	defer ls.insertLock.Unlock()
	return len(ls.frameDeque) < ls.bufferMaxHeight
}

func (ls *LineSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	ls.insertLock.Lock()
	ls.frameDeque = append(ls.frameDeque, frame)

	if ls.samplesPerFrame == 0 && len(frame) > 0 {
		ls.samplesPerFrame = len(frame)
		if ls.samplerate > 0 {
			ls.frameTime = float64(ls.samplesPerFrame) / float64(ls.samplerate)
		}
		log.Printf("LineSink.HandleFrame: first frame detected: samples=%d, channels=%d, backendRate=%d, frameTime=%.4f",
			ls.samplesPerFrame, len(frame[0]), ls.samplerate, ls.frameTime)
	}
	dequeLen := len(ls.frameDeque)
	ls.insertLock.Unlock()

	ls.mu.Lock()
	shouldStart := ls.autodigest && !ls.shouldRun && dequeLen >= ls.autostartMin
	ls.mu.Unlock()

	if shouldStart {
		if err := ls.Start(); err != nil {
			log.Printf("LineSink.HandleFrame: Start failed: %v", err)
		}
	}

	return nil
}

func (ls *LineSink) Start() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.shouldRun {
		return nil
	}

	ls.shouldRun = true

	thread := &digestThreadInfo{
		done: make(chan struct{}),
	}
	ls.digestThread = thread
	thread.wg.Add(1)
	go ls.digestJobWithThread(thread)

	return nil
}

func (ls *LineSink) Stop() error {
	ls.mu.Lock()
	if !ls.shouldRun {
		ls.mu.Unlock()
		return nil
	}
	ls.shouldRun = false

	var thread *digestThreadInfo
	player := ls.player
	ls.player = nil
	if ls.digestThread != nil {
		thread = ls.digestThread
		ls.digestThread = nil
		close(thread.done)
	}
	ls.mu.Unlock()

	// Close the player to unblock any pending Play() calls in digestJob,
	// so the goroutine can check the done channel and exit.
	if player != nil {
		if err := player.Close(); err != nil {
			log.Printf("LineSink.Stop: player.Close failed: %v", err)
		}
		if err := ls.backend.ReleasePlayer(); err != nil {
			log.Printf("LineSink.Stop: backend.ReleasePlayer failed: %v", err)
		}
	}

	if thread != nil {
		thread.wg.Wait()
	}
	return nil
}

func (ls *LineSink) Running() bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.shouldRun
}

func (ls *LineSink) EnableLowLatency() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.wantsLowLatency = true
}

func (ls *LineSink) OutputLatency() float64 {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.outputLatency
}

func (ls *LineSink) MaxLatency() float64 {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.maxLatency
}

func (ls *LineSink) SampleRate() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.samplerate
}

func (ls *LineSink) Channels() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.channels
}

func (ls *LineSink) SamplesPerFrame() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.samplesPerFrame
}

func (ls *LineSink) digestJobWithThread(thread *digestThreadInfo) {
	defer thread.wg.Done()

	ls.digestLock.Lock()
	defer ls.digestLock.Unlock()

	ls.insertLock.Lock()
	backendSPF := ls.samplesPerFrame
	ls.insertLock.Unlock()

	ls.mu.Lock()
	lowLatency := ls.lowLatency
	ls.mu.Unlock()

	player, err := ls.backend.GetPlayer(backendSPF, lowLatency)
	if err != nil {
		log.Printf("LineSink.digestJob: GetPlayer failed (spf=%d, lowLatency=%v, backend=%T (%p)): %v", backendSPF, lowLatency, ls.backend, ls.backend, err)
		return
	}
	log.Printf("LineSink.digestJob: GetPlayer succeeded (spf=%d, backend=%T (%p))", backendSPF, ls.backend, ls.backend)
	ls.mu.Lock()
	ls.player = player
	ls.mu.Unlock()

	// Ensure the player is closed and released when the digestJob exits,
	// whether due to normal shutdown, underrun timeout, or thread.done.
	// Without this, a subsequent autodigest restart would fail with
	// "player already in use" because the old player was never released.
	defer func() {
		ls.mu.Lock()
		ls.player = nil
		ls.mu.Unlock()
		if err := player.Close(); err != nil {
			log.Printf("LineSink.digestJob: player.Close on exit failed: %v", err)
		}
		if err := ls.backend.ReleasePlayer(); err != nil {
			log.Printf("LineSink.digestJob: backend.ReleasePlayer on exit failed: %v", err)
		}
	}()

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

		ls.insertLock.Lock()
		framesReady := len(ls.frameDeque)
		ls.insertLock.Unlock()

		if framesReady > 0 {
			ls.insertLock.Lock()
			ls.mu.Lock()
			ls.outputLatency = float64(len(ls.frameDeque)) * ls.frameTime
			ls.maxLatency = float64(ls.bufferMaxHeight) * ls.frameTime
			ls.underrunAt = nil
			channels := ls.channels
			ls.mu.Unlock()

			var frame [][]float32
			if len(ls.frameDeque) > 0 {
				frame = ls.frameDeque[0]
				ls.frameDeque = ls.frameDeque[1:]
			}
			ls.insertLock.Unlock()

			if len(frame) > 0 {
				if len(frame[0]) > channels {
					for i := range frame {
						frame[i] = frame[i][:channels]
					}
				}
				if err := player.Play(frame); err != nil {
					log.Printf("LineSink.digestJob: player.Play failed: %v", err)
				}
			}

			ls.insertLock.Lock()
			if len(ls.frameDeque) > ls.bufferMaxHeight {
				ls.frameDeque = ls.frameDeque[1:]
			}
			ls.insertLock.Unlock()
		} else {
			ls.mu.Lock()
			underrunAt := ls.underrunAt
			frameTimeout := ls.frameTimeout
			frameTime := ls.frameTime
			ls.mu.Unlock()

			if underrunAt == nil {
				now := time.Now()
				ls.mu.Lock()
				ls.underrunAt = &now
				ls.mu.Unlock()
			} else {
				if time.Since(*underrunAt).Seconds() > frameTime*float64(frameTimeout) {
					ls.mu.Lock()
					ls.shouldRun = false
					ls.mu.Unlock()
					return
				}
				time.Sleep(time.Duration(frameTime * float64(time.Second) * 0.1))
			}
		}

		ls.mu.Lock()
		wantsLowLatency := ls.wantsLowLatency
		ls.wantsLowLatency = false
		ls.mu.Unlock()

		if wantsLowLatency {
			if err := player.EnableLowLatency(); err != nil {
				log.Printf("LineSink.digestJob: EnableLowLatency failed: %v", err)
			}
		}
	}
}

// Ensure LineSink implements sources.LocalSource
var _ sources.LocalSource = (*LineSink)(nil)

// HandleEncodedFrame handles already-encoded audio data. In the normal
// LXST pipeline, LineSink receives unencoded float32 frames from the
// receive Mixer (which has a Null codec). If encoded data arrives here,
// it is dropped since LineSink has no codec to decode it.
func (ls *LineSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

// PreferredDevice returns the preferred audio output device name.
func (ls *LineSink) PreferredDevice() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.preferredDevice
}

// AvailableSpeakers returns the list of available speaker device names
// from the audio backend.
func (ls *LineSink) AvailableSpeakers() []string {
	ls.mu.Lock()
	backend := ls.backend
	ls.mu.Unlock()
	if backend == nil {
		return nil
	}
	return backend.AllSpeakers()
}
