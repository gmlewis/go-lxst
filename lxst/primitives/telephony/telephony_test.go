// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package telephony

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
)

func TestProfileIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile byte
		want    int
	}{
		{ProfileBandwidthUltraLow, 0},
		{ProfileBandwidthVeryLow, 1},
		{ProfileBandwidthLow, 2},
		{ProfileQualityMedium, 3},
		{ProfileQualityHigh, 4},
		{ProfileQualityMax, 5},
		{ProfileLatencyLow, 6},
		{ProfileLatencyUltraLow, 7},
		{0x99, -1},
	}

	for _, tt := range tests {
		got := ProfileIndex(tt.profile)
		if got != tt.want {
			t.Errorf("ProfileIndex(0x%02x) = %d, want %d", tt.profile, got, tt.want)
		}
	}
}

func TestProfileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile byte
		want    string
	}{
		{ProfileBandwidthUltraLow, "Ultra Low Bandwidth"},
		{ProfileBandwidthVeryLow, "Very Low Bandwidth"},
		{ProfileBandwidthLow, "Low Bandwidth"},
		{ProfileQualityMedium, "Medium Quality"},
		{ProfileQualityHigh, "High Quality"},
		{ProfileQualityMax, "Super High Quality"},
		{ProfileLatencyLow, "Low Latency"},
		{ProfileLatencyUltraLow, "Ultra Low Latency"},
		{0x99, "Default"},
	}

	for _, tt := range tests {
		got := ProfileName(tt.profile)
		if got != tt.want {
			t.Errorf("ProfileName(0x%02x) = %q, want %q", tt.profile, got, tt.want)
		}
	}
}

func TestProfileAbbreviation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile byte
		want    string
	}{
		{ProfileBandwidthUltraLow, "ULBW"},
		{ProfileBandwidthVeryLow, "VLBW"},
		{ProfileBandwidthLow, "LBW"},
		{ProfileQualityMedium, "MQ"},
		{ProfileQualityHigh, "HQ"},
		{ProfileQualityMax, "SHQ"},
		{ProfileLatencyLow, "LL"},
		{ProfileLatencyUltraLow, "ULL"},
		{0x99, "DFLT"},
	}

	for _, tt := range tests {
		got := ProfileAbbreviation(tt.profile)
		if got != tt.want {
			t.Errorf("ProfileAbbreviation(0x%02x) = %q, want %q", tt.profile, got, tt.want)
		}
	}
}

func TestGetFrameTime(t *testing.T) {
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
		{0x99, 60.0},
	}

	for _, tt := range tests {
		got := GetFrameTime(tt.profile)
		if got != tt.want {
			t.Errorf("GetFrameTime(0x%02x) = %f, want %f", tt.profile, got, tt.want)
		}
	}
}

func TestGetCodec_BandwidthUltraLow(t *testing.T) {
	t.Parallel()

	c, err := GetCodec(ProfileBandwidthUltraLow)
	if err != nil {
		t.Fatalf("GetCodec failed: %v", err)
	}
	if _, ok := c.(*codec2.Codec2); !ok {
		t.Error("Expected Codec2 codec for BANDWIDTH_ULTRA_LOW")
	}
}

func TestGetCodec_QualityMedium(t *testing.T) {
	t.Parallel()

	c, err := GetCodec(ProfileQualityMedium)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	if _, ok := c.(*opus.Opus); !ok {
		t.Error("Expected Opus codec for QUALITY_MEDIUM")
	}
}

func TestGetCodec_QualityMax(t *testing.T) {
	t.Parallel()

	c, err := GetCodec(ProfileQualityMax)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}
	if _, ok := c.(*opus.Opus); !ok {
		t.Error("Expected Opus codec for QUALITY_MAX")
	}
}

func TestNextProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile byte
		want    byte
	}{
		{ProfileBandwidthUltraLow, ProfileBandwidthVeryLow},
		{ProfileQualityMedium, ProfileQualityHigh},
		{ProfileLatencyUltraLow, ProfileBandwidthUltraLow},
	}

	for _, tt := range tests {
		got := NextProfile(tt.profile)
		if got != tt.want {
			t.Errorf("NextProfile(0x%02x) = 0x%02x, want 0x%02x", tt.profile, got, tt.want)
		}
	}
}

func TestNextProfile_Invalid(t *testing.T) {
	t.Parallel()

	got := NextProfile(0x99)
	if got != DefaultProfile {
		t.Errorf("NextProfile(0x99) = 0x%02x, want 0x%02x", got, DefaultProfile)
	}
}

