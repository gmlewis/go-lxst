// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package lxst

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
)

func TestPackageExports_NewOpus(t *testing.T) {
	t.Parallel()

	o, err := NewOpus(OpusProfileVoiceLow)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	if o == nil {
		t.Error("NewOpus should return non-nil")
	}
}

func TestPackageExports_NewRaw(t *testing.T) {
	t.Parallel()

	r, err := NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}
	if r == nil {
		t.Error("NewRaw should return non-nil")
	}
}

func TestPackageExports_NewCodec2(t *testing.T) {
	t.Parallel()

	c, err := NewCodec2(Codec2Mode700C)
	if err != nil {
		t.Fatalf("NewCodec2 failed: %v", err)
	}
	if c == nil {
		t.Error("NewCodec2 should return non-nil")
	}
}

func TestPackageExports_CodecHeaderRoundtrip(t *testing.T) {
	t.Parallel()

	r, _ := NewRaw(1, 16)
	b, err := CodecHeaderByte(r)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}

	recovered, err := CodecTypeFromHeader(b)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := recovered.(*raw.Raw); !ok {
		t.Error("Expected Raw codec from header byte")
	}
}

func TestPackageExports_Filters(t *testing.T) {
	t.Parallel()

	hp := NewHighPass(300)
	if hp == nil {
		t.Error("NewHighPass should return non-nil")
	}

	lp := NewLowPass(3000)
	if lp == nil {
		t.Error("NewLowPass should return non-nil")
	}

	bp := NewBandPass(300, 3000)
	if bp == nil {
		t.Error("NewBandPass should return non-nil")
	}

	agc := NewAGC(1.0, 30.0, 0.01, 0.1, 0.0)
	if agc == nil {
		t.Error("NewAGC should return non-nil")
	}
}

func TestPackageExports_Generators(t *testing.T) {
	t.Parallel()

	ts := NewToneSource(440.0, 0.1, true, 20.0, 80.0, nil, nil, 1)
	if ts == nil {
		t.Error("NewToneSource should return non-nil")
	}
}

func TestPackageExports_Profiles(t *testing.T) {
	t.Parallel()

	ft := GetFrameTime(ProfileQualityMedium)
	if ft != 60.0 {
		t.Errorf("GetFrameTime(QualityMedium) = %f, want 60.0", ft)
	}
}

func TestPackageExports_CodecInterface(t *testing.T) {
	t.Parallel()

	var _ Codec = codecs.NullCodec{}

	r, _ := NewRaw(1, 16)
	var _ Codec = r

	c, _ := NewCodec2(Codec2Mode700C)
	var _ Codec = c
}

func TestPackageExports_Codec2ModeConstants(t *testing.T) {
	t.Parallel()

	modes := []int{
		Codec2Mode700C,
		Codec2Mode700B,
		Codec2Mode1200,
		Codec2Mode1300,
		Codec2Mode1400,
		Codec2Mode1600,
		Codec2Mode2400,
		Codec2Mode3200,
	}

	for _, m := range modes {
		c, err := NewCodec2(m)
		if err != nil {
			t.Errorf("NewCodec2(%d) failed: %v", m, err)
		}
		if c == nil {
			t.Errorf("NewCodec2(%d) returned nil", m)
		}
	}
}

func TestPackageExports_SourceInterfaces(t *testing.T) {
	t.Parallel()

	ls := NewLineSource("", 20.0, codecs.NullCodec{}, nil, nil, 0.0, 0.0, 0.0)
	var _ Source = ls
}

func TestPackageExports_SinkInterfaces(t *testing.T) {
	t.Parallel()

	var s Sink = &LocalSink{}
	_ = s

	var s2 Sink = &RemoteSink{}
	_ = s2
}
