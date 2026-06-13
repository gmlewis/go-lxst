// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/mixer"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// AudioPipeline manages the transmit and receive audio pipelines for a call.
type AudioPipeline struct {
	mu             sync.Mutex
	transmitCodec  codecs.Codec
	receiveCodec   codecs.Codec
	audioInput     *sources.LineSource
	audioOutput    *sinks.LineSink
	transmitMixer  *mixer.Mixer
	receiveMixer   *mixer.Mixer
	packetizer     *network.Packetizer
	linkSource     *network.LinkSource
	signallingRcvr *network.SignallingReceiver
	speakerDevice  string
	micDevice      string
	targetFrameMs  float64
	samplerate     int
	transmitGain   float64
	receiveGain    float64
}

// NewAudioPipeline creates a new AudioPipeline with the given configuration.
func NewAudioPipeline(
	transmitCodec, receiveCodec codecs.Codec,
	speakerDevice, micDevice string,
	targetFrameMs float64,
	samplerate int,
	transmitGain, receiveGain float64,
) *AudioPipeline {
	if targetFrameMs <= 0 {
		targetFrameMs = 60.0
	}
	if samplerate <= 0 {
		samplerate = 48000
	}

	return &AudioPipeline{
		transmitCodec: transmitCodec,
		receiveCodec:  receiveCodec,
		speakerDevice: speakerDevice,
		micDevice:     micDevice,
		targetFrameMs: targetFrameMs,
		samplerate:    samplerate,
		transmitGain:  transmitGain,
		receiveGain:   receiveGain,
	}
}

// SetupTransmit creates the transmit pipeline components.
func (ap *AudioPipeline) SetupTransmit(sendFunc func(data []byte) error, failureCallback func()) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.packetizer = network.NewPacketizer(sendFunc, failureCallback)
	ap.packetizer.SetCodec(ap.transmitCodec)

	ap.transmitMixer = mixer.NewMixer(ap.targetFrameMs, ap.samplerate, nil, nil, ap.transmitGain)
	if err := ap.transmitMixer.SetCodec(ap.transmitCodec); err != nil {
		return fmt.Errorf("setting transmit codec: %w", err)
	}

	ap.audioInput = sources.NewLineSource(
		ap.micDevice,
		ap.targetFrameMs,
		nil,
		ap.transmitMixer,
		nil,
		0.0,
		0.225,
		0.075,
	)

	return nil
}

// SetupReceive creates the receive pipeline components.
func (ap *AudioPipeline) SetupReceive(signallingRcvr *network.SignallingReceiver) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.signallingRcvr = signallingRcvr

	ap.audioOutput = sinks.NewLineSink(ap.speakerDevice, true, false)

	ap.receiveMixer = mixer.NewMixer(ap.targetFrameMs, ap.samplerate, nil, nil, ap.receiveGain)

	ap.linkSource = network.NewLinkSource(signallingRcvr, ap.receiveMixer)

	return nil
}

// Start begins the audio pipelines.
func (ap *AudioPipeline) Start() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.receiveMixer != nil {
		if err := ap.receiveMixer.Start(); err != nil {
			return fmt.Errorf("starting receive mixer: %w", err)
		}
	}
	if ap.transmitMixer != nil {
		if err := ap.transmitMixer.Start(); err != nil {
			return fmt.Errorf("starting transmit mixer: %w", err)
		}
	}
	if ap.audioInput != nil {
		if err := ap.audioInput.Start(); err != nil {
			return fmt.Errorf("starting audio input: %w", err)
		}
	}
	if ap.linkSource != nil {
		if err := ap.linkSource.Start(); err != nil {
			return fmt.Errorf("starting link source: %w", err)
		}
	}
	return nil
}

// Stop halts the audio pipelines.
func (ap *AudioPipeline) Stop() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.receiveMixer != nil {
		_ = ap.receiveMixer.Stop()
	}
	if ap.transmitMixer != nil {
		_ = ap.transmitMixer.Stop()
	}
	if ap.audioInput != nil {
		_ = ap.audioInput.Stop()
	}
	if ap.linkSource != nil {
		_ = ap.linkSource.Stop()
	}
}

// Packetizer returns the network packetizer for sending audio frames.
func (ap *AudioPipeline) Packetizer() *network.Packetizer {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.packetizer
}

// LinkSource returns the network link source for receiving audio frames.
func (ap *AudioPipeline) LinkSource() *network.LinkSource {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.linkSource
}

// ReceivePacket passes a received network packet to the link source.
func (ap *AudioPipeline) ReceivePacket(data []byte) {
	ap.mu.Lock()
	ls := ap.linkSource
	ap.mu.Unlock()

	if ls != nil {
		ls.ReceivePacket(data)
	}
}
