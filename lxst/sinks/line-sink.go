// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sinks

import (
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

func NewLineSink(preferredDevice string, autodigest bool, lowLatency bool) *LineSink {
	backend := platforms.NewBackendWithDevice(48000, 2, 32, preferredDevice)

	ls := &LineSink{
		preferredDevice:     preferredDevice,
		shouldRun:           false,
		frameDeque:          make([][][]float32, 0, LineSinkMaxFrames),
		frameTimeout:        LineSinkFrameTimeout,
		autodigest:          autodigest,
		autostartMin:        LineSinkAutostartMin,
		bufferMaxHeight:     LineSinkMaxFrames - 3,
		lowLatency:          lowLatency,
		preferredSamplerate: 48000,
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
		ls.frameTime = float64(ls.samplesPerFrame) / float64(ls.samplerate)
	}
	dequeLen := len(ls.frameDeque)
	ls.insertLock.Unlock()

	ls.mu.Lock()
	shouldStart := ls.autodigest && !ls.shouldRun && dequeLen >= ls.autostartMin
	ls.mu.Unlock()

	if shouldStart {
		_ = ls.Start()
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
		_ = player.Close()
		_ = ls.backend.ReleasePlayer()
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

	ls.mu.Lock()
	backendSPF := ls.samplesPerFrame
	lowLatency := ls.lowLatency
	ls.mu.Unlock()

	player, err := ls.backend.GetPlayer(backendSPF, lowLatency)
	if err != nil {
		return
	}
	ls.mu.Lock()
	ls.player = player
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
				_ = player.Play(frame)
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
			_ = player.EnableLowLatency()
		}
	}
}

// Ensure LineSink implements sources.LocalSource
var _ sources.LocalSource = (*LineSink)(nil)

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
