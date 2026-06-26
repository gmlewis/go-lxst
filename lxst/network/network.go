// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package network provides audio streaming over Reticulum networks.
// It implements the NetworkSource and NetworkSink types for sending
// and receiving audio frames over Reticulum links, with support for
// configurable codec selection, link management, and automatic
// reconnection handling.
package network

import (
	"log"
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

// isNullOrUnsetType reports whether a codec is nil or a NullCodec,
// indicating that no real codec has been configured.
func isNullOrUnsetType(c codecs.Codec) bool {
	if c == nil {
		return true
	}
	switch c.(type) {
	case codecs.NullCodec:
		return true
	}
	return false
}

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
	proxy             *SignallingReceiver
	signallingHandler func(signals []any, source any)
	packetSender      func(destination any, data []byte) error
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
		if err := sendFunc(packed); err != nil {
			log.Printf("SignallingReceiver.sendSignal: sendFunc failed: %v", err)
		}
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
	destination     any
	shouldRun       bool
	source          sources.Source
	transmitFailure bool
	failureCallback func()
	codec           codecs.Codec
	sendFunc        func(data []byte) error
}

func NewPacketizer(sendFunc func(data []byte) error, failureCallback func()) *Packetizer {
	return &Packetizer{
		sendFunc:        sendFunc,
		failureCallback: failureCallback,
	}
}

// Ensure Packetizer implements sources.LocalSource
var _ sources.LocalSource = (*Packetizer)(nil)

func (p *Packetizer) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	p.mu.Lock()
	codec := p.codec
	p.mu.Unlock()

	if codec == nil {
		log.Printf("Packetizer.HandleFrame: codec is nil, dropping frame (len=%d)", len(frame))
		return nil
	}

	encoded := codec.Encode(frame)
	if len(encoded) == 0 {
		return nil
	}

	return p.HandleEncodedFrame(encoded, fromSource)
}

// HandleEncodedFrame receives already-encoded audio data from the
// transmit Mixer, prepends a codec header byte, and sends it over
// the RNS link. This matches the Python Packetizer.handle_frame
// which receives codec.encode(mixed_frame) from the Mixer.
func (p *Packetizer) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sendFunc == nil {
		log.Printf("Packetizer.HandleEncodedFrame: sendFunc is nil, dropping frame")
		return nil
	}

	if len(data) == 0 {
		return nil
	}

	// Stop sending after a transmit failure to prevent repeated
	// send-on-closed-link errors. The failure callback will trigger
	// call termination, which will stop the pipeline.
	if p.transmitFailure {
		return nil
	}

	log.Printf("Packetizer.HandleEncodedFrame: called (dataLen=%d, sendFunc=%v, transmitFailure=%v, codec=%T)",
		len(data), p.sendFunc != nil, p.transmitFailure, p.codec)

	var header byte
	if p.codec != nil {
		var err error
		header, err = CodecHeaderByte(p.codec)
		if err != nil {
			header = CodeNull
		}
	} else {
		header = CodeNull
	}

	frameData := append([]byte{header}, data...)
	packetData := map[byte]any{FieldFrames: frameData}
	packed, err := PackData(packetData)
	if err != nil {
		return err
	}

	if err := p.sendFunc(packed); err != nil {
		log.Printf("Packetizer.HandleEncodedFrame: sendFunc failed: %v", err)
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
	log.Printf("Packetizer.Start: shouldRun=true, sendFunc=%v, codec=%v", p.sendFunc != nil, p.codec != nil)
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
	log.Printf("Packetizer.SetCodec: codec=%T", codec)
}

// LinkSource receives audio frames over RNS links.
type LinkSource struct {
	mu                 sync.Mutex
	shouldRun          bool
	sink               sources.LocalSource
	codec              codecs.Codec
	receiveLock        sync.Mutex
	signallingReceiver *SignallingReceiver
	channels           int
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

// HandleEncodedFrame is not used by LinkSource in normal pipeline
// operation. LinkSource is the entry point for received audio and
// sends decoded float32 frames to its sink.
func (ls *LinkSource) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

// Sink returns the current output destination for this LinkSource.
func (ls *LinkSource) Sink() sources.LocalSource {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.sink
}

// SetSink sets the output destination for this LinkSource, matching
// the Python LocalSource.sink property setter.
func (ls *LinkSource) SetSink(sink sources.LocalSource) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.sink = sink
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
		log.Printf("LinkSource.ReceivePacket: UnpackData failed: %v", err)
		return
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		log.Printf("LinkSource.ReceivePacket: unpacked data is not map[byte]any, type=%T", unpacked)
		return
	}

	if framesData, exists := m[FieldFrames]; exists {
		frameData, ok := framesData.([]byte)
		if !ok || len(frameData) < 1 {
			log.Printf("LinkSource.ReceivePacket: FieldFrames present but invalid: type=%T len=%d", framesData, len(frameData))
			return
		}

		headerByte := frameData[0]
		payload := frameData[1:]

		// Use the pre-configured codec if available (matches the
		// negotiated profile from signalling). Fall back to creating
		// a new codec from the header byte only when no codec is set.
		ls.mu.Lock()
		activeCodec := ls.codec
		sink := ls.sink
		channels := ls.channels
		ls.mu.Unlock()

		if isNullOrUnsetType(activeCodec) {
			newCodec, err := CodecTypeFromHeader(headerByte)
			if err != nil {
				log.Printf("LinkSource.ReceivePacket: unknown codec header 0x%02x", headerByte)
				return
			}
			activeCodec = newCodec
			ls.mu.Lock()
			ls.codec = newCodec
			ls.mu.Unlock()
		}

		if sink == nil {
			log.Printf("LinkSource.ReceivePacket: sink is nil, dropping audio frame (codec=%T, payloadLen=%d)", activeCodec, len(payload))
			return
		}
		if !sink.CanReceive(ls) {
			log.Printf("LinkSource.ReceivePacket: sink cannot receive, dropping audio frame (codec=%T)", activeCodec)
			return
		}

		decoded := activeCodec.Decode(payload, channels)
		if len(decoded) == 0 {
			log.Printf("LinkSource.ReceivePacket: decode returned empty frame (codec=%T, payloadLen=%d, channels=%d)", activeCodec, len(payload), channels)
		}
		if err := sink.HandleFrame(decoded, ls); err != nil {
			log.Printf("LinkSource.ReceivePacket: sink.HandleFrame failed: %v", err)
		}
	} else {
		log.Printf("LinkSource.ReceivePacket: no FieldFrames in packet, signalling-only (fields=%v)", func() []byte {
			keys := make([]byte, 0)
			for k := range m {
				keys = append(keys, k)
			}
			return keys
		}())
	}

	if _, exists := m[FieldSignalling]; exists {
		if ls.signallingReceiver != nil {
			if err := ls.signallingReceiver.HandlePacket(data, nil); err != nil {
				log.Printf("LinkSource.ReceivePacket: signallingReceiver.HandlePacket failed: %v", err)
			}
		}
	}
}

func (ls *LinkSource) Channels() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.channels
}

// SetChannels sets the channel count for this LinkSource, used
// for decoding incoming audio frames.
func (ls *LinkSource) SetChannels(channels int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.channels = channels
}
