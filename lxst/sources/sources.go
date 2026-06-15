// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package sources provides audio source implementations for the LXST library.
// Sources produce audio frames — including LineSource for microphone input,
// OpusFileSource for reading Opus-encoded audio files, and Loopback for
// connecting pipeline outputs back as inputs. Each source implements the
// LocalSource interface for pipeline integration with configurable codecs.
package sources

import (
	"errors"
	"sync"

	"github.com/gmlewis/go-lxst/lxst/codecs"
)

// Common errors
var (
	ErrInvalidCodec         = errors.New("invalid codec")
	ErrSourceNotRunning     = errors.New("source not running")
	ErrSourceAlreadyRunning = errors.New("source already running")
)

const (
	LoopbackMaxFrames = 128
)

type Source interface {
	Start() error
	Stop() error
	Running() bool
}

// LocalSource is the interface for audio sources that can receive frames
// from other sources in a pipeline. It extends Source with frame handling
// methods. In the LXST architecture, HandleFrame receives unencoded
// [][]float32 frames (receive path), while HandleEncodedFrame receives
// already-encoded []byte data (transmit path from Mixer with codec).
type LocalSource interface {
	Source
	HandleFrame(frame [][]float32, fromSource Source) error
	HandleEncodedFrame(data []byte, fromSource Source) error
	CanReceive(fromSource Source) bool
}

type RemoteSource interface {
	Source
}

type Loopback struct {
	mu         sync.Mutex
	frameDeque [][]float32
	shouldRun  bool
	codec      codecs.Codec
	sink       LocalSource
	source     Source
	maxFrames  int
}

func NewLoopback(codec codecs.Codec, sink LocalSource) *Loopback {
	return &Loopback{
		codec:     codec,
		sink:      sink,
		maxFrames: LoopbackMaxFrames,
	}
}

func (l *Loopback) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.shouldRun {
		return ErrSourceAlreadyRunning
	}
	l.shouldRun = true
	return nil
}

func (l *Loopback) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shouldRun = false
	return nil
}

func (l *Loopback) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.shouldRun
}

func (l *Loopback) CanReceive(fromSource Source) bool {
	if l.sink == nil {
		return true
	}
	return l.sink.CanReceive(fromSource)
}

func (l *Loopback) HandleFrame(frame [][]float32, fromSource Source) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.shouldRun {
		return ErrSourceNotRunning
	}

	if l.codec == nil || l.sink == nil {
		return nil
	}

	if len(frame) == 0 {
		return nil
	}

	if l.sink.CanReceive(l) {
		return l.sink.HandleFrame(frame, l)
	}
	return nil
}

// HandleEncodedFrame forwards already-encoded audio data to this
// loopback's sink, matching the Python pattern where encoded bytes
// flow through the pipeline after codec encoding at the source.
func (l *Loopback) HandleEncodedFrame(data []byte, fromSource Source) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.shouldRun {
		return ErrSourceNotRunning
	}

	if l.sink == nil {
		return nil
	}

	if len(data) == 0 {
		return nil
	}

	if l.sink.CanReceive(l) {
		return l.sink.HandleEncodedFrame(data, l)
	}
	return nil
}

func (l *Loopback) SetSource(src Source) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.source = src
}

func (l *Loopback) GetSource() Source {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.source
}

// Sink returns the current output destination for this loopback.
func (l *Loopback) Sink() LocalSource {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sink
}

// SetSink sets the output destination for this loopback, matching the
// Python LocalSource.sink property setter.
func (l *Loopback) SetSink(sink LocalSource) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sink = sink
}
