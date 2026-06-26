// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// EchoSource is a LocalSource that serves two purposes:
//
//  1. It generates a continuous sine-wave test tone at the configured
//     frequency and gain, feeding it to the transmit mixer.
//  2. It receives decoded audio frames from the LinkSource (the remote
//     caller's audio), buffers them for the configured delay, then
//     feeds them to the transmit mixer as an echo.
//
// The EchoSource implements sources.LocalSource so the LinkSource can
// send it decoded frames via HandleFrame. It also runs a background
// goroutine that generates tone frames and emits delayed echo frames
// at the target frame rate.
type EchoSource struct {
	mu         sync.Mutex
	shouldRun  bool
	frequency  float64
	gain       float64
	delay      time.Duration
	frameMs    float64
	sink       sources.LocalSource
	linkSource *network.LinkSource

	// Tone generation state
	phase      float64
	sampleRate float64

	// Echo buffer: stores frames with timestamps for delayed playback
	echoBuffer   []timedFrame
	echoBufferMu sync.Mutex

	// Channels for the generated/echoed audio
	channels int
}

type timedFrame struct {
	frame    [][]float32
	emitTime time.Time
}

// NewEchoSource creates a new EchoSource that generates a tone at the
// given frequency and gain, and echoes received audio after the given
// delay. The frameMs parameter sets the audio frame duration. The
// sink is the transmit mixer that receives the mixed tone+echo audio.
func NewEchoSource(frequency, gain float64, delay time.Duration, frameMs float64, sink sources.LocalSource) *EchoSource {
	return &EchoSource{
		frequency:  frequency,
		gain:       gain,
		delay:      delay,
		frameMs:    frameMs,
		sink:       sink,
		channels:   1,
		sampleRate: 48000,
	}
}

// SetLinkSource connects the LinkSource so the EchoSource can access
// received audio codec info (channels, sample rate).
func (es *EchoSource) SetLinkSource(ls *network.LinkSource) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.linkSource = ls
	if ls != nil {
		if ls.Channels() > 0 {
			es.channels = ls.Channels()
		}
	}
}

// SampleRate returns the audio sample rate, needed by the Mixer to
// calculate samples per frame.
func (es *EchoSource) SampleRate() int {
	es.mu.Lock()
	defer es.mu.Unlock()
	return int(es.sampleRate)
}

// Channels returns the number of audio channels, needed by the Mixer
// to configure its output format.
func (es *EchoSource) Channels() int {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.channels
}

// Start begins the tone generation and echo playback goroutine.
func (es *EchoSource) Start() error {
	es.mu.Lock()
	if es.shouldRun {
		es.mu.Unlock()
		return nil
	}
	es.shouldRun = true
	es.mu.Unlock()

	go es.generateLoop()
	return nil
}

// Stop halts tone generation and echo playback.
func (es *EchoSource) Stop() error {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.shouldRun = false
	return nil
}

// Running reports whether the EchoSource is active.
func (es *EchoSource) Running() bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.shouldRun
}

// CanReceive always returns true — the EchoSource accepts all incoming
// frames from the LinkSource.
func (es *EchoSource) CanReceive(fromSource sources.Source) bool {
	return true
}

// HandleFrame receives decoded audio frames from the LinkSource and
// stores them in the echo buffer with a timestamp for delayed playback.
func (es *EchoSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	if len(frame) == 0 {
		return nil
	}

	es.echoBufferMu.Lock()
	// Set channels from received frame if we haven't set them yet.
	if es.channels == 0 && len(frame) > 0 {
		es.channels = len(frame)
	}
	es.echoBufferMu.Unlock()

	es.mu.Lock()
	running := es.shouldRun
	es.mu.Unlock()

	if !running {
		return nil
	}

	// Copy the frame and schedule it for delayed playback.
	now := time.Now()
	copied := make([][]float32, len(frame))
	for i, ch := range frame {
		copied[i] = make([]float32, len(ch))
		copy(copied[i], ch)
	}

	es.echoBufferMu.Lock()
	es.echoBuffer = append(es.echoBuffer, timedFrame{
		frame:    copied,
		emitTime: now.Add(es.delay),
	})
	es.echoBufferMu.Unlock()

	return nil
}

// HandleEncodedFrame is not used — the LinkSource decodes before
// calling HandleFrame.
func (es *EchoSource) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

