// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package players provides audio file playback primitives for the LXST library.
// It includes the FilePlayer type which reads audio from file sources
// (Opus, FLAC, MP3, Vorbis) and streams decoded frames to a connected
// sink with configurable repeat and timing modes.
package players

import (
	"errors"
	"os"
	"sync"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/pipeline"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrNoSource        = errors.New("no source configured")
	ErrCallbackNotFunc = errors.New("provided callback is not callable")
)

type FinishedCallback func(player *FilePlayer)

type FilePlayer struct {
	mu               sync.Mutex
	filePath         string
	playbackDevice   string
	finishedCallback FinishedCallback
	loop             bool
	source           *sources.OpusFileSource
	outputPipeline   *pipeline.Pipeline
	inputPipeline    *pipeline.Pipeline
	sink             *sinks.LineSink
	loopback         *sources.Loopback
	raw              codecs.NullCodec
	running          bool
}

func NewFilePlayer(path string, device string, loop bool) (*FilePlayer, error) {
	fp := &FilePlayer{
		filePath:       path,
		playbackDevice: device,
		loop:           loop,
		sink:           sinks.NewLineSink(device, true, false, 0),
		loopback:       sources.NewLoopback(codecs.NullCodec{}, nil),
		raw:            codecs.NullCodec{},
	}

	if path != "" {
		if err := fp.setSource(path); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

func (fp *FilePlayer) setSource(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrFileNotFound
	}

	src, err := sources.NewOpusFileSource(path, 80.0, fp.loop, nil, nil, false)
	if err != nil {
		return err
	}

	fp.source = src

	return nil
}

func (fp *FilePlayer) SetSource(path string) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return fp.setSource(path)
}

func (fp *FilePlayer) Running() bool {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if fp.source == nil {
		return false
	}
	return fp.source.Running()
}

func (fp *FilePlayer) Playing() bool {
	return fp.Running()
}

func (fp *FilePlayer) SetFinishedCallback(callback FinishedCallback) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.finishedCallback = callback
	return nil
}

func (fp *FilePlayer) SetLoop(loop bool) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.loop = loop
}

func (fp *FilePlayer) Start() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.source == nil {
		return ErrNoSource
	}

	if fp.source.Running() {
		return nil
	}

	if fp.inputPipeline == nil {
		fp.loopback = sources.NewLoopback(codecs.NullCodec{}, nil)

		var err error
		fp.inputPipeline, err = pipeline.NewPipeline(fp.source, codecs.NullCodec{}, fp.loopback)
		if err != nil {
			return err
		}
	}

	if fp.outputPipeline == nil {
		fp.sink = sinks.NewLineSink(fp.playbackDevice, true, false, 0)
		var err error
		fp.outputPipeline, err = pipeline.NewPipeline(fp.loopback, codecs.NullCodec{}, fp.sink)
		if err != nil {
			return err
		}
	}

	if err := fp.inputPipeline.Start(); err != nil {
		return err
	}

	if err := fp.outputPipeline.Start(); err != nil {
		_ = fp.inputPipeline.Stop()
		return err
	}

	fp.running = true

	if fp.finishedCallback != nil {
		go fp.callbackJob()
	}

	return nil
}

func (fp *FilePlayer) Stop() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.source == nil {
		return nil
	}

	var inputErr, outputErr error
	if fp.inputPipeline != nil {
		inputErr = fp.inputPipeline.Stop()
	}
	if fp.outputPipeline != nil {
		outputErr = fp.outputPipeline.Stop()
	}

	fp.running = false

	if inputErr != nil {
		return inputErr
	}
	return outputErr
}

func (fp *FilePlayer) Play() error {
	return fp.Start()
}

func (fp *FilePlayer) callbackJob() {
	if fp.finishedCallback != nil {
		for fp.Running() {
			// Wait for playback to finish
		}
		fp.finishedCallback(fp)
	}
}

func (fp *FilePlayer) FilePath() string {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return fp.filePath
}
