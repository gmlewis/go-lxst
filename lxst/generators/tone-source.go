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

	thread := &generateThreadInfo{
		done: make(chan struct{}),
	}
	ts.generateThread = thread
	thread.wg.Add(1)
	go ts.generateJobWithThread(thread)

	return nil
}

func (ts *ToneSource) Stop() error {
	ts.mu.Lock()
	if !ts.shouldRun && !ts.easingOut {
		ts.mu.Unlock()
		return nil
	}

	if !ts.ease {
		ts.shouldRun = false
	} else {
		ts.easingOut = true
	}

	var thread *generateThreadInfo
	if ts.generateThread != nil && !ts.ease {
		thread = ts.generateThread
		ts.generateThread = nil
		close(thread.done)
	}
	ts.mu.Unlock()

	if thread != nil {
		thread.wg.Wait()
	}

	return nil
}

func (ts *ToneSource) Running() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.shouldRun && !ts.easingOut
}

func (ts *ToneSource) generate() [][]float32 {
	ts.mu.Lock()
	samplesPerFrame := ts.samplesPerFrame
	channels := ts.channels
	samplerate := ts.samplerate
	frequency := ts.frequency
	currentGain := ts.currentGain
	gain := ts.gain
	gainStep := ts.gainStep
	ease := ts.ease
	easeGain := ts.easeGain
	easeStep := ts.easeStep
	easingOut := ts.easingOut
	theta := ts.theta
	ts.mu.Unlock()

	frame := make([][]float32, samplesPerFrame)
	step := (frequency * 2.0 * math.Pi) / float64(samplerate)

	for n := 0; n < samplesPerFrame; n++ {
		theta += step
		amplitude := float32(math.Sin(theta) * currentGain * easeGain)

		frame[n] = make([]float32, channels)
		for c := 0; c < channels; c++ {
			frame[n][c] = amplitude
		}

		if gain > currentGain {
			currentGain += gainStep
			if currentGain > gain {
				currentGain = gain
			}
		} else if gain < currentGain {
			currentGain -= gainStep
			if currentGain < gain {
				currentGain = gain
			}
		}

		if ease {
			if easeGain < 1.0 && !easingOut {
				easeGain += easeStep
				if easeGain > 1.0 {
					easeGain = 1.0
				}
			} else if easingOut && easeGain > 0.0 {
				easeGain -= easeStep
				if easeGain <= 0.0 {
					easeGain = 0.0
					easingOut = false
					ts.mu.Lock()
					ts.shouldRun = false
					ts.mu.Unlock()
				}
			}
		}
	}

	ts.mu.Lock()
	ts.theta = theta
	ts.currentGain = currentGain
	ts.easeGain = easeGain
	ts.easingOut = easingOut
	ts.mu.Unlock()

	return frame
}

func (ts *ToneSource) generateJobWithThread(thread *generateThreadInfo) {
	defer thread.wg.Done()

	for {
		select {
		case <-thread.done:
			return
		default:
		}

		ts.mu.Lock()
		shouldRun := ts.shouldRun
		codec := ts.codec
		sink := ts.sink
		frameTime := ts.frameTime
		ts.mu.Unlock()

		if !shouldRun {
			return
		}

		if codec != nil && !codecs.IsNullCodec(codec) && sink != nil && sink.CanReceive(ts) {
			frame := ts.generate()
			encoded := codec.Encode(frame)
			if len(encoded) > 0 && sink.CanReceive(ts) {
				_ = sink.HandleEncodedFrame(encoded, ts)
			}
		} else if sink != nil && sink.CanReceive(ts) {
			frame := ts.generate()
			_ = sink.HandleFrame(frame, ts)
		}

		time.Sleep(time.Duration(frameTime * float64(time.Second) * 0.1))
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

func (ts *ToneSource) EaseTimeMs() float64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.easeTimeMs
}

func (ts *ToneSource) SampleRate() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.samplerate
}

func (ts *ToneSource) Channels() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.channels
}

func (ts *ToneSource) SamplesPerFrame() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.samplesPerFrame
}

func (ts *ToneSource) TargetFrameMs() float64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
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

// HandleEncodedFrame handles already-encoded audio data. ToneSource
// is an audio generator and does not process incoming encoded data.
func (ts *ToneSource) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

// Sink returns the current output destination for this ToneSource.
func (ts *ToneSource) Sink() sources.LocalSource {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.sink
}

// SetSink sets the output destination for this ToneSource, matching
// the Python LocalSource.sink property setter.
func (ts *ToneSource) SetSink(sink sources.LocalSource) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.sink = sink
}
