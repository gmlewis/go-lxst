// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package recorders provides audio file recording primitives.
package recorders

import (
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

type FileRecorder struct {
	mu           sync.Mutex
	filePath     string
	recordDevice string
	profile      int
	source       *sources.LineSource
	sink         *sinks.OpusFileSink
	nullCodec    codecs.NullCodec
	filterChain  []filters.Filter
	easeIn       float64
	skip         float64
	gain         float64
}

func NewFileRecorder(path string, device string, profile int, gain, easeIn, skip float64) *FileRecorder {
	fr := &FileRecorder{
		filePath:     path,
		recordDevice: device,
		profile:      profile,
		nullCodec:    codecs.NullCodec{},
		filterChain:  []filters.Filter{filters.NewBandPass(25, 24000)},
		easeIn:       easeIn,
		skip:         skip,
		gain:         gain,
	}

	sink, err := sinks.NewOpusFileSink(path, true, profile)
	if err == nil {
		fr.sink = sink
	}

	fr.setSource(device)

	return fr
}

func (fr *FileRecorder) setSource(device string) {
	fr.recordDevice = device
	if fr.sink == nil {
		fr.source = sources.NewLineSource(device, 20.0, fr.nullCodec, nil, fr.filterChain, fr.gain, fr.easeIn, fr.skip)
		return
	}
	fr.source = sources.NewLineSource(device, 20.0, fr.nullCodec, nil, fr.filterChain, fr.gain, fr.easeIn, fr.skip)
}

func (fr *FileRecorder) SetSource(device string) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.setSource(device)
}

func (fr *FileRecorder) Running() bool {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if fr.source == nil {
		return false
	}
	return fr.source.Running()
}

func (fr *FileRecorder) Recording() bool {
	return fr.Running()
}

func (fr *FileRecorder) Start() error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if fr.source != nil {
		return fr.source.Start()
	}
	return nil
}

func (fr *FileRecorder) Stop() error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if fr.source == nil {
		return nil
	}

	err := fr.source.Stop()
	if err != nil {
		return err
	}

	if fr.sink != nil {
		for fr.sink.FramesWaiting() > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		fr.sink.Stop()
	}

	return nil
}

func (fr *FileRecorder) Record() error {
	return fr.Start()
}

func (fr *FileRecorder) FilePath() string {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	return fr.filePath
}

func (fr *FileRecorder) FramesWaiting() int {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if fr.sink == nil {
		return 0
	}
	return fr.sink.FramesWaiting()
}
