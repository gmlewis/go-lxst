// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package telephony provides telephony primitives for audio calls over
// Reticulum. It implements tone generation for DTMF and call signaling
// (ringback, busy, ringtones), dialing state machines, and audio transport
// management for establishing and maintaining voice connections between
// Reticulum peers.
package telephony

import (
	"fmt"
	"sync"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
)

const (
	ProfileBandwidthUltraLow byte = 0x10
	ProfileBandwidthVeryLow  byte = 0x20
	ProfileBandwidthLow      byte = 0x30
	ProfileQualityMedium     byte = 0x40
	ProfileQualityHigh       byte = 0x50
	ProfileQualityMax        byte = 0x60
	ProfileLatencyLow        byte = 0x70
	ProfileLatencyUltraLow   byte = 0x80

	DefaultProfile byte = ProfileQualityMedium
)

var AvailableProfiles = []byte{
	ProfileBandwidthUltraLow,
	ProfileBandwidthVeryLow,
	ProfileBandwidthLow,
	ProfileQualityMedium,
	ProfileQualityHigh,
	ProfileQualityMax,
	ProfileLatencyLow,
	ProfileLatencyUltraLow,
}

func ProfileIndex(profile byte) int {
	for i, p := range AvailableProfiles {
		if p == profile {
			return i
		}
	}
	return -1
}

func ProfileName(profile byte) string {
	switch profile {
	case ProfileBandwidthUltraLow:
		return "Ultra Low Bandwidth"
	case ProfileBandwidthVeryLow:
		return "Very Low Bandwidth"
	case ProfileBandwidthLow:
		return "Low Bandwidth"
	case ProfileQualityMedium:
		return "Medium Quality"
	case ProfileQualityHigh:
		return "High Quality"
	case ProfileQualityMax:
		return "Super High Quality"
	case ProfileLatencyLow:
		return "Low Latency"
	case ProfileLatencyUltraLow:
		return "Ultra Low Latency"
	default:
		return "Default"
	}
}

func ProfileAbbreviation(profile byte) string {
	switch profile {
	case ProfileBandwidthUltraLow:
		return "ULBW"
	case ProfileBandwidthVeryLow:
		return "VLBW"
	case ProfileBandwidthLow:
		return "LBW"
	case ProfileQualityMedium:
		return "MQ"
	case ProfileQualityHigh:
		return "HQ"
	case ProfileQualityMax:
		return "SHQ"
	case ProfileLatencyLow:
		return "LL"
	case ProfileLatencyUltraLow:
		return "ULL"
	default:
		return "DFLT"
	}
}

func GetCodec(profile byte) (codecs.Codec, error) {
	switch profile {
	case ProfileBandwidthUltraLow:
		return codec2.NewCodec2(codec2.MODE_700C)
	case ProfileBandwidthVeryLow:
		return codec2.NewCodec2(codec2.MODE_1600)
	case ProfileBandwidthLow:
		return codec2.NewCodec2(codec2.MODE_3200)
	case ProfileQualityMedium:
		return opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	case ProfileQualityHigh:
		return opus.NewOpus(opus.PROFILE_VOICE_HIGH)
	case ProfileQualityMax:
		return opus.NewOpus(opus.PROFILE_VOICE_MAX)
	case ProfileLatencyLow:
		return opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	case ProfileLatencyUltraLow:
		return opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	default:
		return opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	}
}

func GetFrameTime(profile byte) float64 {
	switch profile {
	case ProfileBandwidthUltraLow:
		return 400.0
	case ProfileBandwidthVeryLow:
		return 320.0
	case ProfileBandwidthLow:
		return 200.0
	case ProfileQualityMedium:
		return 60.0
	case ProfileQualityHigh:
		return 60.0
	case ProfileQualityMax:
		return 60.0
	case ProfileLatencyLow:
		return 20.0
	case ProfileLatencyUltraLow:
		return 10.0
	default:
		return 60.0
	}
}

