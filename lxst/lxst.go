// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package lxst provides the LXST audio processing library for Reticulum.
//
// LXST provides audio codecs, sources, sinks, filters, generators, mixers,
// and pipelines for real-time audio processing and telephony over
// delay-tolerant networks.
package lxst

import (
	"github.com/gmlewis/go-lxst/lxst/call"
	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/generators"
	"github.com/gmlewis/go-lxst/lxst/mixer"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/pipeline"
	"github.com/gmlewis/go-lxst/lxst/platforms"
	"github.com/gmlewis/go-lxst/lxst/primitives/players"
	"github.com/gmlewis/go-lxst/lxst/primitives/recorders"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// Codecs
type Codec = codecs.Codec

// Opus codec
var (
	OpusProfileVoiceLow    = opus.PROFILE_VOICE_LOW
	OpusProfileVoiceMedium = opus.PROFILE_VOICE_MEDIUM
	OpusProfileVoiceHigh   = opus.PROFILE_VOICE_HIGH
	OpusProfileVoiceMax    = opus.PROFILE_VOICE_MAX
	OpusProfileAudioMin    = opus.PROFILE_AUDIO_MIN
	OpusProfileAudioLow    = opus.PROFILE_AUDIO_LOW
	OpusProfileAudioMedium = opus.PROFILE_AUDIO_MEDIUM
	OpusProfileAudioHigh   = opus.PROFILE_AUDIO_HIGH
	OpusProfileAudioMax    = opus.PROFILE_AUDIO_MAX
	NewOpus                = opus.NewOpus
)

// Raw codec
var NewRaw = raw.NewRaw

// Codec2
var (
	Codec2Mode700C = codec2.MODE_700C
	Codec2Mode700B = codec2.MODE_700B
	Codec2Mode1200 = codec2.MODE_1200
	Codec2Mode1300 = codec2.MODE_1300
	Codec2Mode1400 = codec2.MODE_1400
	Codec2Mode1600 = codec2.MODE_1600
	Codec2Mode2400 = codec2.MODE_2400
	Codec2Mode3200 = codec2.MODE_3200
	NewCodec2      = codec2.NewCodec2
)

// Filters
var (
	NewHighPass = filters.NewHighPass
	NewLowPass  = filters.NewLowPass
	NewBandPass = filters.NewBandPass
	NewAGC      = filters.NewAGC
)

// Sources
var (
	NewLineSource     = sources.NewLineSource
	NewLoopback       = sources.NewLoopback
	NewOpusFileSource = sources.NewOpusFileSource
)

// Sinks
var (
	NewLineSink     = sinks.NewLineSink
	NewOpusFileSink = sinks.NewOpusFileSink
)

// Pipeline
var NewPipeline = pipeline.NewPipeline

// Mixer
var NewMixer = mixer.NewMixer

// Generators
var NewToneSource = generators.NewToneSource

// Network codec headers
var (
	CodecHeaderByte       = network.CodecHeaderByte
	CodecTypeFromHeader   = network.CodecTypeFromHeader
	NewSignallingReceiver = network.NewSignallingReceiver
	NewPacketizer         = network.NewPacketizer
	NewLinkSource         = network.NewLinkSource
)

// Platform backends
var NewBackend = platforms.NewBackend

// Primitives
var (
	NewFilePlayer   = players.NewFilePlayer
	NewFileRecorder = recorders.NewFileRecorder
	NewCallEndpoint = call.NewCallEndpoint
	NewTelephone    = telephony.NewTelephone
)

// Telephony profiles
var (
	ProfileBandwidthUltraLow = telephony.ProfileBandwidthUltraLow
	ProfileBandwidthVeryLow  = telephony.ProfileBandwidthVeryLow
	ProfileBandwidthLow      = telephony.ProfileBandwidthLow
	ProfileQualityMedium     = telephony.ProfileQualityMedium
	ProfileQualityHigh       = telephony.ProfileQualityHigh
	ProfileQualityMax        = telephony.ProfileQualityMax
	ProfileLatencyLow        = telephony.ProfileLatencyLow
	ProfileLatencyUltraLow   = telephony.ProfileLatencyUltraLow
	GetCodec                 = telephony.GetCodec
	GetFrameTime             = telephony.GetFrameTime
	NextProfile              = telephony.NextProfile
)

// Source interfaces
type (
	Source       = sources.Source
	LocalSource  = sources.LocalSource
	RemoteSource = sources.RemoteSource
)

// Sink interfaces
type (
	Sink       = sinks.Sink
	LocalSink  = sinks.LocalSink
	RemoteSink = sinks.RemoteSink
)

// Pipeline types
type Pipeline = pipeline.Pipeline
