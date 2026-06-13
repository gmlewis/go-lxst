// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package mixer

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestMixer_FramePool_Basic(t *testing.T) {
	t.Parallel()

	pool := NewFramePool(1920, 2)

	frame := pool.Get()
	if frame == nil {
		t.Fatal("FramePool.Get() returned nil")
	}
	if len(frame) != 1920 {
		t.Errorf("Expected frame length 1920, got %d", len(frame))
	}
	if len(frame[0]) != 2 {
		t.Errorf("Expected channel count 2, got %d", len(frame[0]))
	}

	pool.Put(frame)
}

func TestMixer_FramePool_Reuse(t *testing.T) {
	t.Parallel()

	pool := NewFramePool(1920, 2)

	frame1 := pool.Get()
	frame1[0][0] = 1.0
	pool.Put(frame1)

	frame2 := pool.Get()
	if frame2[0][0] != 0.0 {
		t.Error("Frame should be zeroed after Put/Get cycle")
	}
	pool.Put(frame2)
}

func TestMixer_FramePool_Concurrent(t *testing.T) {
	t.Parallel()

	pool := NewFramePool(1920, 2)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				frame := pool.Get()
				frame[0][0] = float32(j)
				pool.Put(frame)
			}
		}()
	}

	wg.Wait()
}

func BenchmarkMixer_FramePool_GetPut(b *testing.B) {
	pool := NewFramePool(1920, 2)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		frame := pool.Get()
		pool.Put(frame)
	}
}

func BenchmarkMixer_MakeSlice(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		frame := make([][]float32, 1920)
		for j := range frame {
			frame[j] = make([]float32, 2)
		}
	}
}

func BenchmarkMixer_MixingWithPool(b *testing.B) {
	sink := &benchSink{}
	m := NewMixer(40.0, 48000, nil, sink, 0.0)
	sink.SetMixer(m)

	frame := make([][]float32, 1920)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.05)) * 0.5, float32(math.Cos(float64(i)*0.05)) * 0.3}
	}
	src1 := &benchSource{frame: frame}
	src2 := &benchSource{frame: frame}

	_ = m.HandleFrame(frame, src1)
	_ = m.HandleFrame(frame, src2)

	_ = m.Start()
	defer func() { _ = m.Stop() }()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.HandleFrame(frame, src1)
		_ = m.HandleFrame(frame, src2)
		time.Sleep(time.Microsecond)
	}
}
