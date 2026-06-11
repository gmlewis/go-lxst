// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package vorbis provides Vorbis audio file decoding for the LXST audio
// processing library. It uses github.com/jfreymuth/oggvorbis (pure-Go)
// for decoding Ogg Vorbis files, making it compatible with CGO_ENABLED=0
// builds.
package vorbis

import (
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
	oggvorbis "github.com/jfreymuth/oggvorbis"
)

var (
	ErrInvalidFilePath = errors.New("invalid file path")
	ErrFileNotFound    = errors.New("file not found")
	ErrNoSamples       = errors.New("no samples in file")
	ErrDecodeFailed    = errors.New("failed to decode Vorbis")
	ErrAlreadyRunning  = errors.New("vorbis file source already running")
)

const (
	VorbisFileSourceDefaultFrameMs = 100.0
)

// DecodeVorbis decodes an Ogg Vorbis stream into float32 samples, returning
// the sample rate, channel count, and decoded samples. Each sample is a
// []float32 with one element per channel.
func DecodeVorbis(r io.ReadCloser) (int, int, [][]float32, error) {
	decoder, err := oggvorbis.NewReader(r)
	if err != nil {
		r.Close()
		return 0, 0, nil, err
	}

	sampleRate := decoder.SampleRate()
	channels := decoder.Channels()

	// Read all samples
	bufSize := 4096
	buf := make([]float32, bufSize)
	var allPCM []float32
	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			allPCM = append(allPCM, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, 0, nil, err
		}
	}

	if len(allPCM) < channels {
		return 0, 0, nil, ErrNoSamples
	}

	// Deinterleave: oggvorbis returns interleaved samples
	sampleCount := len(allPCM) / channels
	samples := make([][]float32, sampleCount)
	for i := 0; i < sampleCount; i++ {
		samples[i] = make([]float32, channels)
		for ch := 0; ch < channels; ch++ {
			samples[i][ch] = allPCM[i*channels+ch]
		}
	}

	return sampleRate, channels, samples, nil
}

type vorbisThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

// VorbisFileSource implements an audio source that reads from an Ogg Vorbis
// file, decoding it on construction.
type VorbisFileSource struct {
	mu              sync.Mutex
	targetFrameMs   float64
	loop            bool
	timed           bool
	shouldRun       bool
	ingestThread    *vorbisThreadInfo
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

// NewVorbisFileSource creates a new Vorbis file source. It decodes the
// Ogg Vorbis file immediately on construction.
func NewVorbisFileSource(filePath string, targetFrameMs float64, loop bool, codec codecs.Codec, sink sources.LocalSource, timed bool) (*VorbisFileSource, error) {
	if filePath == "" {
		return nil, ErrInvalidFilePath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}

	if targetFrameMs <= 0 {
		targetFrameMs = VorbisFileSourceDefaultFrameMs
	}

	src := &VorbisFileSource{
		filePath:      filePath,
		targetFrameMs: targetFrameMs,
		loop:          loop,
		timed:         timed,
		codec:         codec,
		sink:          sink,
		bitdepth:      16,
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

func (src *VorbisFileSource) loadFile() error {
	f, err := os.Open(src.filePath)
	if err != nil {
		return err
	}

	sampleRate, channels, samples, err := DecodeVorbis(f)
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

func (src *VorbisFileSource) applyCodecConstraints() {
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

func (src *VorbisFileSource) Start() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.shouldRun {
		return ErrAlreadyRunning
	}

	src.shouldRun = true
	src.ingestThread = &vorbisThreadInfo{
		done: make(chan struct{}),
	}
	src.ingestThread.wg.Add(1)
	go src.ingestJob()

	return nil
}

func (src *VorbisFileSource) Stop() error {
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

func (src *VorbisFileSource) Running() bool {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.shouldRun
}

func (src *VorbisFileSource) ingestJob() {
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
						src.sink.HandleFrame(frame, src)
					}
				} else if src.sink != nil {
					src.sink.HandleFrame(frame, src)
				}
			}
		} else {
			time.Sleep(time.Duration(src.frameTime * float64(time.Second) * 0.1))
		}
	}
}

func (src *VorbisFileSource) SetCodec(codec codecs.Codec) error {
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

func (src *VorbisFileSource) GetCodec() codecs.Codec {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.codec
}

func (src *VorbisFileSource) SampleRate() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.samplerate
}

func (src *VorbisFileSource) Channels() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.channels
}

func (src *VorbisFileSource) SampleCount() int {
	return src.sampleCount
}

func (src *VorbisFileSource) SamplesPerFrame() int {
	return src.samplesPerFrame
}

func (src *VorbisFileSource) TargetFrameMs() float64 {
	return src.targetFrameMs
}

func (src *VorbisFileSource) FrameTime() float64 {
	return src.frameTime
}

func (src *VorbisFileSource) Loop() bool {
	return src.loop
}

func (src *VorbisFileSource) LengthMs() float64 {
	return src.lengthMs
}

func (src *VorbisFileSource) CanReceive(fromSource sources.Source) bool {
	return true
}

func (src *VorbisFileSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (src *VorbisFileSource) GetSink() sources.LocalSource {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.sink
}

func (src *VorbisFileSource) SetSink(sink sources.LocalSource) {
	src.mu.Lock()
	defer src.mu.Unlock()
	src.sink = sink
}

// Samples returns the decoded sample data.
func (src *VorbisFileSource) Samples() [][]float32 {
	return src.samples
}
