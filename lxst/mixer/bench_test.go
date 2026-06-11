// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package mixer

import (
	"math"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/sources"
)

func BenchmarkMixer_SingleSource(b *testing.B) {
	m := NewMixer(40.0, 48000, nil, nil, 0.0)
	frame := make([][]float32, 1920)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.05)) * 0.5, float32(math.Cos(float64(i) * 0.05)) * 0.3}
	}
	src := &benchSource{frame: frame}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.HandleFrame(frame, src)
	}
}

func BenchmarkMixer_DualSource(b *testing.B) {
	m := NewMixer(40.0, 48000, nil, nil, 0.0)
	frame := make([][]float32, 1920)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.05)) * 0.5, float32(math.Cos(float64(i) * 0.05)) * 0.3}
	}
	src1 := &benchSource{frame: frame}
	src2 := &benchSource{frame: frame}

	m.HandleFrame(frame, src1)
	m.HandleFrame(frame, src2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.HandleFrame(frame, src1)
		m.HandleFrame(frame, src2)
	}
}

type benchSource struct {
	frame   [][]float32
	running bool
}

func (s *benchSource) Start() error                                  { return nil }
func (s *benchSource) Stop() error                                   { return nil }
func (s *benchSource) Running() bool                                  { return true }
func (s *benchSource) CanReceive(fromSource sources.Source) bool      { return true }
func (s *benchSource) HandleFrame(frame [][]float32, fromSource sources.Source) error { return nil }