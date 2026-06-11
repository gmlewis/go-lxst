// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package mixer provides audio mixing functionality.
package mixer

import (
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

const (
	MixerMaxFrames = 8
)

type Mixer struct {
	mu              sync.Mutex
	insertLock      sync.Mutex
	mixerLock       sync.Mutex
	targetFrameMs   float64
	frameTime       float64
	shouldRun       bool
	mixerThread     *mixerThreadInfo
	incomingFrames  map[sources.Source]*sourceQueue
	muted           bool
	gain            float64
	bitdepth        int
	channels        int
	samplerate      int
	samplesPerFrame int
	codec           codecs.Codec
	sink            sources.LocalSource
	source          sources.Source
}

type sourceQueue struct {
	frames    [][][]float32
	maxFrames int
}

type mixerThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

func NewMixer(targetFrameMs float64, samplerate int, codec codecs.Codec, sink sources.LocalSource, gain float64) *Mixer {
	if targetFrameMs <= 0 {
		targetFrameMs = 40.0
	}

	m := &Mixer{
		targetFrameMs:  targetFrameMs,
		frameTime:      targetFrameMs / 1000.0,
		incomingFrames: make(map[sources.Source]*sourceQueue),
		gain:           gain,
		bitdepth:       32,
		sink:           sink,
		codec:          codec,
	}

	if samplerate > 0 {
		m.samplerate = samplerate
		m.samplesPerFrame = int(math.Ceil((targetFrameMs / 1000.0) * float64(samplerate)))
		m.frameTime = float64(m.samplesPerFrame) / float64(samplerate)
	}

	if codec != nil {
		m.applyCodecConstraints()
	}

	return m
}

func (m *Mixer) applyCodecConstraints() {
	if m.codec == nil {
		return
	}

	if quanta := m.codec.FrameQuantumMs(); quanta > 0 {
		if math.Mod(m.targetFrameMs, quanta) != 0 {
			m.targetFrameMs = math.Ceil(m.targetFrameMs/quanta) * quanta
		}
	}

	if maxMs := m.codec.FrameMaxMs(); maxMs > 0 && m.targetFrameMs > maxMs {
		m.targetFrameMs = maxMs
	}

	if valid := m.codec.ValidFrameMs(); len(valid) > 0 {
		closest := valid[0]
		minDiff := math.Abs(m.targetFrameMs - valid[0])
		for _, v := range valid[1:] {
			diff := math.Abs(m.targetFrameMs - v)
			if diff < minDiff {
				minDiff = diff
				closest = v
			}
		}
		m.targetFrameMs = closest
	}

	if m.samplerate > 0 {
		m.samplesPerFrame = int(math.Ceil((m.targetFrameMs / 1000.0) * float64(m.samplerate)))
		m.frameTime = float64(m.samplesPerFrame) / float64(m.samplerate)
	}
}

func (m *Mixer) mixingGain() float64 {
	if m.muted {
		return 0.0
	}
	if m.gain == 0.0 {
		return 1.0
	}
	return math.Pow(10, m.gain/10.0)
}

func (m *Mixer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldRun {
		return nil
	}

	m.shouldRun = true

	m.mixerThread = &mixerThreadInfo{
		done: make(chan struct{}),
	}
	m.mixerThread.wg.Add(1)
	go m.mixerJob()

	return nil
}

func (m *Mixer) Stop() error {
	m.mu.Lock()
	m.shouldRun = false
	m.mu.Unlock()
	return nil
}

func (m *Mixer) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shouldRun
}

func (m *Mixer) SetGain(gain float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gain = gain
}

func (m *Mixer) Mute() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted = true
}

func (m *Mixer) Unmute() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted = false
}

func (m *Mixer) IsMuted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.muted
}

func (m *Mixer) SetSourceMaxFrames(src sources.Source, maxFrames int) {
	m.insertLock.Lock()
	defer m.insertLock.Unlock()

	if q, ok := m.incomingFrames[src]; ok {
		q.maxFrames = maxFrames
	} else {
		m.incomingFrames[src] = &sourceQueue{
			frames:    make([][][]float32, 0),
			maxFrames: maxFrames,
		}
	}
}