func TestStatusName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status byte
		want  string
	}{
		{SignallingBusy, "Busy"},
		{SignallingRejected, "Rejected"},
		{SignallingCalling, "Calling"},
		{SignallingAvailable, "Available"},
		{SignallingRinging, "Ringing"},
		{SignallingConnecting, "Connecting"},
		{SignallingEstablished, "Established"},
	}

	for _, tt := range tests {
		got := StatusName(tt.status)
		if got != tt.want {
			t.Errorf("StatusName(0x%02x) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestAvailableProfiles(t *testing.T) {
	t.Parallel()

	if len(AvailableProfiles) != 8 {
		t.Errorf("Expected 8 available profiles, got %d", len(AvailableProfiles))
	}
}

func TestAutoStatusCodes(t *testing.T) {
	t.Parallel()

	if len(AutoStatusCodes) != 5 {
		t.Errorf("Expected 5 auto status codes, got %d", len(AutoStatusCodes))
	}
}

func TestTelephone_New(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if tel == nil {
		t.Fatal("NewTelephone returned nil")
	}
	if !tel.IsIdle() {
		t.Error("Telephone should start idle")
	}
	if tel.State() != StateIdle {
		t.Error("Telephone should start in Idle state")
	}
}

func TestTelephone_Call(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.Call(ProfileQualityMedium)

	if !tel.IsCalling() {
		t.Error("Telephone should be in Calling state after Call()")
	}
	if tel.CurrentProfile() != ProfileQualityMedium {
		t.Errorf("Expected profile 0x%02x, got 0x%02x", ProfileQualityMedium, tel.CurrentProfile())
	}
}

func TestTelephone_CallFromNonIdle(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.Call(ProfileQualityMedium)

	if tel.State() != StateEstablished {
		t.Error("Telephone should stay in Established state if Call() from non-idle")
	}
}

func TestTelephone_Answer(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.Answer()

	if tel.State() != StateConnecting {
		t.Error("Telephone should be in Connecting state after Answer()")
	}
}

func TestTelephone_AnswerFromNonRinging(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.Answer()

	if tel.State() != StateIdle {
		t.Error("Telephone should stay in Idle state if Answer() from non-ringing")
	}
}

func TestTelephone_Hangup(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.Hangup()

	if !tel.IsIdle() {
		t.Error("Telephone should be Idle after Hangup()")
	}
}

func TestTelephone_MuteReceive(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.MuteReceive(true)
	if !tel.ReceiveMuted() {
		t.Error("Receive should be muted")
	}
	tel.UnmuteReceive(true)
	if tel.ReceiveMuted() {
		t.Error("Receive should be unmuted")
	}
}

func TestTelephone_MuteTransmit(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.MuteTransmit(true)
	if !tel.TransmitMuted() {
		t.Error("Transmit should be muted")
	}
	tel.UnmuteTransmit(true)
	if tel.TransmitMuted() {
		t.Error("Transmit should be unmuted")
	}
}

func TestTelephone_Gain(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 5.0, 3.0)
	if tel.ReceiveGain() != 5.0 {
		t.Errorf("Expected receive gain 5.0, got %f", tel.ReceiveGain())
	}
	if tel.TransmitGain() != 3.0 {
		t.Errorf("Expected transmit gain 3.0, got %f", tel.TransmitGain())
	}

	tel.SetReceiveGain(10.0)
	if tel.ReceiveGain() != 10.0 {
		t.Errorf("Expected receive gain 10.0, got %f", tel.ReceiveGain())
	}
}

func TestTelephone_Profile(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if tel.CurrentProfile() != DefaultProfile {
		t.Errorf("Expected default profile 0x%02x, got 0x%02x", DefaultProfile, tel.CurrentProfile())
	}
	tel.SetProfile(ProfileQualityHigh)
	if tel.CurrentProfile() != ProfileQualityHigh {
		t.Errorf("Expected profile 0x%02x, got 0x%02x", ProfileQualityHigh, tel.CurrentProfile())
	}
}

func TestTelephone_AutoAnswer(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if !tel.AutoAnswer() {
		t.Error("AutoAnswer should default to true")
	}
	tel.SetAutoAnswer(false)
	if tel.AutoAnswer() {
		t.Error("AutoAnswer should be false after SetAutoAnswer(false)")
	}
}

func TestTelephone_IsEstablished(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if tel.IsEstablished() {
		t.Error("Should not be established initially")
	}
	tel.SetState(StateEstablished)
	if !tel.IsEstablished() {
		t.Error("Should be established after SetState")
	}
}

func TestStateFromSignalling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status byte
		want   TelephoneState
	}{
		{SignallingBusy, StateBusy},
		{SignallingRejected, StateRejected},
		{SignallingCalling, StateCalling},
		{SignallingAvailable, StateIdle},
		{SignallingRinging, StateRinging},
		{SignallingConnecting, StateConnecting},
		{SignallingEstablished, StateEstablished},
		{0xFF, StateIdle},
	}

	for _, tt := range tests {
		got := StateFromSignalling(tt.status)
		if got != tt.want {
			t.Errorf("StateFromSignalling(0x%02x) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestTelephone_Devices(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	tel.SetSpeakerDevice("speaker")
	tel.SetMicDevice("mic")
	tel.SetRingtonePath("/path/to/ringtone.opus")

	if tel.SpeakerDevice() != "speaker" {
		t.Errorf("Expected 'speaker', got %q", tel.SpeakerDevice())
	}
	if tel.MicDevice() != "mic" {
		t.Errorf("Expected 'mic', got %q", tel.MicDevice())
	}
	if tel.RingtonePath() != "/path/to/ringtone.opus" {
		t.Errorf("Expected ringtone path, got %q", tel.RingtonePath())
	}
}

func TestTelephone_LowLatency(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if tel.LowLatency() {
		t.Error("LowLatency should default to false")
	}
	tel.SetLowLatency(true)
	if !tel.LowLatency() {
		t.Error("LowLatency should be true after SetLowLatency(true)")
	}
}

func TestTelephone_UseAGC(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(60, 70, true, AllowAll, 0.0, 0.0)
	if !tel.UseAGC() {
		t.Error("AGC should default to true")
	}
	tel.SetUseAGC(false)
	if tel.UseAGC() {
		t.Error("AGC should be false after SetUseAGC(false)")
	}
}