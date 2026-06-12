// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package generators provides audio signal generators for the LXST library.
// It includes ToneSource for producing sine wave tones with configurable
// frequency, gain, and easing parameters, supporting both mono and stereo
// output with optional codec constraints for frame size and sample rate.
package generators

import (
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

const (
	ToneSourceDefaultFrameMs    = 80.0
	ToneSourceDefaultSampleRate = 48000
	ToneSourceDefaultFrequency  = 400.0
	ToneSourceEaseTimeMs        = 20.0
)

type ToneSource struct {
	mu              sync.Mutex
	targetFrameMs   float64
	samplerate      int
	channels        int
	bitdepth        int
	frequency       float64
	gain            float64
	ease            bool
	easeTimeMs      float64
	theta           float64
	easeGain        float64
	easeStep        float64
	gainStep        float64
	currentGain     float64
	easingOut       bool
	shouldRun       bool
	generateThread  *generateThreadInfo
	codec           codecs.Codec
	sink            sources.LocalSource
	samplesPerFrame int
	frameTime       float64
}

type generateThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

func NewToneSource(frequency, gain float64, ease bool, easeTimeMs, targetFrameMs float64, codec codecs.Codec, sink sources.LocalSource, channels int) *ToneSource {
	if targetFrameMs <= 0 {
		targetFrameMs = ToneSourceDefaultFrameMs
	}
	if frequency <= 0 {
		frequency = ToneSourceDefaultFrequency
	}
	if channels <= 0 {
		channels = 1
	}

	ts := &ToneSource{
		targetFrameMs: targetFrameMs,
		samplerate:    ToneSourceDefaultSampleRate,
		channels:      channels,
		bitdepth:      32,
		frequency:     frequency,
		gain:          gain,
		currentGain:   gain,
		ease:          ease,
		easeTimeMs:    easeTimeMs,
		codec:         codec,
		sink:          sink,
	}

	if codec != nil {
		ts.applyCodecConstraints()
	}

	ts.samplesPerFrame = int(math.Ceil((ts.targetFrameMs / 1000.0) * float64(ts.samplerate)))
	ts.frameTime = float64(ts.samplesPerFrame) / float64(ts.samplerate)
	ts.easeStep = 1.0 / (float64(ts.samplerate) * (ts.easeTimeMs / 1000.0))
	ts.gainStep = 0.02 / (float64(ts.samplerate) * (ts.easeTimeMs / 1000.0))

	return ts
}

func (ts *ToneSource) applyCodecConstraints() {
	if ts.codec == nil {
		return
	}

	if pref := ts.codec.PreferredSampleRate(); pref > 0 {
		ts.samplerate = pref
	}

	if quanta := ts.codec.FrameQuantumMs(); quanta > 0 {
		if math.Mod(ts.targetFrameMs, quanta) != 0 {
			ts.targetFrameMs = math.Ceil(ts.targetFrameMs/quanta) * quanta
		}
	}

	if maxMs := ts.codec.FrameMaxMs(); maxMs > 0 && ts.targetFrameMs > maxMs {
		ts.targetFrameMs = maxMs
	}

	if valid := ts.codec.ValidFrameMs(); len(valid) > 0 {
		closest := valid[0]
		minDiff := math.Abs(ts.targetFrameMs - valid[0])
		for _, v := range valid[1:] {
			diff := math.Abs(ts.targetFrameMs - v)
			if diff < minDiff {
				minDiff = diff
				closest = v
			}
		}
		ts.targetFrameMs = closest
	}
}

func (ts *ToneSource) Start() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.shouldRun {
		return sources.ErrSourceAlreadyRunning
	}

	if ts.ease {
		ts.easeGain = 0.0
	} else {
		ts.easeGain = 1.0
	}

	ts.shouldRun = true
	ts.easingOut = false

	ts.generateThread = &generateThreadInfo{
		done: make(chan struct{}),
	}
	ts.generateThread.wg.Add(1)
	go ts.generateJob()

	return nil
}

func (ts *ToneSource) Stop() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.ease {
		ts.shouldRun = false
	} else {
		ts.easingOut = true
	}

	return nil
}

