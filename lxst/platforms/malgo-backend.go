// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build cgo

// Package platforms provides platform-specific audio I/O backends.
// This file implements the malgo-based audio backend using miniaudio
// bindings via CGO. Build with CGO enabled to use this backend.
package platforms

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

const (
	malgoDefaultSampleRate = 48000
	malgoDefaultChannels   = 2
)

var (
	ErrMalgoInitFailed    = errors.New("malgo context initialization failed")
	ErrMalgoNotRecording  = errors.New("malgo recorder not active")
	ErrMalgoNotPlaying    = errors.New("malgo player not active")
	ErrMalgoRecorderInUse = errors.New("malgo recorder already in use")
	ErrMalgoPlayerInUse   = errors.New("malgo player already in use")
)

// MalgoBackend implements AudioBackend using github.com/gen2brain/malgo
// (miniaudio CGO bindings). Supports WASAPI, CoreAudio, ALSA,
// PulseAudio, JACK, AAudio, OpenSL ES backends.
type MalgoBackend struct {
	sampleRate int
	channels   int
	bitDepth   int

	mu           sync.Mutex
	ctx          *malgo.AllocatedContext
	recorder     *malgoRecorder
	player       *malgoPlayer
	recorderMu   sync.Mutex
	playerMu     sync.Mutex
	micNames     []string
	speakerNames []string
}

// malgoRecorder wraps malgo for audio recording (input).
type malgoRecorder struct {
	backend         *MalgoBackend
	sampleRate      int
	channels        int
	samplesPerFrame int
	device          *malgo.Device
	frameBuf        [][]float32
	frameMu         sync.Mutex
	frameReady      chan struct{}
	closed          bool
}

// malgoPlayer wraps malgo for audio playback (output).
type malgoPlayer struct {
	backend         *MalgoBackend
	sampleRate      int
	channels        int
	samplesPerFrame int
	device          *malgo.Device
	closed          bool
}

// NewMalgoBackend creates a new malgo-based audio backend.
// If sampleRate or channels are zero or negative, sensible defaults
// (48000 Hz, 2 channels) are used.
func NewMalgoBackend(sampleRate, channels, bitDepth int) (*MalgoBackend, error) {
	if sampleRate <= 0 {
		sampleRate = malgoDefaultSampleRate
	}
	if channels <= 0 {
		channels = malgoDefaultChannels
	}
	if channels > 2 {
		channels = 2
	}

	mb := &MalgoBackend{
		sampleRate: sampleRate,
		channels:   channels,
		bitDepth:   bitDepth,
	}

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {})
	if err != nil {
		return nil, err
	}
	mb.ctx = ctx

	mb.enumerateDevices()

	return mb, nil
}

func (mb *MalgoBackend) enumerateDevices() {
	playbackDevices, err := mb.ctx.Devices(malgo.Playback)
	if err == nil {
		for _, d := range playbackDevices {
			mb.speakerNames = append(mb.speakerNames, d.Name())
		}
	}

	captureDevices, err := mb.ctx.Devices(malgo.Capture)
	if err == nil {
		for _, d := range captureDevices {
			mb.micNames = append(mb.micNames, d.Name())
		}
	}
}

func (mb *MalgoBackend) SampleRate() int { return mb.sampleRate }
func (mb *MalgoBackend) Channels() int   { return mb.channels }
func (mb *MalgoBackend) BitDepth() int   { return mb.bitDepth }

func (mb *MalgoBackend) AllMicrophones() []string {
	result := make([]string, len(mb.micNames))
	copy(result, mb.micNames)
	return result
}

func (mb *MalgoBackend) DefaultMicrophone() string {
	if len(mb.micNames) > 0 {
		return mb.micNames[0]
	}
	return "default"
}

func (mb *MalgoBackend) AllSpeakers() []string {
	result := make([]string, len(mb.speakerNames))
	copy(result, mb.speakerNames)
	return result
}

func (mb *MalgoBackend) DefaultSpeaker() string {
	if len(mb.speakerNames) > 0 {
		return mb.speakerNames[0]
	}
	return "default"
}

func (mb *MalgoBackend) Flush() error { return nil }

func (mb *MalgoBackend) ReleaseRecorder() error {
	mb.recorderMu.Lock()
	defer mb.recorderMu.Unlock()

	if mb.recorder != nil {
		_ = mb.recorder.Close()
		mb.recorder = nil
	}
	return nil
}

func (mb *MalgoBackend) ReleasePlayer() error {
	mb.playerMu.Lock()
	defer mb.playerMu.Unlock()

	if mb.player != nil {
		_ = mb.player.Close()
		mb.player = nil
	}
	return nil
}

