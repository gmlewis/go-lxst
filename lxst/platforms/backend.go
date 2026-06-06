// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package platforms provides platform-specific audio I/O backends.
package platforms

import (
	"errors"
)

var (
	ErrNotImplemented = errors.New("not implemented in null backend")
)

// AudioBackend defines the interface for platform-specific audio backends.
type AudioBackend interface {
	SampleRate() int
	Channels() int
	BitDepth() int
	AllMicrophones() []string
	DefaultMicrophone() string
	AllSpeakers() []string
	DefaultSpeaker() string
	Flush() error
	GetRecorder(samplesPerFrame int) (AudioRecorder, error)
	GetPlayer(samplesPerFrame int, lowLatency bool) (AudioPlayer, error)
	ReleaseRecorder() error
	ReleasePlayer() error
}

// AudioRecorder defines the interface for audio recording.
type AudioRecorder interface {
	Record(numFrames int) ([][]float32, error)
	Close() error
}

// AudioPlayer defines the interface for audio playback.
type AudioPlayer interface {
	Play(frame [][]float32) error
	Close() error
	EnableLowLatency() error
}

// NullBackend is a no-op audio backend for testing without hardware.
type NullBackend struct {
	sampleRate int
	channels   int
	bitDepth   int
}

func NewNullBackend(sampleRate, channels, bitDepth int) *NullBackend {
	return &NullBackend{
		sampleRate: sampleRate,
		channels:   channels,
		bitDepth:   bitDepth,
	}
}

func (n *NullBackend) SampleRate() int      { return n.sampleRate }
func (n *NullBackend) Channels() int        { return n.channels }
func (n *NullBackend) BitDepth() int        { return n.bitDepth }
func (n *NullBackend) AllMicrophones() []string    { return []string{"null-mic"} }
func (n *NullBackend) DefaultMicrophone() string   { return "null-mic" }
func (n *NullBackend) AllSpeakers() []string       { return []string{"null-speaker"} }
func (n *NullBackend) DefaultSpeaker() string      { return "null-speaker" }
func (n *NullBackend) Flush() error                { return nil }
func (n *NullBackend) ReleaseRecorder() error      { return nil }
func (n *NullBackend) ReleasePlayer() error        { return nil }

func (n *NullBackend) GetRecorder(samplesPerFrame int) (AudioRecorder, error) {
	return &NullRecorder{samplesPerFrame: samplesPerFrame}, nil
}

func (n *NullBackend) GetPlayer(samplesPerFrame int, lowLatency bool) (AudioPlayer, error) {
	return &NullPlayer{}, nil
}

type NullRecorder struct {
	samplesPerFrame int
}

func (n *NullRecorder) Record(numFrames int) ([][]float32, error) {
	// Return silence
	frame := make([][]float32, numFrames)
	for i := range frame {
		frame[i] = make([]float32, 2) // stereo silence
	}
	return frame, nil
}

func (n *NullRecorder) Close() error { return nil }

type NullPlayer struct{}

func (n *NullPlayer) Play(frame [][]float32) error { return nil }
func (n *NullPlayer) Close() error               { return nil }
func (n *NullPlayer) EnableLowLatency() error    { return nil }