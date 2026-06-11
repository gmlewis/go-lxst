// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package pipeline

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func TestPipeline_Constructor(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	src := sources.NewLoopback(codec, nil)
	sink := &sinks.LocalSink{}

	p, err := NewPipeline(src, codec, sink)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	if p.Codec() != codec {
		t.Error("Pipeline codec mismatch")
	}
}

func TestPipeline_Constructor_NilSource(t *testing.T) {
	t.Parallel()

	_, err := NewPipeline(nil, codecs.NullCodec{}, &sinks.LocalSink{})
	if err == nil {
		t.Error("Expected error for nil source")
	}
}

func TestPipeline_Constructor_NilSink(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	src := sources.NewLoopback(codec, nil)

	_, err := NewPipeline(src, codec, nil)
	if err == nil {
		t.Error("Expected error for nil sink")
	}
}

func TestPipeline_Constructor_NilCodec(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	src := sources.NewLoopback(codec, nil)
	sink := &sinks.LocalSink{}

	_, err := NewPipeline(src, nil, sink)
	if err == nil {
		t.Error("Expected error for nil codec")
	}
}

func TestPipeline_StartStop(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	src := sources.NewLoopback(codec, nil)
	sink := &sinks.LocalSink{}

	p, err := NewPipeline(src, codec, sink)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	if p.Running() {
		t.Error("Pipeline should not be running initially")
	}

	err = p.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !p.Running() {
		t.Error("Pipeline should be running after Start()")
	}

	err = p.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if p.Running() {
		t.Error("Pipeline should not be running after Stop()")
	}
}

func TestPipeline_Loopback(t *testing.T) {
	t.Parallel()

	codec := codecs.NullCodec{}
	mockSink := &mockPipelineSink{canReceive: true}
	src := sources.NewLoopback(codec, nil)

	p, err := NewPipeline(src, codec, mockSink)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = p.Start()

	frame := [][]float32{{0.5, -0.3}, {-0.7, 0.9}}
	err = src.HandleFrame(frame, src)
	if err != nil {
		t.Errorf("HandleFrame failed: %v", err)
	}

	_ = p.Stop()
}

func TestPipeline_WithRawCodec(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}
	src := sources.NewLoopback(codec, nil)
	sink := &sinks.LocalSink{}

	p, err := NewPipeline(src, codec, sink)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	if p.Codec() != codec {
		t.Error("Pipeline codec should match")
	}
}

type mockPipelineSink struct {
	canReceive bool
	running    bool
}

func (m *mockPipelineSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (m *mockPipelineSink) CanReceive(fromSource sources.Source) bool {
	return m.canReceive
}

func (m *mockPipelineSink) Start() error {
	m.running = true
	return nil
}

func (m *mockPipelineSink) Stop() error {
	m.running = false
	return nil
}

func (m *mockPipelineSink) Running() bool {
	return m.running
}
