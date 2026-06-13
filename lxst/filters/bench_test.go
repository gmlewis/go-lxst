// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package filters

import (
	"math"
	"testing"
)

func BenchmarkHighPass_HandleFrame(b *testing.B) {
	hp := NewHighPass(300)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.1)) * 0.5, float32(math.Cos(float64(i)*0.1)) * 0.3}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hp.HandleFrame(frame, 48000)
	}
}

func BenchmarkLowPass_HandleFrame(b *testing.B) {
	lp := NewLowPass(3000)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.1)) * 0.5, float32(math.Cos(float64(i)*0.1)) * 0.3}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lp.HandleFrame(frame, 48000)
	}
}

func BenchmarkBandPass_HandleFrame(b *testing.B) {
	bp := NewBandPass(300, 3000)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.1)) * 0.5, float32(math.Cos(float64(i)*0.1)) * 0.3}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bp.HandleFrame(frame, 48000)
	}
}

func BenchmarkAGC_HandleFrame(b *testing.B) {
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i)*0.1)) * 0.5, float32(math.Cos(float64(i)*0.1)) * 0.3}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agc.HandleFrame(frame, 48000)
	}
}
