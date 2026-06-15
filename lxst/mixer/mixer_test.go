// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package mixer

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func TestMixer_New(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)
	if m == nil {
		t.Fatal("NewMixer returned nil")
	}
	if m.TargetFrameMs() != 40.0 {
		t.Errorf("Expected target frame ms 40.0, got %f", m.TargetFrameMs())
	}
}

func TestMixer_DefaultFrameTime(t *testing.T) {
	t.Parallel()

	m := NewMixer(0, 0, nil, nil, 0.0)
	if m.TargetFrameMs() != 40.0 {
		t.Errorf("Expected default frame ms 40.0, got %f", m.TargetFrameMs())
	}
}

func TestMixer_MuteUnmute(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)

	if m.IsMuted() {
		t.Error("Mixer should not be muted initially")
	}

	m.Mute()
	if !m.IsMuted() {
		t.Error("Mixer should be muted after Mute()")
	}
	if m.mixingGain() != 0.0 {
		t.Errorf("Muted gain should be 0.0, got %f", m.mixingGain())
	}

	m.Unmute()
	if m.IsMuted() {
		t.Error("Mixer should not be muted after Unmute()")
	}
}

func TestMixer_Gain(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)

	if m.Gain() != 0.0 {
		t.Errorf("Expected initial gain 0.0, got %f", m.Gain())
	}

	if g := m.mixingGain(); g != 1.0 {
		t.Errorf("Zero dB gain should be 1.0, got %f", g)
	}

	m.SetGain(10.0)
	if m.Gain() != 10.0 {
		t.Errorf("Expected gain 10.0, got %f", m.Gain())
	}

	expectedGain := math.Pow(10, 1.0)
	if g := m.mixingGain(); math.Abs(float64(g)-expectedGain) > 0.001 {
		t.Errorf("10dB gain should be %f, got %f", expectedGain, g)
	}
}

func TestMixer_MutedGain(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 10.0)
	m.Mute()

	if m.mixingGain() != 0.0 {
		t.Errorf("Muted gain should be 0.0, got %f", m.mixingGain())
	}
}

func TestMixer_CanReceive(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)

	if !m.CanReceive(nil) {
		t.Error("Mixer should be able to receive from unknown source")
	}
}

func TestMixer_HandleFrame(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)

	frame := [][]float32{{0.5, -0.3}, {-0.7, 0.9}}
	src := &mockMixerSource{sampleRate: 48000, channels: 2}

	err := m.HandleFrame(frame, src)
	if err != nil {
		t.Errorf("HandleFrame should not error: %v", err)
	}
}

func TestMixer_SampleRate(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)
	if m.SampleRate() != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", m.SampleRate())
	}
}

func TestMixer_CodecSetterConstraints(t *testing.T) {
	t.Parallel()

	opus, err := opusPkg.NewOpus(opusPkg.PROFILE_VOICE_LOW)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	m := NewMixer(21.0, 48000, opus, nil, 0.0)

	if math.Abs(m.TargetFrameMs()-20.0) > 0.001 {
		t.Errorf("Expected frame time 20.0ms (closest valid), got %f", m.TargetFrameMs())
	}
}

func TestMixer_SetSourceMaxFrames(t *testing.T) {
	t.Parallel()

	m := NewMixer(40.0, 48000, nil, nil, 0.0)
	src := &mockMixerSource{sampleRate: 48000, channels: 2}

	m.SetSourceMaxFrames(src, 4)

	if !m.CanReceive(src) {
		t.Error("Should be able to receive from source")
	}
}

type mockMixerSource struct {
	sampleRate int
	channels   int
}

func (m *mockMixerSource) Start() error                                                   { return nil }
func (m *mockMixerSource) Stop() error                                                    { return nil }
func (m *mockMixerSource) Running() bool                                                  { return true }
func (m *mockMixerSource) SampleRate() int                                                { return m.sampleRate }
func (m *mockMixerSource) Channels() int                                                  { return m.channels }
func (m *mockMixerSource) CanReceive(fromSource sources.Source) bool                      { return true }
func (m *mockMixerSource) HandleFrame(frame [][]float32, fromSource sources.Source) error { return nil }
func (m *mockMixerSource) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	return nil
}

