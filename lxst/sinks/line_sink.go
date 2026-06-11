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

	ls.digestThread = &digestThreadInfo{
		done: make(chan struct{}),
	}
	ls.digestThread.wg.Add(1)
	go ls.digestJob()

	return nil
}

func (ls *LineSink) Stop() error {
	ls.mu.Lock()
	ls.shouldRun = false
	ls.mu.Unlock()
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

func (ls *LineSink) digestJob() {
	defer ls.digestThread.wg.Done()

	ls.digestLock.Lock()
	defer ls.digestLock.Unlock()

	backendSPF := ls.samplesPerFrame
	player, err := ls.backend.GetPlayer(backendSPF, ls.lowLatency)
	if err != nil {
		return
	}
	ls.player = player
	defer func() {
		player.Close()
		ls.backend.ReleasePlayer()
	}()

	for {
		select {
		case <-ls.digestThread.done:
			return
		default:
		}

		if !ls.shouldRun {
			return
		}

		ls.insertLock.Lock()
		framesReady := len(ls.frameDeque)
		ls.insertLock.Unlock()

		if framesReady > 0 {
			ls.insertLock.Lock()
			ls.outputLatency = float64(len(ls.frameDeque)) * ls.frameTime
			ls.maxLatency = float64(ls.bufferMaxHeight) * ls.frameTime
			ls.underrunAt = nil

			var frame [][]float32
			if len(ls.frameDeque) > 0 {
				frame = ls.frameDeque[0]
				ls.frameDeque = ls.frameDeque[1:]
			}
			ls.insertLock.Unlock()

			if len(frame) > 0 {
				if len(frame[0]) > ls.channels {
					for i := range frame {
						frame[i] = frame[i][:ls.channels]
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
			if ls.underrunAt == nil {
				now := time.Now()
				ls.underrunAt = &now
			} else {
				if time.Since(*ls.underrunAt).Seconds() > ls.frameTime*float64(ls.frameTimeout) {
					ls.shouldRun = false
					return
				}
				time.Sleep(time.Duration(ls.frameTime * float64(time.Second) * 0.1))
			}
		}

		if ls.wantsLowLatency {
			ls.wantsLowLatency = false
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
