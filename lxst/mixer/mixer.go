// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package mixer provides audio mixing functionality for the LXST library.
// The Mixer combines multiple audio sources into a single output stream,
// applying per-source gain and optional codec encoding. It supports
// configurable frame timing, codec constraints, and graceful start/stop
// with configurable buffer sizes per source.
package mixer

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// framePool is a package-level pool for reusable frame buffers.
// It reduces allocations in the hot path by recycling frames
// that are no longer in use. The pool is keyed by frame size
// to handle different sample rates and channel counts.
var framePool = sync.Pool{
	New: func() any {
		return &framePoolEntry{}
	},
}

type framePoolEntry struct {
	frame [][]float32
}

// getFrame retrieves a zeroed frame from the pool with the specified
// dimensions. If no suitable frame is available, a new one is allocated.
func getFrame(rows, cols int) [][]float32 {
	entry := framePool.Get().(*framePoolEntry)
	if cap(entry.frame) >= rows {
		entry.frame = entry.frame[:rows]
	} else {
		entry.frame = make([][]float32, rows)
	}
	for i := range entry.frame {
		if cap(entry.frame[i]) >= cols {
			entry.frame[i] = entry.frame[i][:cols]
		} else {
			entry.frame[i] = make([]float32, cols)
		}
		for j := range entry.frame[i] {
			entry.frame[i][j] = 0
		}
	}
	frame := entry.frame
	entry.frame = nil
	framePool.Put(entry)
	return frame
}

// putFrame returns a frame to the pool for reuse.
// The frame should not be used after calling putFrame.
func putFrame(frame [][]float32) {
	if frame == nil {
		return
	}
	entry := framePool.Get().(*framePoolEntry)
	entry.frame = frame
	framePool.Put(entry)
}

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
	m.mu.Lock()
	defer m.mu.Unlock()
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

	thread := &mixerThreadInfo{
		done: make(chan struct{}),
	}
	m.mixerThread = thread
	thread.wg.Add(1)
	go m.mixerJobWithThread(thread)

	return nil
}

func (m *Mixer) Stop() error {
	m.mu.Lock()
	if !m.shouldRun {
		m.mu.Unlock()
		return nil
	}
	m.shouldRun = false

	var thread *mixerThreadInfo
	if m.mixerThread != nil {
		thread = m.mixerThread
		m.mixerThread = nil
		close(thread.done)
	}
	m.mu.Unlock()

	if thread != nil {
		thread.wg.Wait()
	}
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
		log.Printf("Mixer.HandleFrame: registering new source %T (channels=%d, samples=%d)", fromSource, func() int {
			if len(frame) > 0 {
				return len(frame[0])
			}
			return 0
		}(), len(frame))
		maxFrames := MixerMaxFrames
		m.incomingFrames[fromSource] = &sourceQueue{
			frames:    make([][][]float32, 0),
			maxFrames: maxFrames,
		}

		m.mu.Lock()
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
		m.mu.Unlock()
	}

	q := m.incomingFrames[fromSource]
	if len(q.frames) < q.maxFrames {
		q.frames = append(q.frames, frame)
	}

	m.insertLock.Unlock()
	return nil
}

// HandleEncodedFrame decodes already-encoded audio data and inserts
// it into the mixing queue, matching the Python pattern where a
// source with a codec sends encoded data downstream.
func (m *Mixer) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	m.mu.Lock()
	codec := m.codec
	channels := m.channels
	m.mu.Unlock()

	if codec == nil || len(data) == 0 || channels == 0 {
		return nil
	}

	frame := codec.Decode(data, channels)
	if len(frame) == 0 {
		return nil
	}

	return m.HandleFrame(frame, fromSource)
}

func (m *Mixer) mixerJobWithThread(thread *mixerThreadInfo) {
	defer thread.wg.Done()

	m.mixerLock.Lock()
	defer m.mixerLock.Unlock()

	m.mu.Lock()
	samplerate := m.samplerate
	samplesPerFrame := m.samplesPerFrame
	frameTime := m.frameTime
	sinkNil := m.sink == nil
	codec := m.codec
	m.mu.Unlock()

	log.Printf("Mixer.mixerJob: starting (samplerate=%d, samplesPerFrame=%d, frameTime=%.4f, sink=%v, codec=%T)",
		samplerate, samplesPerFrame, frameTime, !sinkNil, codec)

	for {
		select {
		case <-thread.done:
			return
		default:
		}

		m.mu.Lock()
		shouldRun := m.shouldRun
		m.mu.Unlock()

		if !shouldRun {
			return
		}

		m.mu.Lock()
		sinkOk := m.sink != nil
		m.mu.Unlock()

		if sinkOk {
			m.insertLock.Lock()
			sourceCount := 0
			var mixedFrame [][]float32

			for _, q := range m.incomingFrames {
				if len(q.frames) > 0 {
					nextFrame := q.frames[0]
					q.frames = q.frames[1:]

					if len(nextFrame) == 0 {
						continue
					}

					g := m.mixingGain()

					if sourceCount == 0 {
						mixedFrame = getFrame(len(nextFrame), len(nextFrame[0]))
						for i := range nextFrame {
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
			}
			m.insertLock.Unlock()

			if sourceCount > 0 {
				log.Printf("Mixer.digestJob: processing %d source(s), mixedFrame samples=%d", sourceCount, len(mixedFrame))
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

				if codec != nil && !codecs.IsNullCodec(codec) {
					encoded := codec.Encode(mixedFrame)
					putFrame(mixedFrame)
					if len(encoded) > 0 && sink != nil && sink.CanReceive(m) {
						if err := sink.HandleEncodedFrame(encoded, m); err != nil {
							log.Printf("Mixer.digestJob: HandleEncodedFrame failed: %v", err)
						}
					} else if len(encoded) == 0 {
						log.Printf("Mixer.digestJob: codec.Encode returned empty (codec=%T, samples=%d)", codec, len(mixedFrame))
					}
				} else if sink != nil && sink.CanReceive(m) {
					if err := sink.HandleFrame(mixedFrame, m); err != nil {
						log.Printf("Mixer.digestJob: HandleFrame failed: %v", err)
					}
				} else {
					putFrame(mixedFrame)
				}
			} else {
				m.mu.Lock()
				ft := m.frameTime
				m.mu.Unlock()
				time.Sleep(time.Duration(ft * float64(time.Second) * 0.1))
			}
		} else {
			m.mu.Lock()
			ft := m.frameTime
			m.mu.Unlock()
			time.Sleep(time.Duration(ft * float64(time.Second) * 0.1))
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

// Sink returns the current output destination for this mixer, matching
// the Python Mixer.sink property getter.
func (m *Mixer) Sink() sources.LocalSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sink
}

// SetSink sets the output destination for this mixer, matching the
// Python Mixer.sink property setter. The Pipeline calls this during
// construction to wire the mixer to its downstream sink (Packetizer or
// LineSink).
func (m *Mixer) SetSink(sink sources.LocalSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sink = sink
}

// Ensure Mixer implements sources.LocalSource
var _ sources.LocalSource = (*Mixer)(nil)
