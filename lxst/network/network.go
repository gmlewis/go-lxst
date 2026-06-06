// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package network provides audio networking over Reticulum.
package network

import (
	"errors"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
)

const (
	FieldSignalling byte = 0x00
	FieldFrames     byte = 0x01

	CodeNull   byte = 0xFF
	CodeRaw    byte = 0x00
	CodeOpus   byte = 0x01
	CodeCodec2 byte = 0x02
)

var (
	ErrUnknownCodecType = errors.New("unknown codec type")
)

// CodecHeaderByte returns the single-byte codec identifier for a given codec.
func CodecHeaderByte(codec codecs.Codec) (byte, error) {
	switch codec.(type) {
	case *raw.Raw:
		return CodeRaw, nil
	case *opus.Opus:
		return CodeOpus, nil
	case *codec2.Codec2:
		return CodeCodec2, nil
	default:
		return 0, ErrUnknownCodecType
	}
}

// CodecTypeFromHeader returns a new codec instance based on the header byte.
func CodecTypeFromHeader(headerByte byte) (codecs.Codec, error) {
	switch headerByte {
	case CodeRaw:
		return raw.NewRaw(1, 16)
	case CodeOpus:
		return opus.NewOpus(opus.PROFILE_VOICE_LOW)
	case CodeCodec2:
		return codec2.NewCodec2(codec2.MODE_700B)
	default:
		return nil, ErrUnknownCodecType
	}
}