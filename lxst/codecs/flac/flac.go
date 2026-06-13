// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package flac provides FLAC audio file decoding for the LXST audio
// processing library. It uses github.com/mewkiz/flac (pure-Go) for
// decoding, making it compatible with CGO_ENABLED=0 builds.
package flac

import (
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
	"github.com/mewkiz/flac"
)

var (
	ErrInvalidFilePath = errors.New("invalid file path")
	ErrFileNotFound    = errors.New("file not found")
	ErrNoSamples       = errors.New("no samples in file")
	ErrDecodeFailed    = errors.New("failed to decode FLAC")
	ErrAlreadyRunning  = errors.New("flac file source already running")
)

const (
	FLACFileSourceDefaultFrameMs = 100.0
)

// DecodeFLAC decodes a FLAC stream into float32 samples, returning the
// sample rate, channel count, and decoded samples.
func DecodeFLAC(r io.ReadCloser) (int, int, [][]float32, error) {
	dec, err := flac.New(r)
	if err != nil {
		_ = r.Close()
		return 0, 0, nil, err
	}

	sampleRate := int(dec.Info.SampleRate)
	channels := int(dec.Info.NChannels)
	bps := int(dec.Info.BitsPerSample)

	var allSamples [][]float32
	for {
		frame, err := dec.ParseNext()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, 0, nil, err
		}

		for i := 0; i < frame.Subframes[0].NSamples; i++ {
			sample := make([]float32, channels)
			for ch := 0; ch < channels; ch++ {
				if ch < len(frame.Subframes) {
					val := frame.Subframes[ch].Samples[i]
					maxVal := int64(1) << (bps - 1)
					sample[ch] = float32(val) / float32(maxVal)
				}
			}
			allSamples = append(allSamples, sample)
		}
	}

	if len(allSamples) == 0 {
		return 0, 0, nil, ErrNoSamples
	}

	return sampleRate, channels, allSamples, nil
}

type flacThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

// FLACFileSource implements an audio source that reads from a FLAC file.
type FLACFileSource struct {
	mu              sync.Mutex
	targetFrameMs   float64
	loop            bool
	timed           bool
	shouldRun       bool
	ingestThread    *flacThreadInfo
	readLock        sync.Mutex
	nextFrame       time.Time
	codec           codecs.Codec
	sink            sources.LocalSource
	samplerate      int
	channels        int
	bitdepth        int
	samples         [][]float32
	sampleCount     int
	lengthMs        float64
	samplesPerFrame int
	frameTime       float64
	filePath        string
}

// NewFLACFileSource creates a new FLAC file source. It decodes the FLAC
// file immediately on construction.
func NewFLACFileSource(filePath string, targetFrameMs float64, loop bool, codec codecs.Codec, sink sources.LocalSource, timed bool) (*FLACFileSource, error) {
	if filePath == "" {
		return nil, ErrInvalidFilePath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}

	if targetFrameMs <= 0 {
		targetFrameMs = FLACFileSourceDefaultFrameMs
	}

	src := &FLACFileSource{
		filePath:      filePath,
		targetFrameMs: targetFrameMs,
		loop:          loop,
		timed:         timed,
		codec:         codec,
		sink:          sink,
	}

	if err := src.loadFile(); err != nil {
		return nil, err
	}

	src.samplesPerFrame = int(math.Ceil((src.targetFrameMs / 1000.0) * float64(src.samplerate)))
	src.frameTime = float64(src.samplesPerFrame) / float64(src.samplerate)

	if codec != nil {
		src.applyCodecConstraints()
	}

	return src, nil
}

func (src *FLACFileSource) loadFile() error {
	f, err := os.Open(src.filePath)
	if err != nil {
		return err
	}

	sampleRate, channels, samples, err := DecodeFLAC(f)
	if err != nil {
		return err
	}

	src.samplerate = sampleRate
	src.channels = channels
	src.samples = samples
	src.sampleCount = len(samples)
	if sampleRate > 0 {
		src.lengthMs = (float64(src.sampleCount) / float64(src.samplerate)) * 1000.0
	}

	return nil
}

