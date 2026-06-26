// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package network

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func TestCodecHeaderByte_Raw(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeRaw {
		t.Errorf("Expected CodeRaw (0x%02x), got 0x%02x", CodeRaw, b)
	}
}

func TestCodecHeaderByte_Opus(t *testing.T) {
	t.Parallel()

	codec, err := opus.NewOpus(opus.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeOpus {
		t.Errorf("Expected CodeOpus (0x%02x), got 0x%02x", CodeOpus, b)
	}
}

func TestCodecHeaderByte_Codec2(t *testing.T) {
	t.Parallel()

	codec, err := codec2.NewCodec2(codec2.MODE_700B)
	if err != nil {
		t.Skipf("Codec2 not available: %v", err)
	}
	b, err := CodecHeaderByte(codec)
	if err != nil {
		t.Fatalf("CodecHeaderByte failed: %v", err)
	}
	if b != CodeCodec2 {
		t.Errorf("Expected CodeCodec2 (0x%02x), got 0x%02x", CodeCodec2, b)
	}
}

func TestCodecHeaderByte_Unknown(t *testing.T) {
	t.Parallel()

	_, err := CodecHeaderByte(nil)
	if err == nil {
		t.Error("Expected error for nil codec")
	}
}

func TestCodecTypeFromHeader_Raw(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeRaw)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*raw.Raw); !ok {
		t.Error("Expected Raw codec")
	}
}

func TestCodecTypeFromHeader_Opus(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeOpus)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*opus.Opus); !ok {
		t.Error("Expected Opus codec")
	}
}

func TestCodecTypeFromHeader_Codec2(t *testing.T) {
	t.Parallel()

	codec, err := CodecTypeFromHeader(CodeCodec2)
	if err != nil {
		t.Fatalf("CodecTypeFromHeader failed: %v", err)
	}
	if _, ok := codec.(*codec2.Codec2); !ok {
		t.Error("Expected Codec2 codec")
	}
}

func TestCodecTypeFromHeader_Unknown(t *testing.T) {
	t.Parallel()

	_, err := CodecTypeFromHeader(0xFE)
	if err == nil {
		t.Error("Expected error for unknown header byte")
	}
}

func TestCodecHeaderByte_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		codec func() (codecs.Codec, error)
		code  byte
	}{
		{"Raw", func() (codecs.Codec, error) { return raw.NewRaw(1, 16) }, CodeRaw},
		{"Opus", func() (codecs.Codec, error) { return opus.NewOpus(opus.PROFILE_VOICE_LOW) }, CodeOpus},
		{"Codec2", func() (codecs.Codec, error) { return codec2.NewCodec2(codec2.MODE_700B) }, CodeCodec2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codec, err := tc.codec()
			if err != nil {
				t.Skipf("Codec not available: %v", err)
			}

			b, err := CodecHeaderByte(codec)
			if err != nil {
				t.Fatalf("CodecHeaderByte failed: %v", err)
			}
			if b != tc.code {
				t.Errorf("Expected 0x%02x, got 0x%02x", tc.code, b)
			}

			recovered, err := CodecTypeFromHeader(b)
			if err != nil {
				t.Fatalf("CodecTypeFromHeader failed: %v", err)
			}

			b2, err := CodecHeaderByte(recovered)
			if err != nil {
				t.Fatalf("CodecHeaderByte on recovered codec failed: %v", err)
			}
			if b2 != b {
				t.Errorf("Roundtrip failed: expected 0x%02x, got 0x%02x", b, b2)
			}
		})
	}
}

func TestPackData_Signalling(t *testing.T) {
	t.Parallel()

	data := map[byte]any{
		FieldSignalling: []any{"ring"},
	}
	packed, err := PackData(data)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}
	if len(packed) == 0 {
		t.Error("Packed data should not be empty")
	}

	unpacked, err := UnpackData(packed)
	if err != nil {
		t.Fatalf("UnpackData failed: %v", err)
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		t.Fatal("Expected map[byte]any")
	}

	if _, exists := m[FieldSignalling]; !exists {
		t.Error("Expected FieldSignalling in unpacked data")
	}
}

func TestPackData_Frames(t *testing.T) {
	t.Parallel()

	frameData := []byte{CodeOpus, 0x01, 0x02, 0x03}
	data := map[byte]any{
		FieldFrames: frameData,
	}
	packed, err := PackData(data)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}

	unpacked, err := UnpackData(packed)
	if err != nil {
		t.Fatalf("UnpackData failed: %v", err)
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		t.Fatal("Expected map[byte]any")
	}

	if _, exists := m[FieldFrames]; !exists {
		t.Error("Expected FieldFrames in unpacked data")
	}
}

func TestPackData_Empty(t *testing.T) {
	t.Parallel()

	data := map[byte]any{}
	packed, err := PackData(data)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}
	if len(packed) != 0 {
		t.Error("Empty map should pack to nil")
	}
}