func NextProfile(profile byte) byte {
	idx := ProfileIndex(profile)
	if idx < 0 {
		return DefaultProfile
	}
	return AvailableProfiles[(idx+1)%len(AvailableProfiles)]
}

// Signalling status codes
const (
	SignallingBusy             byte = 0x00
	SignallingRejected         byte = 0x01
	SignallingCalling          byte = 0x02
	SignallingAvailable        byte = 0x03
	SignallingRinging          byte = 0x04
	SignallingConnecting       byte = 0x05
	SignallingEstablished      byte = 0x06
	SignallingPreferredProfile byte = 0xFF
)

var AutoStatusCodes = []byte{
	SignallingCalling,
	SignallingAvailable,
	SignallingRinging,
	SignallingConnecting,
	SignallingEstablished,
}

func StatusName(status byte) string {
	switch status {
	case SignallingBusy:
		return "Busy"
	case SignallingRejected:
		return "Rejected"
	case SignallingCalling:
		return "Calling"
	case SignallingAvailable:
		return "Available"
	case SignallingRinging:
		return "Ringing"
	case SignallingConnecting:
		return "Connecting"
	case SignallingEstablished:
		return "Established"
	default:
		return fmt.Sprintf("Unknown(0x%02x)", status)
	}
}

const (
	AllowAll  byte = 0xFF
	AllowNone byte = 0xFE
)

const (
	RingTime            = 60
	WaitTime            = 70
	ConnectTime         = 5
	DialToneFrequency   = 382.0
	DialToneEaseMs      = 3.14159
	JobIntervalSec      = 5
	AnnounceIntervalMin = 60 * 5
	AnnounceInterval    = 60 * 60 * 3
)

// TelephoneState represents the current state of a telephone call.
type TelephoneState byte

const (
	StateIdle TelephoneState = iota
	StateCalling
	StateRinging
	StateConnecting
	StateEstablished
	StateBusy
	StateRejected
	StateEnded
)

func (s TelephoneState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateCalling:
		return "Calling"
	case StateRinging:
		return "Ringing"
	case StateConnecting:
		return "Connecting"
	case StateEstablished:
		return "Established"
	case StateBusy:
		return "Busy"
	case StateRejected:
		return "Rejected"
	case StateEnded:
		return "Ended"
	default:
		return "Unknown"
	}
}

// StateFromSignalling converts a Signalling status code to a TelephoneState.
func StateFromSignalling(status byte) TelephoneState {
	switch status {
	case SignallingBusy:
		return StateBusy
	case SignallingRejected:
		return StateRejected
	case SignallingCalling:
		return StateCalling
	case SignallingAvailable:
		return StateIdle
	case SignallingRinging:
		return StateRinging
	case SignallingConnecting:
		return StateConnecting
	case SignallingEstablished:
		return StateEstablished
	default:
		return StateIdle
	}
}

// Telephone manages call state, audio pipelines, and signalling for a telephony endpoint.
type Telephone struct {
	mu             sync.Mutex
	ringTime       int
	waitTime       int
	connectTime    int
	autoAnswer     bool
	allowed        byte
	receiveGain    float64
	transmitGain   float64
	useAGC         bool
	state          TelephoneState
	currentProfile byte
	receiveMuted   bool
	transmitMuted  bool
	ringtonePath   string
	speakerDevice  string
	micDevice      string
	ringerDevice   string
	lowLatency     bool
	dialToneFreq   float64
	dialToneEaseMs float64
	externalBusy   bool
}

func NewTelephone(ringTime, waitTime int, autoAnswer bool, allowed byte, receiveGain, transmitGain float64) *Telephone {
	return &Telephone{
		ringTime:       ringTime,
		waitTime:       waitTime,
		connectTime:    ConnectTime,
		autoAnswer:     autoAnswer,
		allowed:        allowed,
		receiveGain:    receiveGain,
		transmitGain:   transmitGain,
		useAGC:         true,
		state:          StateIdle,
		currentProfile: DefaultProfile,
		dialToneFreq:   DialToneFrequency,
		dialToneEaseMs: DialToneEaseMs,
	}
}

