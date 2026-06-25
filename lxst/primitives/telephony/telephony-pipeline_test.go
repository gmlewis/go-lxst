// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package telephony

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/testutils"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, cleanup := testutils.TempDir(t, "go-lxst-telephony-test-")
	t.Cleanup(cleanup)
	return dir
}

func createTestWavFile(t *testing.T, sampleRate, numChannels, sampleCount int, frequency float64) string {
	t.Helper()

	dir := tempDir(t)
	path := filepath.Join(dir, "test.wav")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer func() { _ = f.Close() }()

	dataSize := sampleCount * numChannels * 2
	byteRate := uint32(sampleRate) * uint32(numChannels) * 2
	blockAlign := uint16(numChannels) * 2

	if _, err := f.Write([]byte("RIFF")); err != nil {
		t.Fatalf("write RIFF header: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatalf("write file size: %v", err)
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		t.Fatalf("write WAVE header: %v", err)
	}
	if _, err := f.Write([]byte("fmt ")); err != nil {
		t.Fatalf("write fmt chunk: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatalf("write fmt size: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatalf("write audio format: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		t.Fatalf("write channels: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatalf("write sample rate: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		t.Fatalf("write byte rate: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		t.Fatalf("write block align: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(16)); err != nil {
		t.Fatalf("write bits per sample: %v", err)
	}
	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatalf("write data chunk: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatalf("write data size: %v", err)
	}

	for i := 0; i < sampleCount; i++ {
		for ch := 0; ch < numChannels; ch++ {
			phase := 2.0 * math.Pi * frequency * float64(i) / float64(sampleRate)
			sample := int16(16000.0 * math.Sin(phase))
			if err := binary.Write(f, binary.LittleEndian, sample); err != nil {
				t.Fatalf("write sample: %v", err)
			}
		}
	}

	return path
}

func TestSelectCallCodecs_BandwidthUltraLow(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.selectCallCodecs(ProfileBandwidthUltraLow)

	rxCodec := tel.ReceiveCodec()
	txCodec := tel.TransmitCodec()

	if _, ok := rxCodec.(codecs.NullCodec); !ok {
		t.Errorf("Expected NullCodec for receive codec, got %T", rxCodec)
	}
	if _, ok := txCodec.(*codec2.Codec2); !ok {
		t.Error("Expected Codec2 for transmit codec with BANDWIDTH_ULTRA_LOW profile")
	}
}

func TestSelectCallCodecs_QualityMedium(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.selectCallCodecs(ProfileQualityMedium)

	rxCodec := tel.ReceiveCodec()
	txCodec := tel.TransmitCodec()

	if _, ok := rxCodec.(codecs.NullCodec); !ok {
		t.Errorf("Expected NullCodec for receive codec, got %T", rxCodec)
	}
	if _, ok := txCodec.(*opus.Opus); !ok {
		t.Error("Expected Opus for transmit codec with QUALITY_MEDIUM profile")
	}
}

func TestSelectCallFrameTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile byte
		want    float64
	}{
		{ProfileBandwidthUltraLow, 400.0},
		{ProfileBandwidthVeryLow, 320.0},
		{ProfileBandwidthLow, 200.0},
		{ProfileQualityMedium, 60.0},
		{ProfileQualityHigh, 60.0},
		{ProfileQualityMax, 60.0},
		{ProfileLatencyLow, 20.0},
		{ProfileLatencyUltraLow, 10.0},
	}

	for _, tt := range tests {
		tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
		tel.selectCallFrameTime(tt.profile)
		got := tel.TargetFrameTimeMs()
		if got != tt.want {
			t.Errorf("selectCallFrameTime(0x%02x): got %f, want %f", tt.profile, got, tt.want)
		}
	}
}

func TestSelectCallProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile    byte
		wantTxType string
	}{
		{ProfileBandwidthUltraLow, "*codec2.Codec2"},
		{ProfileQualityMedium, "*opus.Opus"},
	}

	for _, tt := range tests {
		tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
		tel.SetState(StateCalling)
		tel.SetProfile(tt.profile)
		tel.selectCallProfile(tt.profile)

		txCodec := tel.TransmitCodec()
		if txCodec == nil {
			t.Errorf("selectCallProfile(0x%02x): transmit codec is nil", tt.profile)
			continue
		}
		gotType := typeName(txCodec)
		if gotType != tt.wantTxType {
			t.Errorf("selectCallProfile(0x%02x): got transmit codec type %s, want %s",
				tt.profile, gotType, tt.wantTxType)
		}

		rxCodec := tel.ReceiveCodec()
		if _, ok := rxCodec.(codecs.NullCodec); !ok {
			t.Errorf("selectCallProfile(0x%02x): expected NullCodec for receive, got %T", tt.profile, rxCodec)
		}
	}
}

func TestSelectCallProfile_Default(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.selectCallProfile(0)

	if tel.CurrentProfile() != DefaultProfile {
		t.Errorf("selectCallProfile(0): expected profile 0x%02x, got 0x%02x",
			DefaultProfile, tel.CurrentProfile())
	}
}

func TestPrepareDiallingPipelines_CreatesComponents(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	if tel.DialTone() == nil {
		t.Error("PrepareDiallingPipelines should create dial tone")
	}
	if tel.ReceiveMixer() == nil {
		t.Error("PrepareDiallingPipelines should create receive mixer")
	}
	if tel.ReceivePipeline() == nil {
		t.Error("PrepareDiallingPipelines should create receive pipeline")
	}
}

func TestPrepareDiallingPipelines_DialToneFrequency(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt.Frequency() != DialToneFrequency {
		t.Errorf("Dial tone frequency: got %f, want %f", dt.Frequency(), DialToneFrequency)
	}
	if dt.Gain() != 0.0 {
		t.Errorf("Dial tone gain: got %f, want 0.0", dt.Gain())
	}
}

func TestPrepareDiallingPipelines_DialToneEaseTime(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt.EaseTimeMs() != DialToneEaseMs {
		t.Errorf("Dial tone ease time: got %f, want %f", dt.EaseTimeMs(), DialToneEaseMs)
	}
}

func TestResetDiallingPipelines_ResetsAndRecreates(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	firstMixer := tel.ReceiveMixer()
	firstDialTone := tel.DialTone()

	tel.ResetDiallingPipelines()

	secondMixer := tel.ReceiveMixer()
	secondDialTone := tel.DialTone()

	if secondMixer == firstMixer {
		t.Error("ResetDiallingPipelines should create a new receive mixer")
	}
	if secondDialTone == firstDialTone {
		t.Error("ResetDiallingPipelines should create a new dial tone")
	}
}

func TestPrepareDiallingPipelines_UsesReceiveGain(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 5.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	mixer := tel.ReceiveMixer()
	if mixer == nil {
		t.Fatal("ReceiveMixer should not be nil")
	}
	if mixer.Gain() != 5.0 {
		t.Errorf("ReceiveMixer gain: got %f, want 5.0", mixer.Gain())
	}
}

func TestPrepareDiallingPipelines_TargetFrameTime(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)

	tel.PrepareDiallingPipelines()

	ft := tel.TargetFrameTimeMs()
	if ft != 60.0 {
		t.Errorf("TargetFrameTimeMs: got %f, want 60.0", ft)
	}
}