func (src *FLACFileSource) applyCodecConstraints() {
	if src.codec == nil {
		return
	}

	if quanta := src.codec.FrameQuantumMs(); quanta > 0 {
		if math.Mod(src.targetFrameMs, quanta) != 0 {
			src.targetFrameMs = math.Ceil(src.targetFrameMs/quanta) * quanta
		}
	}

	if maxMs := src.codec.FrameMaxMs(); maxMs > 0 && src.targetFrameMs > maxMs {
		src.targetFrameMs = maxMs
	}

	if valid := src.codec.ValidFrameMs(); len(valid) > 0 {
		closest := valid[0]
		minDiff := math.Abs(src.targetFrameMs - valid[0])
		for _, v := range valid[1:] {
			diff := math.Abs(src.targetFrameMs - v)
			if diff < minDiff {
				minDiff = diff
				closest = v
			}
		}
		src.targetFrameMs = closest
	}

	src.samplesPerFrame = int(math.Ceil((src.targetFrameMs / 1000.0) * float64(src.samplerate)))
	src.frameTime = float64(src.samplesPerFrame) / float64(src.samplerate)
}

func (src *FLACFileSource) Start() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.shouldRun {
		return ErrAlreadyRunning
	}

	src.shouldRun = true
	src.ingestThread = &flacThreadInfo{
		done: make(chan struct{}),
	}
	src.ingestThread.wg.Add(1)
	go src.ingestJob()

	return nil
}

func (src *FLACFileSource) Stop() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if !src.shouldRun {
		return nil
	}

	src.shouldRun = false

	if src.ingestThread != nil {
		close(src.ingestThread.done)
		src.ingestThread.wg.Wait()
		src.ingestThread = nil
	}

	return nil
}

func (src *FLACFileSource) Running() bool {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.shouldRun
}

func (src *FLACFileSource) ingestJob() {
	defer src.ingestThread.wg.Done()

	src.readLock.Lock()
	defer src.readLock.Unlock()

	src.nextFrame = time.Now()
	fi := 0
	spf := src.samplesPerFrame
	sc := src.sampleCount

	for {
		select {
		case <-src.ingestThread.done:
			return
		default:
		}

		if !src.shouldRun {
			return
		}

		src.mu.Lock()
		canReceive := src.sink != nil && src.sink.CanReceive(src)
		timedOK := !src.timed || time.Now().After(src.nextFrame)
		src.mu.Unlock()

		if canReceive && timedOK {
			src.nextFrame = time.Now().Add(time.Duration(src.frameTime * float64(time.Second)))
			fi++
			fs := (fi - 1) * spf
			fe := fi * spf
			if fe > sc {
				fe = sc
			}

			var frame [][]float32
			if fs >= sc {
				if src.loop {
					fi = 0
					continue
				} else {
					src.shouldRun = false
					return
				}
			} else {
				frame = src.samples[fs:fe]
			}

			if len(frame) > 0 {
				if src.codec != nil {
					encoded := src.codec.Encode(frame)
					if len(encoded) > 0 && src.sink != nil && src.sink.CanReceive(src) {
						_ = src.sink.HandleFrame(frame, src)
					}
				} else if src.sink != nil {
					_ = src.sink.HandleFrame(frame, src)
				}
			}
		} else {
			time.Sleep(time.Duration(src.frameTime * float64(time.Second) * 0.1))
		}
	}
}

func (src *FLACFileSource) SetCodec(codec codecs.Codec) error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if codec == nil {
		src.codec = nil
		return nil
	}

	src.codec = codec
	src.applyCodecConstraints()
	return nil
}

func (src *FLACFileSource) GetCodec() codecs.Codec {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.codec
}

func (src *FLACFileSource) SampleRate() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.samplerate
}

func (src *FLACFileSource) Channels() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.channels
}

func (src *FLACFileSource) SampleCount() int {
	return src.sampleCount
}

func (src *FLACFileSource) SamplesPerFrame() int {
	return src.samplesPerFrame
}

func (src *FLACFileSource) TargetFrameMs() float64 {
	return src.targetFrameMs
}

func (src *FLACFileSource) FrameTime() float64 {
	return src.frameTime
}

func (src *FLACFileSource) Loop() bool {
	return src.loop
}

func (src *FLACFileSource) LengthMs() float64 {
	return src.lengthMs
}

func (src *FLACFileSource) CanReceive(fromSource sources.Source) bool {
	return true
}

func (src *FLACFileSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (src *FLACFileSource) GetSink() sources.LocalSource {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.sink
}

func (src *FLACFileSource) SetSink(sink sources.LocalSource) {
	src.mu.Lock()
	defer src.mu.Unlock()
	src.sink = sink
}

// Samples returns the decoded sample data.
func (src *FLACFileSource) Samples() [][]float32 {
	return src.samples
}