func TestMixer_SetSink(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	m := NewMixer(40.0, 48000, nil, sink, 1.0)

	if m.Sink() != sink {
		t.Error("Sink should return the sink passed to NewMixer")
	}

	newSink := &captureSink{}
	m.SetSink(newSink)
	if m.Sink() != newSink {
		t.Error("Sink should return the new sink after SetSink")
	}
}

type captureSink struct {
	mu           sync.Mutex
	lastFrame    [][]float32
	lastEncoded  []byte
	gotEncoded   bool
	gotUnencoded bool
}

func (c *captureSink) Start() error                              { return nil }
func (c *captureSink) Stop() error                               { return nil }
func (c *captureSink) Running() bool                             { return true }
func (c *captureSink) CanReceive(fromSource sources.Source) bool { return true }
func (c *captureSink) HandleFrame(frame [][]float32, fromSource sources.Source) error {
	c.mu.Lock()
	c.gotUnencoded = true
	c.lastFrame = frame
	c.mu.Unlock()
	return nil
}
func (c *captureSink) HandleEncodedFrame(data []byte, fromSource sources.Source) error {
	c.mu.Lock()
	c.gotEncoded = true
	c.lastEncoded = data
	c.mu.Unlock()
	return nil
}
func (c *captureSink) IsGotEncoded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gotEncoded
}
func (c *captureSink) IsGotUnencoded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gotUnencoded
}
func (c *captureSink) LastEncoded() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastEncoded
}

func TestMixer_SendsEncodedBytesWhenCodecPresent(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	sink := &captureSink{}
	m := NewMixer(40.0, 48000, codec, sink, 1.0)
	m.SetSink(sink)

	src := &mockMixerSource{sampleRate: 48000, channels: 1}

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	frame := [][]float32{{0.5, -0.3, 0.1, -0.2, 0.4, -0.1, 0.3, -0.4}}
	_ = m.HandleFrame(frame, src)

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("Timed out waiting for mixer to process frame")
		case <-ticker.C:
			if sink.IsGotEncoded() || sink.IsGotUnencoded() {
				goto done
			}
		}
	}
done:

	if !sink.IsGotEncoded() {
		t.Error("Expected sink to receive encoded bytes via HandleEncodedFrame")
	}
	if sink.IsGotUnencoded() {
		t.Error("Expected sink NOT to receive unencoded frames via HandleFrame")
	}
	if len(sink.LastEncoded()) == 0 {
		t.Error("Expected non-empty encoded data")
	}
}

func TestMixer_SendsUnencodedFramesWhenNoCodec(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	m := NewMixer(40.0, 48000, nil, sink, 1.0)

	src := &mockMixerSource{sampleRate: 48000, channels: 1}

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	frame := [][]float32{{0.5, -0.3, 0.1, -0.2, 0.4, -0.1, 0.3, -0.4}}
	_ = m.HandleFrame(frame, src)

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("Timed out waiting for mixer to process frame")
		case <-ticker.C:
			if sink.IsGotUnencoded() || sink.IsGotEncoded() {
				goto done2
			}
		}
	}
done2:

	if sink.IsGotEncoded() {
		t.Error("Expected sink NOT to receive encoded bytes when no codec")
	}
	if !sink.IsGotUnencoded() {
		t.Error("Expected sink to receive unencoded frames via HandleFrame")
	}
}

func TestMixer_SendsUnencodedFramesWithNullCodec(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	m := NewMixer(40.0, 48000, codecs.NullCodec{}, sink, 1.0)

	src := &mockMixerSource{sampleRate: 48000, channels: 1}

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	frame := [][]float32{{0.5, -0.3, 0.1, -0.2, 0.4, -0.1, 0.3, -0.4}}
	_ = m.HandleFrame(frame, src)

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("Timed out waiting for mixer to process frame")
		case <-ticker.C:
			if sink.IsGotUnencoded() || sink.IsGotEncoded() {
				goto done3
			}
		}
	}
done3:

	if sink.IsGotEncoded() {
		t.Error("Expected sink NOT to receive encoded bytes with NullCodec")
	}
	if !sink.IsGotUnencoded() {
		t.Error("Expected sink to receive unencoded frames via HandleFrame with NullCodec")
	}
}
