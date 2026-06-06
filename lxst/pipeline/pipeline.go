// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package pipeline provides audio pipeline management.
package pipeline

import (
	"errors"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

var (
	ErrInvalidSource = errors.New("audio pipeline initialized with invalid source")
	ErrInvalidSink   = errors.New("audio pipeline initialized with invalid sink")
	ErrInvalidCodec  = errors.New("audio pipeline initialized with invalid codec")
)

type PipelineError string

func (e PipelineError) Error() string { return string(e) }

// Pipeline connects a source, codec, and sink into an audio processing chain.
type Pipeline struct {
	codecImpl codecs.Codec
	source    sources.LocalSource
	sink      sinks.Sink
}

func NewPipeline(source sources.LocalSource, codec codecs.Codec, sink sinks.Sink) (*Pipeline, error) {
	if source == nil {
		return nil, ErrInvalidSource
	}
	if sink == nil {
		return nil, ErrInvalidSink
	}
	if codec == nil {
		return nil, ErrInvalidCodec
	}

	p := &Pipeline{
		source:    source,
		sink:      sink,
		codecImpl: codec,
	}

	p.setCodec(codec)

	if ls, ok := sink.(*sinks.LineSink); ok {
		_ = ls
	}

	if loopback, ok := sink.(*sources.Loopback); ok {
		loopback.SetSource(source)
	}

	return p, nil
}

func (p *Pipeline) setCodec(codec codecs.Codec) {
	p.codecImpl = codec
	if src, ok := p.source.(interface{ SetCodec(codecs.Codec) error }); ok {
		_ = src.SetCodec(codec)
	}
}

func (p *Pipeline) Codec() codecs.Codec {
	return p.codecImpl
}

func (p *Pipeline) Source() sources.LocalSource {
	return p.source
}

func (p *Pipeline) Sink() sinks.Sink {
	return p.sink
}

func (p *Pipeline) Running() bool {
	if src, ok := p.source.(interface{ Running() bool }); ok {
		return ok && src.Running()
	}
	return false
}

func (p *Pipeline) Start() error {
	if src, ok := p.source.(interface{ Start() error }); ok {
		return src.Start()
	}
	return nil
}

func (p *Pipeline) Stop() error {
	if src, ok := p.source.(interface{ Stop() error }); ok {
		return src.Stop()
	}
	return nil
}