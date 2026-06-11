// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package sources

import (
	"errors"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/youpy/go-wav"
)

var (
	ErrInvalidFilePath    = errors.New("invalid file path")
	ErrFileNotFound       = errors.New("file not found")
	ErrNoSamples          = errors.New("no samples in file")
	ErrOpusFileNotRunning = errors.New("opus file source not running")
)

const (
	OpusFileSourceMaxFrames      = 128
	OpusFileSourceDefaultFrameMs = 100.0
	TypeMapFactor                = 32767.0
)

type OpusFileSource struct {
	mu              sync.Mutex
	targetFrameMs   float64
	loop            bool
	timed           bool
	shouldRun       bool
	ingestThread    *threadInfo
	readLock        sync.Mutex
	nextFrame       time.Time
	codec           codecs.Codec
	sink            LocalSource
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

func NewOpusFileSource(filePath string, targetFrameMs float64, loop bool, codec codecs.Codec, sink LocalSource, timed bool) (*OpusFileSource, error) {
	if filePath == "" {
		return nil, ErrInvalidFilePath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}

	if targetFrameMs <= 0 {
		targetFrameMs = OpusFileSourceDefaultFrameMs
	}

	src := &OpusFileSource{
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

func (src *OpusFileSource) loadFile() error {
	f, err := os.Open(src.filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	wavReader := wav.NewReader(f)
	fmt, err := wavReader.Format()
	if err != nil {
		return err
	}

	src.samplerate = int(fmt.SampleRate)
	src.channels = int(fmt.NumChannels)

	if fmt.AudioFormat == 1 {
		src.bitdepth = int(fmt.BitsPerSample)
	} else {
		src.bitdepth = 16
	}

	var allSamples []wav.Sample
	chunkSize := 8192
	for {
		chunk, err := wavReader.ReadSamples(uint32(chunkSize))
		if err != nil {
			break
		}
		if len(chunk) == 0 {
			break
		}
		allSamples = append(allSamples, chunk...)
		if len(chunk) < chunkSize {
			break
		}
	}

	if len(allSamples) == 0 {
		return ErrNoSamples
	}

	src.sampleCount = len(allSamples)
	src.samples = make([][]float32, src.sampleCount)
	for i, s := range allSamples {
		src.samples[i] = make([]float32, src.channels)
		for ch := 0; ch < src.channels; ch++ {
			val := wavReader.FloatValue(s, uint(ch))
			if val > 1.0 {
				val = 1.0
			} else if val < -1.0 {
				val = -1.0
			}
			src.samples[i][ch] = float32(val)
		}
	}

	src.lengthMs = (float64(src.sampleCount) / float64(src.samplerate)) * 1000.0

	return nil
}

func (src *OpusFileSource) applyCodecConstraints() {
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

func (src *OpusFileSource) Start() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.shouldRun {
		return ErrSourceAlreadyRunning
	}

	src.shouldRun = true
	src.ingestThread = &threadInfo{
		done: make(chan struct{}),
	}
	src.ingestThread.wg.Add(1)
	go src.ingestJob()

	return nil
}

func (src *OpusFileSource) Stop() error {
	src.mu.Lock()
	defer src.mu.Unlock()

	running := src.shouldRun
	if !running {
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

func (src *OpusFileSource) Running() bool {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.shouldRun
}

func (src *OpusFileSource) ingestJob() {
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

func (src *OpusFileSource) SetCodec(codec codecs.Codec) error {
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

func (src *OpusFileSource) GetCodec() codecs.Codec {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.codec
}

func (src *OpusFileSource) SampleRate() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.samplerate
}

func (src *OpusFileSource) Channels() int {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.channels
}

func (src *OpusFileSource) SampleCount() int {
	return src.sampleCount
}

func (src *OpusFileSource) SamplesPerFrame() int {
	return src.samplesPerFrame
}

func (src *OpusFileSource) TargetFrameMs() float64 {
	return src.targetFrameMs
}

func (src *OpusFileSource) FrameTime() float64 {
	return src.frameTime
}

func (src *OpusFileSource) Loop() bool {
	return src.loop
}

func (src *OpusFileSource) LengthMs() float64 {
	return src.lengthMs
}

func (src *OpusFileSource) CanReceive(fromSource Source) bool {
	return true
}

func (src *OpusFileSource) HandleFrame(frame [][]float32, fromSource Source) error {
	return nil
}

func (src *OpusFileSource) GetSink() LocalSource {
	src.mu.Lock()
	defer src.mu.Unlock()
	return src.sink
}

func (src *OpusFileSource) SetSink(sink LocalSource) {
	src.mu.Lock()
	defer src.mu.Unlock()
	src.sink = sink
}
