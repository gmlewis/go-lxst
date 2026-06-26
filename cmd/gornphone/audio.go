// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
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
	started        bool
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

	if ap.started {
		return nil
	}

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
	ap.started = true
	return nil
}

// Stop halts the audio pipelines.
func (ap *AudioPipeline) Stop() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if !ap.started {
		return
	}

	if ap.receiveMixer != nil {
		if err := ap.receiveMixer.Stop(); err != nil {
			log.Printf("AudioPipeline.Stop: receiveMixer.Stop failed: %v", err)
		}
	}
	if ap.transmitMixer != nil {
		if err := ap.transmitMixer.Stop(); err != nil {
			log.Printf("AudioPipeline.Stop: transmitMixer.Stop failed: %v", err)
		}
	}
	if ap.audioInput != nil {
		if err := ap.audioInput.Stop(); err != nil {
			log.Printf("AudioPipeline.Stop: audioInput.Stop failed: %v", err)
		}
	}
	if ap.linkSource != nil {
		if err := ap.linkSource.Stop(); err != nil {
			log.Printf("AudioPipeline.Stop: linkSource.Stop failed: %v", err)
		}
	}
	ap.started = false
}

// Started reports whether the pipeline has been started.
func (ap *AudioPipeline) Started() bool {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.started
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

// TransmitCodec returns the codec used for transmitting audio.
func (ap *AudioPipeline) TransmitCodec() codecs.Codec {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.transmitCodec
}

// ReceiveCodec returns the codec used for receiving audio.
func (ap *AudioPipeline) ReceiveCodec() codecs.Codec {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.receiveCodec
}

// TargetFrameMs returns the target frame time in milliseconds.
func (ap *AudioPipeline) TargetFrameMs() float64 {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.targetFrameMs
}

// Samplerate returns the audio samplerate.
func (ap *AudioPipeline) Samplerate() int {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.samplerate
}

// TransmitGain returns the transmit gain in dB.
func (ap *AudioPipeline) TransmitGain() float64 {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.transmitGain
}

// ReceiveGain returns the receive gain in dB.
func (ap *AudioPipeline) ReceiveGain() float64 {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.receiveGain
}