func (mb *MalgoBackend) GetRecorder(samplesPerFrame int) (AudioRecorder, error) {
	mb.recorderMu.Lock()
	defer mb.recorderMu.Unlock()

	if mb.recorder != nil {
		return nil, ErrMalgoRecorderInUse
	}

	rec := &malgoRecorder{
		backend:         mb,
		sampleRate:      mb.sampleRate,
		channels:        mb.channels,
		samplesPerFrame: samplesPerFrame,
		frameReady:      make(chan struct{}, 1),
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = uint32(mb.channels)
	deviceConfig.SampleRate = uint32(mb.sampleRate)

	onData := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		frames := make([][]float32, int(framecount))
		for i := range frames {
			frames[i] = make([]float32, mb.channels)
		}

		for i := 0; i < int(framecount); i++ {
			for ch := 0; ch < mb.channels; ch++ {
				idx := (i*mb.channels + ch) * 4
				if idx+3 < len(pInputSamples) {
					bits := uint32(pInputSamples[idx]) |
						uint32(pInputSamples[idx+1])<<8 |
						uint32(pInputSamples[idx+2])<<16 |
						uint32(pInputSamples[idx+3])<<24
					frames[i][ch] = *(*float32)(unsafe.Pointer(&bits))
				}
			}
		}

		rec.frameMu.Lock()
		rec.frameBuf = frames
		rec.frameMu.Unlock()

		select {
		case rec.frameReady <- struct{}{}:
		default:
		}
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onData,
	}

	device, err := malgo.InitDevice(mb.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, err
	}

	err = device.Start()
	if err != nil {
		device.Uninit()
		return nil, err
	}

	rec.device = device
	mb.recorder = rec

	return rec, nil
}

func (mb *MalgoBackend) GetPlayer(samplesPerFrame int, lowLatency bool) (AudioPlayer, error) {
	mb.playerMu.Lock()
	defer mb.playerMu.Unlock()

	if mb.player != nil {
		return nil, ErrMalgoPlayerInUse
	}

	pl := &malgoPlayer{
		backend:         mb,
		sampleRate:      mb.sampleRate,
		channels:        mb.channels,
		samplesPerFrame: samplesPerFrame,
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatF32
	deviceConfig.Playback.Channels = uint32(mb.channels)
	deviceConfig.SampleRate = uint32(mb.sampleRate)

	onData := func(pOutputSample, pInputSamples []byte, framecount uint32) {
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onData,
	}

	device, err := malgo.InitDevice(mb.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, err
	}

	err = device.Start()
	if err != nil {
		device.Uninit()
		return nil, err
	}

	pl.device = device
	mb.player = pl

	return pl, nil
}

// malgoRecorder implementation

func (mr *malgoRecorder) Record(numFrames int) ([][]float32, error) {
	mr.frameMu.Lock()
	if mr.closed {
		mr.frameMu.Unlock()
		return nil, ErrMalgoNotRecording
	}
	mr.frameMu.Unlock()

	select {
	case <-mr.frameReady:
		mr.frameMu.Lock()
		frames := mr.frameBuf
		mr.frameMu.Unlock()

		if len(frames) >= numFrames {
			return frames[:numFrames], nil
		}

		result := make([][]float32, numFrames)
		for i := range result {
			result[i] = make([]float32, mr.channels)
		}
		for i, f := range frames {
			copy(result[i], f)
		}
		return result, nil

	default:
		frame := make([][]float32, numFrames)
		for i := range frame {
			frame[i] = make([]float32, mr.channels)
		}
		return frame, nil
	}
}

func (mr *malgoRecorder) Close() error {
	mr.frameMu.Lock()
	defer mr.frameMu.Unlock()

	if !mr.closed {
		_ = mr.device.Stop()
		mr.device.Uninit()
		mr.closed = true
	}
	return nil
}

// malgoPlayer implementation

func (mp *malgoPlayer) Play(frame [][]float32) error {
	mp.backend.mu.Lock()
	defer mp.backend.mu.Unlock()

	if mp.closed {
		return ErrMalgoNotPlaying
	}

	if len(frame) == 0 || len(frame[0]) == 0 {
		return nil
	}

	numFrames := len(frame)
	numChannels := len(frame[0])
	bytesPerSample := 4
	buf := make([]byte, numFrames*numChannels*bytesPerSample)

	for i := 0; i < numFrames; i++ {
		for ch := 0; ch < numChannels; ch++ {
			val := frame[i][ch]
			bits := *(*uint32)(unsafe.Pointer(&val))
			offset := (i*numChannels + ch) * bytesPerSample
			buf[offset] = byte(bits)
			buf[offset+1] = byte(bits >> 8)
			buf[offset+2] = byte(bits >> 16)
			buf[offset+3] = byte(bits >> 24)
		}
	}

	return nil
}

func (mp *malgoPlayer) Close() error {
	if !mp.closed {
		_ = mp.device.Stop()
		mp.device.Uninit()
		mp.closed = true
	}
	return nil
}

func (mp *malgoPlayer) EnableLowLatency() error {
	return nil
}
