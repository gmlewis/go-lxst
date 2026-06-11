// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package mp3 provides MP3 audio file decoding for the LXST audio
// processing library. It uses github.com/hajimehoshi/go-mp3 (pure-Go)
// for decoding, making it compatible with CGO_ENABLED=0 builds.
package mp3

import (
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/sources"
	mp3dec "github.com/hajimehoshi/go-mp3"
)

var (
	ErrInvalidFilePath   = errors.New("invalid file path")
	ErrFileNotFound      = errors.New("file not found")
	ErrNoSamples         = errors.New("no samples in file")
	ErrDecodeFailed      = errors.New("failed to decode MP3")
	ErrMP3FileNotRunning = errors.New("mp3 file source not running")
	ErrAlreadyRunning    = errors.New("mp3 file source already running")
)

const (
	MP3FileSourceMaxFrames      = 128
	MP3FileSourceDefaultFrameMs = 100.0
)

// DecodeMP3 decodes an MP3 stream into float32 samples, returning the
// sample rate, channel count, and decoded samples. Each sample is a
// []float32 with one element per channel (interleaved PCM is split
// into per-sample channel arrays).
func DecodeMP3(r io.ReadCloser) (int, int, [][]float32, error) {
	decoder, err := mp3dec.NewDecoder(r)
	if err != nil {
		r.Close()
		return 0, 0, nil, err
	}

	sampleRate := decoder.SampleRate()

	// go-mp3 always decodes to stereo (2 channels) PCM int16
	channels := 2

	var pcm []byte
	buf := make([]byte, 4096)
	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			pcm = append(pcm, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	if len(pcm) < 4 {
		return 0, 0, nil, ErrNoSamples
	}

	sampleCount := len(pcm) / (channels * 2)
	samples := make([][]float32, sampleCount)
	for i := 0; i < sampleCount; i++ {
		samples[i] = make([]float32, channels)
		for ch := 0; ch < channels; ch++ {
			idx := (i*channels + ch) * 2
			if idx+1 < len(pcm) {
				val := int16(pcm[idx]) | int16(pcm[idx+1])<<8
				samples[i][ch] = float32(val) / 32768.0
			}
		}
	}

	return sampleRate, channels, samples, nil
}

type mp3ThreadInfo struct {
	done chan struct{}
	wg   sync.WaitGroup
}

// MP3FileSource implements an audio source that reads from an MP3 file.
type MP3FileSource struct {
	mu              sync.Mutex
	targetFrameMs   float64
	loop            bool
	timed           bool
	shouldRun       bool
	ingestThread    *mp3ThreadInfo
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

// NewMP3FileSource creates a new MP3 file source. It decodes the MP3
// file immediately on construction. If targetFrameMs is zero or
// negative, the default (100ms) is used.
func NewMP3FileSource(filePath string, targetFrameMs float64, loop bool, codec codecs.Codec, sink sources.LocalSource, timed bool) (*MP3FileSource, error) {
	if filePath == "" {
		return nil, ErrInvalidFilePath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}

	if targetFrameMs <= 0 {
		targetFrameMs = MP3FileSourceDefaultFrameMs
	}

	src := &MP3FileSource{
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

func (src *MP3FileSource) loadFile() error {
	f, err := os.Open(src.filePath)
	if err != nil {
		return err
	}

	sampleRate, channels, samples, err := DecodeMP3(f)
	if err != nil {
		return err
	}

	src.samplerate = sampleRate
	src.channels = channels
	src.samples = samples
	src.sampleCount = len(samples)
	src.lengthMs = (float64(src.sampleCount) / float64(src.samplerate)) * 1000.0

	return nil
}

func (src *MP3FileSource) applyCodecConstraints() {
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

func (src *MP3FileSource) Start() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.shouldRun {
		return ErrAlreadyRunning
	}

	src.shouldRun = true
	src.ingestThread = &mp3ThreadInfo{
		done: make(chan struct{}),
	}
	src.ingestThread.wg.Add(1)
	go src.ingestJob()

	return nil
}

func (src *MP3FileSource) Stop() error {
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

func (src *MP3FileSource) Running() bool {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.shouldRun
}

func (src *MP3FileSource) ingestJob() {
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

func (src *MP3FileSource) SetCodec(codec codecs.Codec) error {
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

func (src *MP3FileSource) GetCodec() codecs.Codec {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.codec
}

func (src *MP3FileSource) SampleRate() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.samplerate
}

func (src *MP3FileSource) Channels() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.channels
}

func (src *MP3FileSource) SampleCount() int {
	return src.sampleCount
}

func (src *MP3FileSource) SamplesPerFrame() int {
	return src.samplesPerFrame
}

func (src *MP3FileSource) TargetFrameMs() float64 {
	return src.targetFrameMs
}

func (src *MP3FileSource) FrameTime() float64 {
	return src.frameTime
}

func (src *MP3FileSource) Loop() bool {
	return src.loop
}

func (src *MP3FileSource) LengthMs() float64 {
	return src.lengthMs
}

func (src *MP3FileSource) CanReceive(fromSource sources.Source) bool {
	return true
}

func (src *MP3FileSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (src *MP3FileSource) GetSink() sources.LocalSource {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.sink
}

func (src *MP3FileSource) SetSink(sink sources.LocalSource) {
	src.mu.Lock()
	defer src.mu.Unlock()
	src.sink = sink
}

// Samples returns the decoded sample data.
func (src *MP3FileSource) Samples() [][]float32 {
	return src.samples
}