func TestPackData_BothFields(t *testing.T) {
	t.Parallel()

	data := map[byte]any{
		FieldSignalling: []any{"busy"},
		FieldFrames:     []byte{0x00, 0x01},
	}
	packed, err := PackData(data)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}

	unpacked, err := UnpackData(packed)
	if err != nil {
		t.Fatalf("UnpackData failed: %v", err)
	}

	m, ok := unpacked.(map[byte]any)
	if !ok {
		t.Fatal("Expected map[byte]any")
	}

	if len(m) != 2 {
		t.Errorf("Expected 2 fields, got %v", len(m))
	}
}

func TestSignallingReceiver_New(t *testing.T) {
	t.Parallel()

	sr := NewSignallingReceiver(nil)
	if sr == nil {
		t.Fatal("NewSignallingReceiver returned nil")
	}
}

func TestSignallingReceiver_HandlePacket(t *testing.T) {
	t.Parallel()

	var receivedSignals []any
	sr := NewSignallingReceiver(nil)
	sr.SetSignallingHandler(func(signals []any, source any) {
		receivedSignals = signals
	})

	data := map[byte]any{
		FieldSignalling: []any{"ring"},
	}
	packed, _ := PackData(data)
	err := sr.HandlePacket(packed, nil)
	if err != nil {
		t.Fatalf("HandlePacket failed: %v", err)
	}

	if len(receivedSignals) != 1 {
		t.Errorf("Expected 1 signal, got %v", len(receivedSignals))
	}
}

func TestSignallingReceiver_Proxy(t *testing.T) {
	t.Parallel()

	var proxiedSignals []any
	proxy := NewSignallingReceiver(nil)
	proxy.SetSignallingHandler(func(signals []any, source any) {
		proxiedSignals = signals
	})

	sr := NewSignallingReceiver(proxy)
	sr.SignallingReceived([]any{"test"}, nil)

	if len(proxiedSignals) != 1 {
		t.Errorf("Expected 1 proxied signal, got %v", len(proxiedSignals))
	}
}

func TestPacketizer_New(t *testing.T) {
	t.Parallel()

	p := NewPacketizer(nil, nil)
	if p == nil {
		t.Fatal("NewPacketizer returned nil")
	}
}

func TestPacketizer_StartStop(t *testing.T) {
	t.Parallel()

	p := NewPacketizer(nil, nil)

	err := p.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !p.Running() {
		t.Error("Packetizer should be running after Start()")
	}

	err = p.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.Running() {
		t.Error("Packetizer should not be running after Stop()")
	}
}

func TestPacketizer_HandleFrame(t *testing.T) {
	t.Parallel()

	var sentData []byte
	p := NewPacketizer(func(data []byte) error {
		sentData = data
		return nil
	}, nil)

	opus, err := opus.NewOpus(opus.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	p.SetCodec(opus)

	// Create a frame with enough samples for Opus encoding (160 samples at 8kHz)
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.1}
	}
	err = p.HandleFrame(frame, nil)
	if err != nil {
		t.Fatalf("HandleFrame failed: %v", err)
	}

	if len(sentData) == 0 {
		t.Error("Expected data to be sent")
	}
}