func (tel *Telephone) State() TelephoneState {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.state
}

func (tel *Telephone) SetState(state TelephoneState) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.state = state
}

func (tel *Telephone) AutoAnswer() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.autoAnswer
}

func (tel *Telephone) SetAutoAnswer(auto bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.autoAnswer = auto
}

func (tel *Telephone) CurrentProfile() byte {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.currentProfile
}

func (tel *Telephone) SetProfile(profile byte) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.currentProfile = profile
}

func (tel *Telephone) ReceiveMuted() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.receiveMuted
}

func (tel *Telephone) MuteReceive(mute bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.receiveMuted = mute
}

func (tel *Telephone) UnmuteReceive(unmute bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.receiveMuted = !unmute
}

func (tel *Telephone) TransmitMuted() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.transmitMuted
}

func (tel *Telephone) MuteTransmit(mute bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.transmitMuted = mute
}

func (tel *Telephone) UnmuteTransmit(unmute bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.transmitMuted = !unmute
}

func (tel *Telephone) ReceiveGain() float64 {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.receiveGain
}

func (tel *Telephone) SetReceiveGain(gain float64) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.receiveGain = gain
}

func (tel *Telephone) TransmitGain() float64 {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.transmitGain
}

func (tel *Telephone) SetTransmitGain(gain float64) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.transmitGain = gain
}

func (tel *Telephone) RingTime() int {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringTime
}

func (tel *Telephone) WaitTime() int {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.waitTime
}

func (tel *Telephone) IsEstablished() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.state == StateEstablished
}

func (tel *Telephone) IsRinging() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.state == StateRinging
}

func (tel *Telephone) IsCalling() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.state == StateCalling
}

func (tel *Telephone) IsIdle() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.state == StateIdle
}

// Hangup terminates the current call and resets state.
func (tel *Telephone) Hangup() {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.state = StateIdle
}

// Answer accepts an incoming call.
func (tel *Telephone) Answer() {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	if tel.state == StateRinging {
		tel.state = StateConnecting
	}
}

// Call initiates an outgoing call.
func (tel *Telephone) Call(profile byte) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	if tel.state == StateIdle {
		tel.state = StateCalling
		tel.currentProfile = profile
	}
}

func (tel *Telephone) RingtonePath() string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringtonePath
}

func (tel *Telephone) SetRingtonePath(path string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.ringtonePath = path
}

func (tel *Telephone) SpeakerDevice() string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.speakerDevice
}

func (tel *Telephone) SetSpeakerDevice(device string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.speakerDevice = device
}

func (tel *Telephone) MicDevice() string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.micDevice
}

func (tel *Telephone) SetMicDevice(device string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.micDevice = device
}

func (tel *Telephone) RingerDevice() string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringerDevice
}

func (tel *Telephone) SetRingerDevice(device string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.ringerDevice = device
}

func (tel *Telephone) LowLatency() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.lowLatency
}

func (tel *Telephone) SetLowLatency(ll bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.lowLatency = ll
}

func (tel *Telephone) UseAGC() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.useAGC
}

func (tel *Telephone) SetUseAGC(use bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.useAGC = use
}

// SetBusy sets the external busy state of the telephone.
func (tel *Telephone) SetBusy(busy bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.externalBusy = busy
}

// Busy reports whether the telephone is busy.
// Returns true if the call status is not Idle or if external busy is set.
func (tel *Telephone) Busy() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	if tel.state != StateIdle {
		return true
	}
	return tel.externalBusy
}

// ActiveProfile returns the profile of the active call, or nil if no call is active.
func (tel *Telephone) ActiveProfile() byte {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	if tel.state == StateIdle {
		return 0
	}
	return tel.currentProfile
}