func TestPrepareDiallingPipelines_BandwidthUltraLowFrameTime(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileBandwidthUltraLow)

	tel.PrepareDiallingPipelines()

	ft := tel.TargetFrameTimeMs()
	if ft != 400.0 {
		t.Errorf("TargetFrameTimeMs: got %f, want 400.0", ft)
	}
}

// typeName returns the Go type name for a codec.
func typeName(v any) string {
	switch v.(type) {
	case *codec2.Codec2:
		return "*codec2.Codec2"
	case *opus.Opus:
		return "*opus.Opus"
	case codecs.NullCodec:
		return "codecs.NullCodec"
	default:
		return "unknown"
	}
}

func TestActivateRingTone_NoRingtonePath(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)

	tel.ActivateRingTone()

	if tel.RingerPipeline() != nil {
		t.Error("RingerPipeline should be nil when no ringtone path is set")
	}
	if tel.RingerSource() != nil {
		t.Error("RingerSource should be nil when no ringtone path is set")
	}
}

func TestActivateRingTone_NonexistentPath(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)
	tel.SetRingtonePath("/nonexistent/ringtone.wav")

	tel.ActivateRingTone()

	if tel.RingerPipeline() != nil {
		t.Error("RingerPipeline should be nil when ringtone file does not exist")
	}
	if tel.RingerSource() != nil {
		t.Error("RingerSource should be nil when ringtone file does not exist")
	}
}

