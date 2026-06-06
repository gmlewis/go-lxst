// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package network

import (
	"sync"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

const (
	FieldSignalling byte = 0x00
	FieldFrames     byte = 0x01

	CodeNull   byte = 0xFF
	CodeRaw    byte = 0x00
	CodeOpus   byte = 0x01
	CodeCodec2 byte = 0x02
)

var ErrUnknownCodecType error = &codecTypeError{}

type codecTypeError struct{}

func (e *codecTypeError) Error() string { return "unknown codec type" }

// CodecHeaderByte returns the single-byte codec identifier for a given codec.
func CodecHeaderByte(codec codecs.Codec) (byte, error) {
	switch codec.(type) {
	case *raw.Raw:
		return CodeRaw, nil
	case *opus.Opus:
		return CodeOpus, nil
	case *codec2.Codec2:
		return CodeCodec2, nil
	default:
		return 0, ErrUnknownCodecType
	}
}

// CodecTypeFromHeader returns a new codec instance based on the header byte.
func CodecTypeFromHeader(headerByte byte) (codecs.Codec, error) {
	switch headerByte {
	case CodeRaw:
		return raw.NewRaw(1, 16)
	case CodeOpus:
		return opus.NewOpus(opus.PROFILE_VOICE_LOW)
	case CodeCodec2:
		return codec2.NewCodec2(codec2.MODE_700B)
	default:
		return nil, ErrUnknownCodecType
	}
}

// SignallingReceiver handles inband signalling over audio links.
type SignallingReceiver struct {
	mu                sync.Mutex
	outgoingSignals   []any
	proxy            *SignallingReceiver
	signallingHandler func(signals []any, source any)
	packetSender     func(destination any, data []byte) error
}

func NewSignallingReceiver(proxy *SignallingReceiver) *SignallingReceiver {
	return &SignallingReceiver{
		proxy: proxy,
	}
}

func (sr *SignallingReceiver) Signal(signal any, sendFunc func(data []byte) error, immediate bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	signallingData := map[byte]any{FieldSignalling: []any{signal}}
	packed, err := PackData(signallingData)
	if err != nil {
		return
	}

	if immediate && sendFunc != nil {
		_ = sendFunc(packed)
	} else {
		sr.outgoingSignals = append(sr.outgoingSignals, signal)
	}
}

func (sr *SignallingReceiver) SignallingReceived(signals []any, source any) {
	if sr.signallingHandler != nil {
		sr.signallingHandler(signals, source)
	}
	if sr.proxy != nil {
		sr.proxy.SignallingReceived(signals, source)
	}
}

func (sr *SignallingReceiver) HandlePacket(data []byte, source any) error {
	unpacked, err := UnpackData(data)
	if err != nil {
		return err
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		return nil
	}

	if signalling, exists := m[FieldSignalling]; exists {
		switch v := signalling.(type) {
		case []any:
			sr.SignallingReceived(v, source)
		default:
			sr.SignallingReceived([]any{v}, source)
		}
	}

	return nil
}

func (sr *SignallingReceiver) SetSignallingHandler(handler func(signals []any, source any)) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.signallingHandler = handler
}

// Packetizer sends encoded audio frames over RNS links.
type Packetizer struct {
	mu              sync.Mutex
	destination    any
	shouldRun       bool
	source          sources.Source
	transmitFailure bool
	failureCallback func()
	codec          codecs.Codec
	sendFunc       func(data []byte) error
}

func NewPacketizer(sendFunc func(data []byte) error, failureCallback func()) *Packetizer {
	return &Packetizer{
		sendFunc:       sendFunc,
		failureCallback: failureCallback,
	}
}

func (p *Packetizer) HandleFrame(frame []byte, source sources.Source) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sendFunc == nil {
		return nil
	}

	header, err := CodecHeaderByte(p.codec)
	if err != nil {
		header = CodeNull
	}

	frameData := append([]byte{header}, frame...)
	packetData := map[byte]any{FieldFrames: frameData}
	packed, err := PackData(packetData)
	if err != nil {
		return err
	}

	if err := p.sendFunc(packed); err != nil {
		p.transmitFailure = true
		if p.failureCallback != nil {
			p.failureCallback()
		}
		return err
	}

	return nil
}

func (p *Packetizer) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shouldRun = true
	return nil
}

func (p *Packetizer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shouldRun = false
	return nil
}

func (p *Packetizer) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shouldRun
}

func (p *Packetizer) CanReceive(fromSource sources.Source) bool { return true }

func (p *Packetizer) SetSource(src sources.Source) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.source = src
}

func (p *Packetizer) SetCodec(codec codecs.Codec) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.codec = codec
}

// LinkSource receives audio frames over RNS links.
type LinkSource struct {
	mu                 sync.Mutex
	shouldRun          bool
	sink              sources.LocalSource
	codec            codecs.Codec
	receiveLock       sync.Mutex
	signallingReceiver *SignallingReceiver
	channels          int
}

func NewLinkSource(signallingReceiver *SignallingReceiver, sink sources.LocalSource) *LinkSource {
	return &LinkSource{
		sink:               sink,
		codec:              codecs.NullCodec{},
		signallingReceiver: signallingReceiver,
	}
}

func (ls *LinkSource) Start() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.shouldRun = true
	return nil
}

func (ls *LinkSource) Stop() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.shouldRun = false
	return nil
}

func (ls *LinkSource) Running() bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.shouldRun
}

func (ls *LinkSource) CanReceive(fromSource sources.Source) bool { return true }

func (ls *LinkSource) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	return nil
}

func (ls *LinkSource) SetCodec(codec codecs.Codec) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.codec = codec
}

func (ls *LinkSource) GetCodec() codecs.Codec {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.codec
}

func (ls *LinkSource) ReceivePacket(data []byte) {
	ls.receiveLock.Lock()
	defer ls.receiveLock.Unlock()

	unpacked, err := UnpackData(data)
	if err != nil {
		return
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		return
	}

	if framesData, exists := m[FieldFrames]; exists {
		frameData, ok := framesData.([]byte)
		if !ok || len(frameData) < 1 {
			return
		}

		headerByte := frameData[0]
		payload := frameData[1:]

		newCodec, err := CodecTypeFromHeader(headerByte)
		if err != nil {
			return
		}

		ls.codec = newCodec

		if newCodec != nil && ls.sink != nil && ls.sink.CanReceive(ls) {
			decoded := ls.codec.Decode(payload, ls.channels)
			ls.sink.HandleFrame(decoded, ls)
		}
	}

	if _, exists := m[FieldSignalling]; exists {
		if ls.signallingReceiver != nil {
			ls.signallingReceiver.HandlePacket(data, nil)
		}
	}
}

func (ls *LinkSource) Channels() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.channels
}