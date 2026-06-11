// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package codecs

import (
	"math"
	"testing"

	opuspkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
)

func BenchmarkNullCodec_Encode(b *testing.B) {
	codec := NullCodec{}
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.1)) * 0.5, float32(math.Cos(float64(i) * 0.1)) * 0.3}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(frame)
	}
}

func BenchmarkNullCodec_Decode(b *testing.B) {
	codec := NullCodec{}
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.1)) * 0.5}
	}
	data := codec.Encode(frame)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Decode(data, 1)
	}
}

func BenchmarkOpusVoiceLow_Encode(b *testing.B) {
	codec, err := opuspkg.NewOpus(opuspkg.PROFILE_VOICE_LOW)
	if err != nil {
		b.Skipf("Opus not available: %v", err)
	}
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.1)) * 0.5}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(frame)
	}
}

func BenchmarkRawCodec_Encode(b *testing.B) {
	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		b.Fatal(err)
	}
	frame := make([][]float32, 960)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.05)) * 0.8}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(frame)
	}
}

func BenchmarkRawCodec_Decode(b *testing.B) {
	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		b.Fatal(err)
	}
	frame := make([][]float32, 960)
	for i := range frame {
		frame[i] = []float32{float32(math.Sin(float64(i) * 0.05)) * 0.8}
	}
	data := codec.Encode(frame)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Decode(data, 1)
	}
}