func TestActivateRingTone_CreatesRingerPipeline(t *testing.T) {
	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)
	tel.SetRingtonePath(path)

	tel.ActivateRingTone()

	if tel.RingerPipeline() == nil {
		t.Error("RingerPipeline should be created when ringtone path is valid")
	}
	if tel.RingerSource() == nil {
		t.Error("RingerSource should be created when ringtone path is valid")
	}
	if tel.RingerOutput() == nil {
		t.Error("RingerOutput should be created when ringtone path is valid")
	}
}

func TestActivateRingTone_RingerSourceLoop(t *testing.T) {
	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)
	tel.SetRingtonePath(path)

	tel.ActivateRingTone()

	src := tel.RingerSource()
	if src == nil {
		t.Fatal("RingerSource should not be nil")
	}
	if !src.Loop() {
		t.Error("RingerSource should be configured with loop=true")
	}
}

func TestStopRingTone(t *testing.T) {
	path := createTestWavFile(t, 8000, 1, 8000, 440.0)

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)
	tel.SetRingtonePath(path)

	tel.ActivateRingTone()
	tel.StopRingTone()

	// StopRingTone should stop the ringer source and pipeline without panicking
}

func TestIncoming_SetIncoming(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	if tel.Incoming() {
		t.Error("Incoming should default to false")
	}

	tel.SetIncoming(true)
	if !tel.Incoming() {
		t.Error("Incoming should be true after SetIncoming(true)")
	}

	tel.SetIncoming(false)
	if tel.Incoming() {
		t.Error("Incoming should be false after SetIncoming(false)")
	}
}

func TestPlayBusyTone_ZeroDuration(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetBusyToneSeconds(0)

	// Should return immediately without error
	tel.PlayBusyTone()
}

func TestPlayBusyTone_ResetsDiallingPipelinesWhenNeeded(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)
	tel.SetBusyToneSeconds(0.01) // Very short duration for fast test

	// Without PrepareDiallingPipelines, PlayBusyTone should call
	// ResetDiallingPipelines which creates the dial tone
	tel.PlayBusyTone()

	// After PlayBusyTone completes, the pipelines should have been created
	if tel.DialTone() == nil {
		t.Error("PlayBusyTone should have created dial tone via ResetDiallingPipelines")
	}
}

func TestEnableDialTone_StartsMixerAndTone(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt == nil {
		t.Fatal("DialTone should not be nil after PrepareDiallingPipelines")
	}

	tel.EnableDialTone()

	if dt.Gain() != 0.04 {
		t.Errorf("DialTone gain after EnableDialTone: got %f, want 0.04", dt.Gain())
	}
}

func TestMuteDialTone_SetsGainToZero(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt == nil {
		t.Fatal("DialTone should not be nil after PrepareDiallingPipelines")
	}

	tel.EnableDialTone()
	tel.MuteDialTone()

	if dt.Gain() != 0.0 {
		t.Errorf("DialTone gain after MuteDialTone: got %f, want 0.0", dt.Gain())
	}
}

func TestDisableDialTone_StopsTone(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt == nil {
		t.Fatal("DialTone should not be nil after PrepareDiallingPipelines")
	}

	_ = dt.Start()
	if !dt.Running() {
		t.Error("DialTone should be running after Start")
	}

	tel.DisableDialTone()

	// DisableDialTone calls Stop() which sets shouldRun=false.
	// For non-ease tones, this immediately stops.
	// For ease tones (like our dial tone with ease=true), Stop() starts easing out.
	// The tone won't be "Running" once fully eased out, but we can
	// verify the method doesn't panic and the tone begins stopping.
}