// generateLoop runs in a background goroutine, generating tone frames
// and emitting delayed echo frames at the target frame rate.
func (es *EchoSource) generateLoop() {
	log.Printf("EchoSource.generateLoop: starting (freq=%.1f, gain=%.2f, frameMs=%.1f, sampleRate=%.0f, channels=%d, sink=%v)",
		es.frequency, es.gain, es.frameMs, es.sampleRate, es.channels, es.sink != nil)

	frameDuration := time.Duration(es.frameMs * float64(time.Millisecond))
	if frameDuration <= 0 {
		frameDuration = 60 * time.Millisecond
	}

	samplesPerFrame := int(es.sampleRate * es.frameMs / 1000.0)
	if samplesPerFrame <= 0 {
		samplesPerFrame = int(es.sampleRate * 0.06) // 60ms default
	}

	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		es.mu.Lock()
		running := es.shouldRun
		es.mu.Unlock()
		if !running {
			return
		}

		<-ticker.C

		// Generate tone frame.
		toneFrame := es.generateToneFrame(samplesPerFrame)

		// Get any echo frames that are ready to emit.
		echoFrames := es.getReadyEchoFrames(samplesPerFrame)

		// Mix tone and echo together, then send to the transmit mixer.
		var mixed [][]float32
		if len(echoFrames) > 0 {
			mixed = mixFrames(toneFrame, echoFrames)
		} else {
			mixed = toneFrame
		}

		// Send to the sink (transmit mixer).
		if es.sink != nil {
			if err := es.sink.HandleFrame(mixed, es); err != nil {
				log.Printf("EchoSource.generateLoop: sink.HandleFrame failed: %v", err)
			}
		}
	}
}

// generateToneFrame produces a single frame of sine-wave audio at the
// configured frequency and gain. The frame is in [samples][channels]
// format to match the Opus codec's expected input layout.
func (es *EchoSource) generateToneFrame(samplesPerFrame int) [][]float32 {
	es.mu.Lock()
	freq := es.frequency
	gain := es.gain
	phase := es.phase
	sr := es.sampleRate
	channels := es.channels
	es.mu.Unlock()

	// frame is [samples][channels] — outer dimension is samples,
	// inner dimension is channels.
	frame := make([][]float32, samplesPerFrame)
	for i := 0; i < samplesPerFrame; i++ {
		frame[i] = make([]float32, channels)
		sample := gain * math.Sin(2.0*math.Pi*freq*phase/sr)
		for c := 0; c < channels; c++ {
			frame[i][c] = float32(sample)
		}
		phase++
	}

	// Wrap phase to prevent overflow.
	phase = math.Mod(phase, sr)

	es.mu.Lock()
	es.phase = phase
	es.mu.Unlock()

	return frame
}

// getReadyEchoFrames returns echo frames whose emit time has passed.
// It flattens them into a single frame capped at maxSamples to match
// the tone frame size, preventing oversized frames from reaching the
// Opus encoder.
func (es *EchoSource) getReadyEchoFrames(maxSamples int) [][]float32 {
	now := time.Now()
	var result [][]float32

	es.echoBufferMu.Lock()
	remaining := es.echoBuffer[:0]
	for _, tf := range es.echoBuffer {
		if !tf.emitTime.After(now) {
			result = append(result, tf.frame...)
		} else {
			remaining = append(remaining, tf)
		}
	}
	es.echoBuffer = remaining
	es.echoBufferMu.Unlock()

	// Cap to maxSamples to prevent oversized frames from exceeding
	// the Opus encoder's expected frame duration.
	if len(result) > maxSamples {
		result = result[:maxSamples]
	}

	return result
}

// mixFrames mixes a tone frame with echo frames by summing samples.
// Both inputs are in [samples][channels] format.
func mixFrames(tone [][]float32, echo [][]float32) [][]float32 {
	samples := len(tone)
	if samples < len(echo) {
		samples = len(echo)
	}

	channels := 0
	if len(tone) > 0 && len(tone[0]) > channels {
		channels = len(tone[0])
	}
	if len(echo) > 0 && len(echo[0]) > channels {
		channels = len(echo[0])
	}

	result := make([][]float32, samples)
	for i := 0; i < samples; i++ {
		result[i] = make([]float32, channels)
		for c := 0; c < channels; c++ {
			var sum float32
			if i < len(tone) && c < len(tone[i]) {
				sum += tone[i][c]
			}
			if i < len(echo) && c < len(echo[i]) {
				sum += echo[i][c]
			}
			// Clamp to [-1, 1] to prevent clipping.
			if sum > 1.0 {
				sum = 1.0
			} else if sum < -1.0 {
				sum = -1.0
			}
			result[i][c] = sum
		}
	}
	return result
}
