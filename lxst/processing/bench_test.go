// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package processing

import (
	"math"
	"testing"
)

func makeBenchFrame(n, ch int) [][]float32 {
	frame := make([][]float32, n)
	for i := range frame {
		frame[i] = make([]float32, ch)
		for c := 0; c < ch; c++ {
			frame[i][c] = float32(math.Sin(float64(i)*0.05+float64(c))) * 0.5
		}
	}
	return frame
}

func BenchmarkRMS(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RMS(frame)
	}
}

func BenchmarkPeak(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Peak(frame)
	}
}

func BenchmarkIsSilence(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsSilence(frame, 0.01)
	}
}

func BenchmarkVAD(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VAD(frame, 0.01)
	}
}

func BenchmarkConvertChannels(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertChannels(frame, 1)
	}
}

func BenchmarkResample(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resample(frame, 48000, 16000)
	}
}

func BenchmarkNormalize(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Normalize(frame)
	}
}

func BenchmarkClipCount(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClipCount(frame)
	}
}

func BenchmarkEnergy(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Energy(frame)
	}
}

func BenchmarkZeroCrossingRate(b *testing.B) {
	frame := makeBenchFrame(960, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ZeroCrossingRate(frame)
	}
}