func TestPacketizer_TransmitFailure(t *testing.T) {
	t.Parallel()

	failureCalled := false
	p := NewPacketizer(func(data []byte) error {
		return fmt.Errorf("transmit error")
	}, func() {
		failureCalled = true
	})

	opus, err := opus.NewOpus(opus.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	p.SetCodec(opus)

	// Create a frame with enough samples for Opus encoding
	frame := make([][]float32, 160)
	for i := range frame {
		frame[i] = []float32{0.1}
	}
	_ = p.HandleFrame(frame, nil)

	if !failureCalled {
		t.Error("Failure callback should have been called")
	}
}

func TestLinkSource_New(t *testing.T) {
	t.Parallel()

	ls := NewLinkSource(nil, nil)
	if ls == nil {
		t.Fatal("NewLinkSource returned nil")
	}
}

func TestLinkSource_StartStop(t *testing.T) {
	t.Parallel()

	ls := NewLinkSource(nil, nil)

	err := ls.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !ls.Running() {
		t.Error("LinkSource should be running after Start()")
	}

	err = ls.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if ls.Running() {
		t.Error("LinkSource should not be running after Stop()")
	}
}

func TestLinkSource_SetCodec(t *testing.T) {
	t.Parallel()

	ls := NewLinkSource(nil, nil)

	codec := codecs.NullCodec{}
	ls.SetCodec(codec)

	if ls.GetCodec() != codec {
		t.Error("Codec should match")
	}
}

func TestLinkSource_ReceivePacket_Frames(t *testing.T) {
	t.Parallel()

	var receivedFrames [][]float32
	mockSink := &mockNetworkSink{
		handleFrameFunc: func(frame [][]float32, fromSource sources.Source) error {
			receivedFrames = frame
			return nil
		},
		canReceiveVal: true,
	}

	ls := NewLinkSource(nil, mockSink)

	frameData := []byte{CodeRaw, 0x01, 0x02}
	data := map[byte]any{
		FieldFrames: frameData,
	}
	packed, _ := PackData(data)

	ls.ReceivePacket(packed)

	_ = receivedFrames
}

func TestUnpackData_InvalidInput(t *testing.T) {
	t.Parallel()

	_, err := UnpackData([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}

	_, err = UnpackData([]byte{0xff})
	if err == nil {
		t.Error("Expected error for invalid data")
	}
}

type mockNetworkSink struct {
	handleFrameFunc func(frame [][]float32, fromSource sources.Source) error
	canReceiveVal   bool
}

func (m *mockNetworkSink) Start() error  { return nil }
func (m *mockNetworkSink) Stop() error   { return nil }
func (m *mockNetworkSink) Running() bool { return true }
func (m *mockNetworkSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	if m.handleFrameFunc != nil {
		return m.handleFrameFunc(frame, fromSource)
	}
	return nil
}
func (m *mockNetworkSink) CanReceive(fromSource sources.Source) bool { return m.canReceiveVal }
func (m *mockNetworkSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

func TestPipelineTransmitReceiveRoundtrip(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	var receivedPackets [][]byte
	var pktMu sync.Mutex

	sendFunc := func(data []byte) error {
		pktMu.Lock()
		receivedPackets = append(receivedPackets, data)
		pktMu.Unlock()
		return nil
	}

	pktz := NewPacketizer(sendFunc, nil)
	pktz.SetCodec(codec)

	frame := [][]float32{{0.5, -0.3, 0.1, -0.2}}
	encoded := codec.Encode(frame)

	err = pktz.HandleEncodedFrame(encoded, nil)
	if err != nil {
		t.Fatalf("HandleEncodedFrame failed: %v", err)
	}

	pktMu.Lock()
	pkts := receivedPackets
	pktMu.Unlock()

	if len(pkts) != 1 {
		t.Fatalf("Expected 1 packet sent, got %v", len(pkts))
	}

	sink := &frameCaptureSink{}
	ls := NewLinkSource(nil, sink)
	ls.SetChannels(1)

	ls.ReceivePacket(pkts[0])

	if !sink.gotFrame {
		t.Error("LinkSource should have delivered decoded frame to sink")
	}
	if len(sink.lastFrame) == 0 {
		t.Error("Decoded frame should not be empty")
	}
}

type frameCaptureSink struct {
	mu        sync.Mutex
	gotFrame  bool
	lastFrame [][]float32
}

func (f *frameCaptureSink) Start() error                              { return nil }
func (f *frameCaptureSink) Stop() error                               { return nil }
func (f *frameCaptureSink) Running() bool                             { return true }
func (f *frameCaptureSink) CanReceive(fromSource sources.Source) bool { return true }
func (f *frameCaptureSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	f.mu.Lock()
	f.gotFrame = true
	f.lastFrame = frame
	f.mu.Unlock()
	return nil
}
func (f *frameCaptureSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

func TestFullCallPipelineRoundtrip(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	var receivedPackets [][]byte
	var pktMu sync.Mutex

	sendFunc := func(data []byte) error {
		pktMu.Lock()
		receivedPackets = append(receivedPackets, data)
		pktMu.Unlock()
		return nil
	}

	pktz := NewPacketizer(sendFunc, nil)
	pktz.SetCodec(codec)

	receiveSink := &frameCaptureSink{}
	linkSrc := NewLinkSource(nil, receiveSink)
	linkSrc.SetChannels(1)

	testFrame := [][]float32{{0.5, -0.3, 0.1, -0.2, 0.4, -0.1, 0.3, -0.4}}

	encoded := codec.Encode(testFrame)
	err = pktz.HandleEncodedFrame(encoded, nil)
	if err != nil {
		t.Fatalf("Packetizer.HandleEncodedFrame failed: %v", err)
	}

	pktMu.Lock()
	pkts := receivedPackets
	pktMu.Unlock()
	if len(pkts) != 1 {
		t.Fatalf("Expected 1 packet, got %v", len(pkts))
	}

	linkSrc.ReceivePacket(pkts[0])

	if !receiveSink.gotFrame {
		t.Fatal("LinkSource should have delivered decoded frame to receive sink")
	}

	if len(receiveSink.lastFrame) != len(testFrame) {
		t.Errorf("Decoded frame has %v samples, want %v", len(receiveSink.lastFrame), len(testFrame))
	}
}
