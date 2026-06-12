// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package sinks provides audio sink implementations for the LXST library.
// Sinks are endpoints that consume audio frames — such as LineSink for
// speaker playback and OpusFileSink for writing to Opus-encoded audio files.
// Each sink implements the Sink interface and the LocalSource interface
// for pipeline integration.
package sinks

import (
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// Sink is the base interface for audio sinks.
type Sink interface {
	HandleFrame(frame [][]float32, fromSource sources.Source) error
	CanReceive(fromSource sources.Source) bool
}

// RemoteSink implements a network-oriented audio sink.
type RemoteSink struct{}

func (r *RemoteSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (r *RemoteSink) CanReceive(fromSource sources.Source) bool {
	return true
}

// LocalSink implements a local audio sink that receives from local sources.
type LocalSink struct{}

func (l *LocalSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (l *LocalSink) CanReceive(fromSource sources.Source) bool {
	return true
}