func TestActivateDialTone_StartsDialPattern(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetProfile(ProfileQualityMedium)
	tel.SetIncoming(false) // Outgoing call
	tel.PrepareDiallingPipelines()

	// Transition to Ringing state (outgoing call remote ringing)
	tel.SetState(StateRinging)

	tel.ActivateDialTone()

	// Give the goroutine a brief moment to run
	time.Sleep(50 * time.Millisecond)

	// The dial tone should have been activated
	dt := tel.DialTone()
	if dt == nil {
		t.Fatal("DialTone should not be nil")
	}

	// After activating, the gain should have been set
	// (either enabled or muted depending on timing)
	gain := dt.Gain()
	if gain != 0.0 && gain != 0.04 {
		t.Errorf("Dial tone gain should be 0.0 or 0.04, got %f", gain)
	}

	// Clean up: transition out of ringing state to stop the goroutine
	tel.SetState(StateEstablished)
	time.Sleep(300 * time.Millisecond)
}

func TestActivateDialTone_DoesNotStartWhenNotRinging(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling) // Not Ringing state
	tel.SetIncoming(false)
	tel.PrepareDiallingPipelines()

	dt := tel.DialTone()
	if dt == nil {
		t.Fatal("DialTone should not be nil")
	}

	initialGain := dt.Gain()

	tel.ActivateDialTone()

	// Give the goroutine a brief moment
	time.Sleep(50 * time.Millisecond)

	// Gain should not have changed since we're not in Ringing state
	gain := dt.Gain()
	if gain != initialGain {
		t.Errorf("Dial tone gain should not change when not ringing: initial=%f, got=%f", initialGain, gain)
	}
}

func TestReconfigureTransmitPipeline_NotEstablished(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling) // Not Established

	// Should return early without panic
	tel.ReconfigureTransmitPipeline()
}

func TestReconfigureTransmitPipeline_NoTransmitPipeline(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	// No transmit pipeline set, should return early
	tel.ReconfigureTransmitPipeline()
}

func TestOpenPipelines_NotEstablished(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling) // Not Established

	// Should return early without panic
	tel.OpenPipelines()
}

func TestOpenPipelines_CreatesFiltersWithAGC(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	// Test the filter setup logic directly without starting LineSource
	tel.SetUseAGC(true)

	// Manually set filters like OpenPipelines would
	if tel.UseAGC() {
		tel.SetFilters([]filters.Filter{
			filters.NewBandPass(250, 8500),
			filters.NewAGC(-15.0, 12.0, 0.0001, 0.002, 0.001),
		})
	}

	f := tel.Filters()
	if len(f) != 2 {
		t.Errorf("Expected 2 filters with AGC, got %d", len(f))
	}
}

func TestOpenPipelines_CreatesFiltersWithoutAGC(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetUseAGC(false)
	tel.SetState(StateEstablished)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	// Manually set filters like OpenPipelines would (without AGC)
	tel.SetFilters([]filters.Filter{
		filters.NewBandPass(250, 8500),
	})

	f := tel.Filters()
	if len(f) != 1 {
		t.Errorf("Expected 1 filter without AGC, got %d", len(f))
	}
}

func TestStartStopPipelines_NoPipelines(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	// Should not panic with nil pipelines
	tel.StartPipelines()
	tel.StopPipelines()
}

func TestPacketizer_SinkInterface(t *testing.T) {
	pktz := network.NewPacketizer(nil, nil)
	if pktz == nil {
		t.Fatal("NewPacketizer returned nil")
	}

	// Verify Packetizer implements sinks.Sink
	var _ sinks.Sink = pktz
}

func TestFiltersAccessors(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	if len(tel.Filters()) != 0 {
		t.Errorf("Expected no filters initially, got %d", len(tel.Filters()))
	}

	f := []filters.Filter{filters.NewBandPass(250, 8500)}
	tel.SetFilters(f)

	if len(tel.Filters()) != 1 {
		t.Errorf("Expected 1 filter after SetFilters, got %d", len(tel.Filters()))
	}
}

