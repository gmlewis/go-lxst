// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package network

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
)

func TestCodecHeaderByte_Raw(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeRaw {
		t.Errorf("Expected CodeRaw (0x%02x), got 0x%02x", CodeRaw, b)
	}
}

func TestCodecHeaderByte_Opus(t *testing.T) {
	t.Parallel()

	codec, err := opus.NewOpus(opus.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeOpus {
		t.Errorf("Expected CodeOpus (0x%02x), got 0x%02x", CodeOpus, b)
	}
}

func TestCodecHeaderByte_Codec2(t *testing.T) {
	t.Parallel()

	codec, err := codec2.NewCodec2(codec2.MODE_700B)
	if err != nil {
		t.Skipf("Codec2 not available: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeCodec2 {
		t.Errorf("Expected CodeCodec2 (0x%02x), got 0x%02x", CodeCodec2, b)
	}
}

func TestCodecHeaderByte_Unknown(t *testing.T) {
	t.Parallel()

	_, err := CodecHeaderByte(nil)
	if err == nil {
		t.Error("Expected error for nil codec")
	}
}

func TestCodecTypeFromHeader_Raw(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeRaw)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*raw.Raw); !ok {
		t.Error("Expected Raw codec")
	}
}

func TestCodecTypeFromHeader_Opus(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeOpus)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*opus.Opus); !ok {
		t.Error("Expected Opus codec")
	}
}

func TestCodecTypeFromHeader_Codec2(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeCodec2)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*codec2.Codec2); !ok {
		t.Error("Expected Codec2 codec")
	}
}

func TestCodecTypeFromHeader_Unknown(t *testing.T) {
	t.Parallel()

	_, err := CodecTypeFromHeader(0xFE)
	if err == nil {
		t.Error("Expected error for unknown header byte")
	}
}

func TestCodecHeaderByte_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		codec func() (codecs.Codec, error)
		code  byte
	}{
		{"Raw", func() (codecs.Codec, error) { return raw.NewRaw(1, 16) }, CodeRaw},
		{"Opus", func() (codecs.Codec, error) { return opus.NewOpus(opus.PROFILE_VOICE_LOW) }, CodeOpus},
		{"Codec2", func() (codecs.Codec, error) { return codec2.NewCodec2(codec2.MODE_700B) }, CodeCodec2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codec, err := tc.codec()
			if err != nil {
				t.Skipf("Codec not available: %v", err)
			}

			b, err := CodecHeaderByte(codec)
			if err != nil {
				t.Fatalf("CodecHeaderByte failed: %v", err)
			}
			if b != tc.code {
				t.Errorf("Expected 0x%02x, got 0x%02x", tc.code, b)
			}

			recovered, err := CodecTypeFromHeader(b)
			if err != nil {
				t.Fatalf("CodecTypeFromHeader failed: %v", err)
			}

			b2, err := CodecHeaderByte(recovered)
			if err != nil {
				t.Fatalf("CodecHeaderByte on recovered codec failed: %v", err)
			}
			if b2 != b {
				t.Errorf("Roundtrip failed: expected 0x%02x, got 0x%02x", b, b2)
			}
		})
	}
}