func (ts *ToneSource) Running() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.shouldRun && !ts.easingOut
}

func (ts *ToneSource) generate() [][]float32 {
	frame := make([][]float32, ts.samplesPerFrame)
	step := (ts.frequency * 2.0 * math.Pi) / float64(ts.samplerate)

	for n := 0; n < ts.samplesPerFrame; n++ {
		ts.theta += step
		amplitude := float32(math.Sin(ts.theta) * ts.currentGain * ts.easeGain)

		frame[n] = make([]float32, ts.channels)
		for c := 0; c < ts.channels; c++ {
			frame[n][c] = amplitude
		}

		if ts.gain > ts.currentGain {
			ts.currentGain += ts.gainStep
			if ts.currentGain > ts.gain {
				ts.currentGain = ts.gain
			}
		} else if ts.gain < ts.currentGain {
			ts.currentGain -= ts.gainStep
			if ts.currentGain < ts.gain {
				ts.currentGain = ts.gain
			}
		}

		if ts.ease {
			if ts.easeGain < 1.0 && !ts.easingOut {
				ts.easeGain += ts.easeStep
				if ts.easeGain > 1.0 {
					ts.easeGain = 1.0
				}
			} else if ts.easingOut && ts.easeGain > 0.0 {
				ts.easeGain -= ts.easeStep
				if ts.easeGain <= 0.0 {
					ts.easeGain = 0.0
					ts.easingOut = false
					ts.shouldRun = false
				}
			}
		}
	}

	return frame
}

func (ts *ToneSource) generateJob() {
	defer ts.generateThread.wg.Done()

	for {
		select {
		case <-ts.generateThread.done:
			return
		default:
		}

		ts.mu.Lock()
		shouldRun := ts.shouldRun
		ts.mu.Unlock()

		if !shouldRun {
			return
		}

		ts.mu.Lock()
		codec := ts.codec
		sink := ts.sink
		ts.mu.Unlock()

		if codec != nil && sink != nil && sink.CanReceive(ts) {
			frame := ts.generate()
			encoded := codec.Encode(frame)
			if len(encoded) > 0 && sink.CanReceive(ts) {
				_ = sink.HandleFrame(frame, ts)
			}
		} else if sink != nil && sink.CanReceive(ts) {
			frame := ts.generate()
			_ = sink.HandleFrame(frame, ts)
		}

		time.Sleep(time.Duration(ts.frameTime * float64(time.Second) * 0.1))
	}
}

func (ts *ToneSource) SetCodec(codec codecs.Codec) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.codec = codec
	ts.applyCodecConstraints()
	ts.samplesPerFrame = int(math.Ceil((ts.targetFrameMs / 1000.0) * float64(ts.samplerate)))
	ts.frameTime = float64(ts.samplesPerFrame) / float64(ts.samplerate)
	ts.easeStep = 1.0 / (float64(ts.samplerate) * (ts.easeTimeMs / 1000.0))
	ts.gainStep = 0.02 / (float64(ts.samplerate) * (ts.easeTimeMs / 1000.0))
	return nil
}

func (ts *ToneSource) GetCodec() codecs.Codec {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.codec
}

func (ts *ToneSource) Frequency() float64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.frequency
}

func (ts *ToneSource) SetFrequency(freq float64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.frequency = freq
}

func (ts *ToneSource) Gain() float64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.gain
}

func (ts *ToneSource) SetGain(gain float64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.gain = gain
}

func (ts *ToneSource) SampleRate() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.samplerate
}

func (ts *ToneSource) Channels() int {
	return ts.channels
}

func (ts *ToneSource) SamplesPerFrame() int {
	return ts.samplesPerFrame
}

func (ts *ToneSource) TargetFrameMs() float64 {
	return ts.targetFrameMs
}

func (ts *ToneSource) CanReceive(fromSource sources.Source) bool {
	return true
}

func (ts *ToneSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

// Ensure ToneSource implements sources.LocalSource
var _ sources.LocalSource = (*ToneSource)(nil)