func (m *Mixer) CanReceive(fromSource sources.Source) bool {
	m.insertLock.Lock()
	defer m.insertLock.Unlock()

	if q, ok := m.incomingFrames[fromSource]; ok {
		return len(q.frames) < MixerMaxFrames
	}
	return true
}

func (m *Mixer) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	m.insertLock.Lock()

	if _, ok := m.incomingFrames[fromSource]; !ok {
		maxFrames := MixerMaxFrames
		m.incomingFrames[fromSource] = &sourceQueue{
			frames:    make([][][]float32, 0),
			maxFrames: maxFrames,
		}

		if m.channels == 0 {
			if src, ok := fromSource.(interface{ Channels() int }); ok {
				m.channels = src.Channels()
			}
		}

		if m.samplerate == 0 {
			if src, ok := fromSource.(interface{ SampleRate() int }); ok {
				m.samplerate = src.SampleRate()
				m.samplesPerFrame = int(math.Ceil((m.targetFrameMs / 1000.0) * float64(m.samplerate)))
				m.frameTime = float64(m.samplesPerFrame) / float64(m.samplerate)
			}
		}
	}

	q := m.incomingFrames[fromSource]
	if len(q.frames) < q.maxFrames {
		q.frames = append(q.frames, frame)
	}

	m.insertLock.Unlock()
	return nil
}

func (m *Mixer) mixerJob() {
	defer m.mixerThread.wg.Done()

	m.mixerLock.Lock()
	defer m.mixerLock.Unlock()

	for {
		select {
		case <-m.mixerThread.done:
			return
		default:
		}

		if !m.shouldRun {
			return
		}

		m.mu.Lock()
		sinkOk := m.sink != nil
		m.mu.Unlock()

		if sinkOk && m.sink.CanReceive(m) {
			m.insertLock.Lock()
			sourceCount := 0
			var mixedFrame [][]float32

			for src, q := range m.incomingFrames {
				if len(q.frames) > 0 {
					nextFrame := q.frames[0]
					q.frames = q.frames[1:]

					g := m.mixingGain()

					if sourceCount == 0 {
						mixedFrame = make([][]float32, len(nextFrame))
						for i := range nextFrame {
							mixedFrame[i] = make([]float32, len(nextFrame[i]))
							for j := range nextFrame[i] {
								mixedFrame[i][j] = nextFrame[i][j] * float32(g)
							}
						}
					} else {
						for i := range nextFrame {
							if i < len(mixedFrame) {
								for j := range nextFrame[i] {
									if j < len(mixedFrame[i]) {
										mixedFrame[i][j] += nextFrame[i][j] * float32(g)
									}
								}
							}
						}
					}
					sourceCount++
				}
				_ = src
			}
			m.insertLock.Unlock()

			if sourceCount > 0 {
				for i := range mixedFrame {
					for j := range mixedFrame[i] {
						if mixedFrame[i][j] > 1.0 {
							mixedFrame[i][j] = 1.0
						} else if mixedFrame[i][j] < -1.0 {
							mixedFrame[i][j] = -1.0
						}
					}
				}

				m.mu.Lock()
				codec := m.codec
				sink := m.sink
				m.mu.Unlock()

				if codec != nil {
					encoded := codec.Encode(mixedFrame)
					if len(encoded) > 0 && sink != nil && sink.CanReceive(m) {
						_ = sink.HandleFrame(mixedFrame, m)
					}
				} else if sink != nil {
					_ = sink.HandleFrame(mixedFrame, m)
				}
			} else {
				time.Sleep(time.Duration(m.frameTime * float64(time.Second) * 0.1))
			}
		} else {
			time.Sleep(time.Duration(m.frameTime * float64(time.Second) * 0.1))
		}
	}
}

func (m *Mixer) SetCodec(codec codecs.Codec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codec = codec
	m.applyCodecConstraints()
	return nil
}

func (m *Mixer) GetCodec() codecs.Codec {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codec
}

func (m *Mixer) Gain() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gain
}

func (m *Mixer) TargetFrameMs() float64 {
	return m.targetFrameMs
}

func (m *Mixer) SampleRate() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.samplerate
}

func (m *Mixer) Channels() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.channels
}

func (m *Mixer) GetSource() sources.Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.source
}

func (m *Mixer) SetSource(src sources.Source) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.source = src
}

// Ensure Mixer implements sources.LocalSource
var _ sources.LocalSource = (*Mixer)(nil)