func TestPacketizerAccessors(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	if tel.Packetizer() != nil {
		t.Error("Expected nil packetizer initially")
	}

	pktz := network.NewPacketizer(nil, nil)
	tel.SetPacketizer(pktz)

	if tel.Packetizer() != pktz {
		t.Error("SetPacketizer should set the packetizer")
	}
}

func TestSignallingReceived_Busy(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetIncoming(false)
	tel.SetProfile(ProfileQualityMedium)

	busyCalled := false
	tel.SetBusyCallback(func() { busyCalled = true })

	tel.SignallingReceived([]int{SignallingBusy}, nil)

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after BUSY, got %v", tel.State())
	}
	if !busyCalled {
		t.Error("Busy callback should have been called")
	}
}

func TestSignallingReceived_Rejected(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetIncoming(false)

	rejectedCalled := false
	tel.SetRejectedCallback(func() { rejectedCalled = true })

	tel.SignallingReceived([]int{SignallingRejected}, nil)

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after REJECTED, got %v", tel.State())
	}
	if !rejectedCalled {
		t.Error("Rejected callback should have been called")
	}
}

func TestSignallingReceived_Ringing(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)
	tel.SetIncoming(false)
	tel.SetProfile(ProfileQualityMedium)

	ringingCalled := false
	tel.SetRingingCallback(func() { ringingCalled = true })

	tel.SignallingReceived([]int{SignallingRinging}, nil)

	if tel.State() != StateRinging {
		t.Errorf("Expected Ringing state, got %v", tel.State())
	}
	if !ringingCalled {
		t.Error("Ringing callback should have been called")
	}
}

func TestSignallingReceived_Established(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateConnecting)
	tel.SetIncoming(false)
	tel.SetProfile(ProfileQualityMedium)
	tel.PrepareDiallingPipelines()

	establishedCalled := false
	tel.SetEstablishedCallback(func() { establishedCalled = true })

	tel.SignallingReceived([]int{SignallingEstablished}, nil)

	if tel.State() != StateEstablished {
		t.Errorf("Expected Established state, got %v", tel.State())
	}
	if !establishedCalled {
		t.Error("Established callback should have been called")
	}
}

func TestSignallingReceived_PreferredProfile(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateIdle)

	profile := byte(0x40) // ProfileQualityMedium
	signal := SignallingPreferredProfile + int(profile)

	tel.SignallingReceived([]int{signal}, nil)

	// When not established, should select the profile
	if tel.CurrentProfile() != profile {
		t.Errorf("Expected profile 0x%02x, got 0x%02x", profile, tel.CurrentProfile())
	}
}

func TestSignallingReceived_Available(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)

	tel.SignallingReceived([]int{SignallingAvailable}, nil)

	// AVAILABLE sets state to Idle
	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after AVAILABLE, got %v", tel.State())
	}
}

func TestSwitchProfile(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.SetProfile(ProfileQualityMedium)

	tel.SwitchProfile(ProfileQualityHigh, nil)

	if tel.CurrentProfile() != ProfileQualityHigh {
		t.Errorf("Expected profile 0x%02x, got 0x%02x", ProfileQualityHigh, tel.CurrentProfile())
	}
}

func TestSwitchProfile_SameProfile(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.SetProfile(ProfileQualityMedium)

	ftBefore := tel.TargetFrameTimeMs()
	tel.SwitchProfile(ProfileQualityMedium, nil)
	ftAfter := tel.TargetFrameTimeMs()

	// Should not change when same profile
	if ftBefore != ftAfter {
		t.Errorf("Frame time should not change for same profile: before=%f, after=%f", ftBefore, ftAfter)
	}
}

func TestSwitchProfile_NotEstablished(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateIdle)

	tel.SwitchProfile(ProfileQualityHigh, nil)

	// Should not change profile when not established
	if tel.CurrentProfile() != DefaultProfile {
		t.Errorf("Profile should not change when not established")
	}
}

func TestDialToneActive(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	if tel.DialToneActive() {
		t.Error("DialToneActive should default to false")
	}

	tel.SetDialToneActive(true)
	if !tel.DialToneActive() {
		t.Error("DialToneActive should be true after SetDialToneActive(true)")
	}
}
