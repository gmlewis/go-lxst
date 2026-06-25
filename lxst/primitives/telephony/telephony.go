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
	"log"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/filters"
	"github.com/gmlewis/go-lxst/lxst/generators"
	"github.com/gmlewis/go-lxst/lxst/mixer"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/pipeline"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

// Time functions are package-level variables for testability.
// In production they use real time; in tests they can be replaced
// with simulated versions.
var (
	now        = time.Now
	elapsed    = func(start time.Time, dur float64) bool { return time.Since(start).Seconds() < dur }
	elapsedMod = func(start time.Time, window float64) float64 { return mathMod(time.Since(start).Seconds(), window) }
	sleep      = time.Sleep
)

func mathMod(a, b float64) float64 {
	result := a - b*float64(int(a/b))
	if result < 0 {
		result += b
	}
	return result
}

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

// Signalling status codes. These are int (not byte) because
// PREFERRED_PROFILE + profile can exceed 255 (e.g. 0xFF + 0x80 = 0x17F).
const (
	SignallingBusy             int = 0x00
	SignallingRejected         int = 0x01
	SignallingCalling          int = 0x02
	SignallingAvailable        int = 0x03
	SignallingRinging          int = 0x04
	SignallingConnecting       int = 0x05
	SignallingEstablished      int = 0x06
	SignallingPreferredProfile int = 0xFF
)

var AutoStatusCodes = []int{
	SignallingCalling,
	SignallingAvailable,
	SignallingRinging,
	SignallingConnecting,
	SignallingEstablished,
}

func StatusName(status int) string {
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
	BusyToneSeconds     = 4.25
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
func StateFromSignalling(status int) TelephoneState {
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
	pipelineLock   sync.Mutex
	ringerLock     sync.Mutex
	ringTime       int
	waitTime       int
	connectTime    int
	autoAnswer     time.Duration
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
	busyToneSecs   float64
	incoming       bool

	// Pipeline components
	targetFrameTimeMs float64
	receiveCodec      codecs.Codec
	transmitCodec     codecs.Codec
	audioOutput       *sinks.LineSink
	audioInput        *sources.LineSource
	dialTone          *generators.ToneSource
	receiveMixer      *mixer.Mixer
	transmitMixer     *mixer.Mixer
	receivePipeline   *pipeline.Pipeline
	transmitPipeline  *pipeline.Pipeline

	// Ringer components
	ringerOutput   *sinks.LineSink
	ringerSource   *sources.OpusFileSource
	ringerPipeline *pipeline.Pipeline

	// Transmit pipeline components
	filters    []filters.Filter
	packetizer *network.Packetizer

	// Callbacks for call lifecycle events
	ringingCallback     func()
	establishedCallback func()
	endedCallback       func()
	busyCallback        func()
	rejectedCallback    func()

	// Caller access control
	blockedList []string
	allowList   []string

	// Timeouts
	establishmentTimeout int
	announceInterval     int

	// Dialling state
	dialToneActive bool

	// Auto-answer callback set by the endpoint, called after the
	// auto-answer delay expires. Matching Python's __caller_identified
	// which spawns a thread to call self.answer(identity).
	autoAnswerFunc func()
}

func NewTelephone(ringTime, waitTime int, autoAnswer time.Duration, allowed byte, receiveGain, transmitGain float64) *Telephone {
	return &Telephone{
		ringTime:             ringTime,
		waitTime:             waitTime,
		connectTime:          ConnectTime,
		autoAnswer:           autoAnswer,
		allowed:              allowed,
		receiveGain:          receiveGain,
		transmitGain:         transmitGain,
		useAGC:               true,
		state:                StateIdle,
		currentProfile:       DefaultProfile,
		dialToneFreq:         DialToneFrequency,
		dialToneEaseMs:       DialToneEaseMs,
		busyToneSecs:         BusyToneSeconds,
		establishmentTimeout: ConnectTime,
		announceInterval:     AnnounceInterval,
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

func (tel *Telephone) AutoAnswer() time.Duration {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.autoAnswer
}

func (tel *Telephone) SetAutoAnswer(auto time.Duration) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.autoAnswer = auto
}

// SetAutoAnswerFunc sets the callback invoked when the auto-answer
// delay expires after a caller is identified. The endpoint sets this
// to its Answer method so the auto-answer can open pipelines and
// signal ESTABLISHED, matching Python's __caller_identified flow.
func (tel *Telephone) SetAutoAnswerFunc(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.autoAnswerFunc = fn
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

// Hangup terminates the current call, stops pipelines, resets state,
// and fires the ended callback.
func (tel *Telephone) Hangup() {
	tel.mu.Lock()
	if tel.state == StateIdle {
		tel.mu.Unlock()
		return
	}
	tel.mu.Unlock()

	tel.StopPipelines()

	tel.pipelineLock.Lock()
	tel.receiveMixer = nil
	tel.transmitMixer = nil
	tel.receivePipeline = nil
	tel.transmitPipeline = nil
	tel.audioOutput = nil
	tel.audioInput = nil
	tel.dialTone = nil
	tel.pipelineLock.Unlock()

	tel.mu.Lock()
	tel.state = StateIdle
	tel.receiveMuted = false
	tel.transmitMuted = false
	cb := tel.endedCallback
	tel.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// HangupWithReason terminates the call with a specific reason code.
// It fires the busy callback for BUSY, rejected callback for REJECTED,
// or the ended callback for no specific reason.
func (tel *Telephone) HangupWithReason(reason int) {
	tel.mu.Lock()
	if tel.state == StateIdle {
		tel.mu.Unlock()
		return
	}
	tel.mu.Unlock()

	tel.StopPipelines()

	tel.pipelineLock.Lock()
	tel.receiveMixer = nil
	tel.transmitMixer = nil
	tel.receivePipeline = nil
	tel.transmitPipeline = nil
	tel.audioOutput = nil
	tel.audioInput = nil
	tel.dialTone = nil
	tel.pipelineLock.Unlock()

	tel.mu.Lock()
	tel.state = StateIdle
	tel.receiveMuted = false
	tel.transmitMuted = false

	var cb func()
	switch reason {
	case SignallingBusy:
		cb = tel.busyCallback
		if cb == nil {
			cb = tel.endedCallback
		}
	case SignallingRejected:
		cb = tel.rejectedCallback
		if cb == nil {
			cb = tel.endedCallback
		}
	default:
		cb = tel.endedCallback
	}
	tel.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// StartRingTimeout starts a goroutine that hangs up the call after the
// ring timeout if the call is still in Ringing state (incoming call not
// answered). This matches the Python __timeout_incoming_call_at method.
func (tel *Telephone) StartRingTimeout() {
	go func() {
		deadline := time.Now().Add(time.Duration(tel.RingTime()) * time.Second)
		for time.Now().Before(deadline) {
			tel.mu.Lock()
			state := tel.state
			incoming := tel.incoming
			tel.mu.Unlock()
			if state != StateRinging || !incoming {
				return
			}
			sleep(250 * time.Millisecond)
		}
		tel.mu.Lock()
		state := tel.state
		incoming := tel.incoming
		tel.mu.Unlock()
		if state == StateRinging && incoming {
			tel.Hangup()
		}
	}()
}

// StartCallTimeout starts a goroutine that hangs up the outgoing call
// after the wait timeout if the call is not yet established. This matches
// the Python __timeout_outgoing_call_at method.
func (tel *Telephone) StartCallTimeout() {
	go func() {
		deadline := time.Now().Add(time.Duration(tel.WaitTime()) * time.Second)
		for time.Now().Before(deadline) {
			tel.mu.Lock()
			state := tel.state
			tel.mu.Unlock()
			if state == StateIdle || state == StateEstablished {
				return
			}
			sleep(250 * time.Millisecond)
		}
		tel.mu.Lock()
		state := tel.state
		tel.mu.Unlock()
		if state != StateIdle && state != StateEstablished {
			tel.Hangup()
		}
	}()
}

// StartEstablishmentTimeout starts a goroutine that hangs up the outgoing
// call after the establishment timeout if the link has not been established.
// This matches the Python __timeout_outgoing_establishment_at method.
func (tel *Telephone) StartEstablishmentTimeout() {
	go func() {
		deadline := time.Now().Add(time.Duration(tel.ConnectTimeout()) * time.Second)
		for time.Now().Before(deadline) {
			tel.mu.Lock()
			state := tel.state
			tel.mu.Unlock()
			if state == StateIdle || state == StateRinging {
				return
			}
			sleep(250 * time.Millisecond)
		}
		tel.mu.Lock()
		state := tel.state
		tel.mu.Unlock()
		if state != StateIdle && state != StateRinging {
			tel.Hangup()
		}
	}()
}

// Answer accepts an incoming call. It transitions from Ringing to Established
// state and fires the established callback. The caller is responsible for
// calling OpenPipelines() and StartPipelines() after Answer() returns true.
// Returns true if the call was answered, false if not in Ringing state.
func (tel *Telephone) Answer() bool {
	tel.mu.Lock()
	if tel.state != StateRinging {
		log.Printf("Telephone.Answer: not ringing (state=%v), returning false", tel.state)
		tel.mu.Unlock()
		return false
	}

	tel.state = StateEstablished
	cb := tel.establishedCallback
	ll := tel.lowLatency
	tel.mu.Unlock()

	log.Printf("Telephone.Answer: state=Established, lowLatency=%v, establishedCallback=%v", ll, cb != nil)

	tel.pipelineLock.Lock()
	ao := tel.audioOutput
	tel.pipelineLock.Unlock()

	if ll && ao != nil {
		ao.EnableLowLatency()
	}
	if cb != nil {
		cb()
	}

	return true
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

// Signal sends a signalling code and updates the call state for
// auto status codes (CALLING, AVAILABLE, RINGING, CONNECTING, ESTABLISHED).
func (tel *Telephone) Signal(signal int, sendFunc func(data []byte) error, immediate bool) {
	if isAutoStatusCode(signal) {
		tel.mu.Lock()
		tel.state = StateFromSignalling(signal)
		tel.mu.Unlock()
	}

	if sendFunc != nil {
		signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
		packed, err := network.PackData(signallingData)
		if err != nil {
			return
		}
		_ = sendFunc(packed)
	}
}

func isAutoStatusCode(code int) bool {
	for _, c := range AutoStatusCodes {
		if c == code {
			return true
		}
	}
	return false
}

// OutgoingLinkEstablished handles the callback when an outgoing RNS
// link is established. Matching Python's __outgoing_link_established,
// it does NOT change the call status — it only ensures the packet
// callback is wired up (handled by the endpoint). The call state
// transitions are driven entirely by signalling messages.
func (tel *Telephone) OutgoingLinkEstablished(signalFunc func(int) error) {
	// Python __outgoing_link_established does not change call_status.
	// State transitions are driven by signalling_received only.
}

// LinkClosed handles the callback when the RNS link is closed by the
// remote peer. It terminates the call and fires the ended callback.
func (tel *Telephone) LinkClosed() {
	tel.Hangup()
}

// PacketizerFailure handles a frame packetization failure by terminating
// the call. It is called when the Packetizer cannot send audio data.
func (tel *Telephone) PacketizerFailure() {
	tel.Hangup()
}

// ConnectTimeout returns the establishment timeout in seconds.
func (tel *Telephone) ConnectTimeout() int {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.establishmentTimeout
}

// SetConnectTimeout sets the establishment timeout in seconds.
func (tel *Telephone) SetConnectTimeout(timeout int) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.establishmentTimeout = timeout
}

// AnnounceInterval returns the announce interval in seconds.
func (tel *Telephone) AnnounceInterval() int {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.announceInterval
}

// SetAnnounceInterval sets the announce interval in seconds.
// Values below AnnounceIntervalMin are clamped to the minimum.
func (tel *Telephone) SetAnnounceInterval(interval int) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	if interval < AnnounceIntervalMin {
		interval = AnnounceIntervalMin
	}
	tel.announceInterval = interval
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

// SetAllowed sets the allowed callers policy. Valid values are AllowAll
// and AllowNone. To use an explicit allow list, set allowed to AllowNone
// and use SetAllowList.
func (tel *Telephone) SetAllowed(allowed byte) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.allowed = allowed
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

// TargetFrameTimeMs returns the current target frame time in milliseconds.
func (tel *Telephone) TargetFrameTimeMs() float64 {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.targetFrameTimeMs
}

// ReceiveCodec returns the current receive codec.
func (tel *Telephone) ReceiveCodec() codecs.Codec {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.receiveCodec
}

// TransmitCodec returns the current transmit codec.
func (tel *Telephone) TransmitCodec() codecs.Codec {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.transmitCodec
}

// AudioOutput returns the current audio output sink.
func (tel *Telephone) AudioOutput() *sinks.LineSink {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.audioOutput
}

// AudioInput returns the current audio input source.
func (tel *Telephone) AudioInput() *sources.LineSource {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.audioInput
}

// DialTone returns the current dial tone generator.
func (tel *Telephone) DialTone() *generators.ToneSource {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.dialTone
}

// ReceiveMixer returns the current receive mixer.
func (tel *Telephone) ReceiveMixer() *mixer.Mixer {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.receiveMixer
}

// TransmitMixer returns the current transmit mixer.
func (tel *Telephone) TransmitMixer() *mixer.Mixer {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.transmitMixer
}

// ReceivePipeline returns the current receive pipeline.
func (tel *Telephone) ReceivePipeline() *pipeline.Pipeline {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.receivePipeline
}

// TransmitPipeline returns the current transmit pipeline.
func (tel *Telephone) TransmitPipeline() *pipeline.Pipeline {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.transmitPipeline
}

// BusyToneSeconds returns the busy tone duration in seconds.
func (tel *Telephone) BusyToneSeconds() float64 {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.busyToneSecs
}

// SetBusyToneSeconds sets the busy tone duration in seconds.
func (tel *Telephone) SetBusyToneSeconds(secs float64) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.busyToneSecs = secs
}

// Filters returns the current audio filter chain.
func (tel *Telephone) Filters() []filters.Filter {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.filters
}

// SetFilters sets the audio filter chain for the transmit pipeline.
func (tel *Telephone) SetFilters(f []filters.Filter) {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	tel.filters = f
}

// Packetizer returns the current network packetizer.
func (tel *Telephone) Packetizer() *network.Packetizer {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	return tel.packetizer
}

// SetPacketizer sets the network packetizer for the transmit pipeline.
func (tel *Telephone) SetPacketizer(p *network.Packetizer) {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	tel.packetizer = p
}

// selectCallCodecs sets receive codec to Null and transmit codec to the
// codec for the given profile, matching the Python __select_call_codecs.
func (tel *Telephone) selectCallCodecs(profile byte) {
	tel.receiveCodec = codecs.NullCodec{}
	teleCodec, err := GetCodec(profile)
	if err != nil {
		teleCodec = codecs.NullCodec{}
	}
	tel.transmitCodec = teleCodec
}

// selectCallFrameTime sets the target frame time based on the profile.
func (tel *Telephone) selectCallFrameTime(profile byte) {
	tel.targetFrameTimeMs = GetFrameTime(profile)
}

// selectCallProfile selects the call profile, codecs, and frame time.
func (tel *Telephone) selectCallProfile(profile byte) {
	if profile == 0 {
		profile = DefaultProfile
	}
	tel.currentProfile = profile
	tel.selectCallCodecs(profile)
	tel.selectCallFrameTime(profile)
}

// PrepareDiallingPipelines creates the audio pipelines needed for call setup,
// matching the Python __prepare_dialling_pipelines method. It sets up the
// receive mixer, dial tone generator, and receive pipeline.
func (tel *Telephone) PrepareDiallingPipelines() {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()
	tel.prepareDiallingPipelinesLocked()
}

func (tel *Telephone) prepareDiallingPipelinesLocked() {
	tel.selectCallProfile(tel.currentProfile)

	if tel.audioOutput == nil {
		tel.audioOutput = sinks.NewLineSink(tel.speakerDevice, true, false)
	}

	if tel.receiveMixer == nil {
		tel.receiveMixer = mixer.NewMixer(tel.targetFrameTimeMs, 0, nil, nil, tel.receiveGain)
	}

	if tel.dialTone == nil {
		tel.dialTone = generators.NewToneSource(
			tel.dialToneFreq, 0.0, true, tel.dialToneEaseMs,
			tel.targetFrameTimeMs, codecs.NullCodec{}, tel.receiveMixer, 1,
		)
	}

	if tel.receivePipeline == nil {
		var err error
		tel.receivePipeline, err = pipeline.NewPipeline(
			tel.receiveMixer, codecs.NullCodec{}, tel.audioOutput,
		)
		if err != nil {
			log.Printf("prepareDiallingPipelinesLocked: NewPipeline failed: %v", err)
		}
	}
}

// ResetDiallingPipelines stops and recreates the dialling pipelines,
// matching the Python __reset_dialling_pipelines method. It stops all
// running pipeline components and reinitializes them.
func (tel *Telephone) ResetDiallingPipelines() {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()

	if tel.audioOutput != nil {
		_ = tel.audioOutput.Stop()
	}
	if tel.dialTone != nil {
		_ = tel.dialTone.Stop()
	}
	if tel.receivePipeline != nil {
		_ = tel.receivePipeline.Stop()
	}
	if tel.receiveMixer != nil {
		_ = tel.receiveMixer.Stop()
	}

	tel.audioOutput = nil
	tel.dialTone = nil
	tel.receivePipeline = nil
	tel.receiveMixer = nil

	tel.selectCallProfile(tel.currentProfile)

	tel.audioOutput = sinks.NewLineSink(tel.speakerDevice, true, false)
	tel.receiveMixer = mixer.NewMixer(tel.targetFrameTimeMs, 0, nil, nil, tel.receiveGain)
	tel.dialTone = generators.NewToneSource(
		tel.dialToneFreq, 0.0, true, tel.dialToneEaseMs,
		tel.targetFrameTimeMs, codecs.NullCodec{}, tel.receiveMixer, 1,
	)
	var err error
	tel.receivePipeline, err = pipeline.NewPipeline(
		tel.receiveMixer, codecs.NullCodec{}, tel.audioOutput,
	)
	if err != nil {
		log.Printf("ResetDiallingPipelines: NewPipeline failed: %v", err)
	}
}

// Incoming reports whether the current call is an incoming call.
func (tel *Telephone) Incoming() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.incoming
}

// SetIncoming sets whether the call is incoming.
func (tel *Telephone) SetIncoming(incoming bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.incoming = incoming
}

// RingerOutput returns the ringer audio output sink.
func (tel *Telephone) RingerOutput() *sinks.LineSink {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringerOutput
}

// RingerSource returns the ringer audio file source.
func (tel *Telephone) RingerSource() *sources.OpusFileSource {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringerSource
}

// RingerPipeline returns the ringer audio pipeline.
func (tel *Telephone) RingerPipeline() *pipeline.Pipeline {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.ringerPipeline
}

// ActivateRingTone activates the ringtone playback when an incoming call
// is received, matching the Python __activate_ring_tone method. If a
// ringtone path is set and the file exists, it creates a ringer pipeline
// and starts playback in a goroutine. The ringtone plays on loop while
// the call remains in the Ringing state and is incoming.
func (tel *Telephone) ActivateRingTone() {
	tel.mu.Lock()
	ringtonePath := tel.ringtonePath
	ringerDevice := tel.ringerDevice
	tel.mu.Unlock()

	if ringtonePath == "" {
		return
	}

	tel.ringerLock.Lock()
	defer tel.ringerLock.Unlock()

	if tel.ringerPipeline == nil {
		if tel.ringerOutput == nil {
			tel.ringerOutput = sinks.NewLineSink(ringerDevice, true, false)
		}

		src, err := sources.NewOpusFileSource(ringtonePath, 60.0, true, nil, nil, false)
		if err != nil {
			return
		}
		tel.ringerSource = src

		tel.ringerPipeline, err = pipeline.NewPipeline(
			tel.ringerSource, codecs.NullCodec{}, tel.ringerOutput,
		)
		if err != nil {
			log.Printf("ActivateRingTone: NewPipeline failed: %v", err)
		}
	}

	go func() {
		tel.ringerLock.Lock()
		defer tel.ringerLock.Unlock()

		for {
			tel.mu.Lock()
			isRinging := tel.state == StateRinging && tel.incoming
			tel.mu.Unlock()

			if !isRinging {
				break
			}

			if tel.ringerPipeline != nil && !tel.ringerPipeline.Running() {
				_ = tel.ringerPipeline.Start()
			}
		}

		if tel.ringerSource != nil {
			_ = tel.ringerSource.Stop()
		}
	}()
}

// StopRingTone stops the ringer pipeline and source.
func (tel *Telephone) StopRingTone() {
	tel.ringerLock.Lock()
	defer tel.ringerLock.Unlock()

	if tel.ringerSource != nil {
		_ = tel.ringerSource.Stop()
	}
	if tel.ringerPipeline != nil {
		_ = tel.ringerPipeline.Stop()
	}
}

// EnableDialTone starts the receive mixer if needed and enables the dial
// tone at a low gain level, matching the Python __enable_dial_tone method.
func (tel *Telephone) EnableDialTone() {
	tel.pipelineLock.Lock()
	receiveMixer := tel.receiveMixer
	dialTone := tel.dialTone
	tel.pipelineLock.Unlock()

	if receiveMixer != nil && !receiveMixer.Running() {
		_ = receiveMixer.Start()
	}

	if dialTone != nil {
		dialTone.SetGain(0.04)
		if !dialTone.Running() {
			_ = dialTone.Start()
		}
	}
}

// MuteDialTone mutes the dial tone by setting gain to 0 while keeping it
// running, matching the Python __mute_dial_tone method.
func (tel *Telephone) MuteDialTone() {
	tel.pipelineLock.Lock()
	receiveMixer := tel.receiveMixer
	dialTone := tel.dialTone
	tel.pipelineLock.Unlock()

	if receiveMixer != nil && !receiveMixer.Running() {
		_ = receiveMixer.Start()
	}

	if dialTone != nil && dialTone.Running() && dialTone.Gain() != 0 {
		dialTone.SetGain(0.0)
	}

	if dialTone != nil && !dialTone.Running() {
		_ = dialTone.Start()
	}
}

// DisableDialTone stops the dial tone entirely, matching the Python
// __disable_dial_tone method.
func (tel *Telephone) DisableDialTone() {
	tel.pipelineLock.Lock()
	dialTone := tel.dialTone
	tel.pipelineLock.Unlock()

	if dialTone != nil && dialTone.Running() {
		_ = dialTone.Stop()
	}
}

// ActivateDialTone starts the dial tone pattern for an outgoing call,
// matching the Python __activate_dial_tone method. It plays the dial tone
// in a 7-second window: enabled from 0.05s to 2.05s, muted for the rest.
// The pattern continues while the call is outgoing and in the Ringing state.
func (tel *Telephone) ActivateDialTone() {
	go func() {
		window := 7.0
		started := now()
		for {
			tel.mu.Lock()
			isOutgoingRinging := !tel.incoming && tel.state == StateRinging
			tel.mu.Unlock()

			if !isOutgoingRinging {
				break
			}

			e := elapsedMod(started, window)
			if e > 0.05 && e < 2.05 {
				tel.EnableDialTone()
			} else {
				tel.MuteDialTone()
			}

			sleep(200 * time.Millisecond)
		}
	}()
}

// PlayBusyTone plays an intermittent busy tone for the configured duration,
// matching the Python __play_busy_tone method. The busy tone alternates
// between muted and enabled at 0.5s intervals. This method blocks for
// busyToneSeconds + 0.5s.
func (tel *Telephone) PlayBusyTone() {
	tel.mu.Lock()
	busyToneSecs := tel.busyToneSecs
	tel.mu.Unlock()

	if busyToneSecs <= 0 {
		return
	}

	tel.pipelineLock.Lock()
	hasAudioOutput := tel.audioOutput != nil
	hasReceiveMixer := tel.receiveMixer != nil
	hasDialTone := tel.dialTone != nil
	tel.pipelineLock.Unlock()

	if !hasAudioOutput || !hasReceiveMixer || !hasDialTone {
		tel.ResetDiallingPipelines()
	}

	playBusyTone(busyToneSecs, tel)
}

// playBusyTone is the internal busy tone playback loop, separated for
// testability. The window is 0.5s with mute for 0.25s then enable for 0.25s.
func playBusyTone(duration float64, tel *Telephone) {
	window := 0.5
	started := now()
	for elapsed(started, duration) {
		e := elapsedMod(started, window)
		if e > 0.25 {
			tel.EnableDialTone()
		} else {
			tel.MuteDialTone()
		}
		sleep(5 * time.Millisecond)
	}
	sleep(500 * time.Millisecond)
}

// ReconfigureTransmitPipeline recreates the transmit pipeline mid-call when
// switching audio profiles, matching the Python __reconfigure_transmit_pipeline
// method. It stops the existing transmit components, creates new ones with the
// current profile's codec and frame time, and restarts them.
func (tel *Telephone) ReconfigureTransmitPipeline() {
	tel.mu.Lock()
	isEstablished := tel.state == StateEstablished
	micDevice := tel.micDevice
	transmitGain := tel.transmitGain
	transmitMuted := tel.transmitMuted
	transmitCodec := tel.transmitCodec
	tel.mu.Unlock()

	tel.pipelineLock.Lock()
	hasTransmitPipeline := tel.transmitPipeline != nil
	targetFrameMs := tel.targetFrameTimeMs
	pktz := tel.packetizer
	filterChain := tel.filters
	tel.pipelineLock.Unlock()

	if !hasTransmitPipeline || !isEstablished {
		return
	}

	tel.pipelineLock.Lock()
	if tel.audioInput != nil {
		_ = tel.audioInput.Stop()
	}
	if tel.transmitMixer != nil {
		_ = tel.transmitMixer.Stop()
	}
	if tel.transmitPipeline != nil {
		_ = tel.transmitPipeline.Stop()
	}

	tel.transmitMixer = mixer.NewMixer(targetFrameMs, 0, nil, nil, transmitGain)

	tel.audioInput = sources.NewLineSource(
		micDevice, targetFrameMs,
		codecs.NullCodec{},
		tel.transmitMixer,
		filterChain,
		transmitGain, 0.0, 0.075,
	)

	if pktz != nil {
		var err error
		tel.transmitPipeline, err = pipeline.NewPipeline(
			tel.transmitMixer, transmitCodec, pktz,
		)
		if err != nil {
			log.Printf("ReconfigureTransmitPipeline: NewPipeline failed: %v", err)
		}
	}

	if transmitMuted {
		tel.transmitMixer.Mute()
	}
	_ = tel.transmitMixer.Start()
	_ = tel.audioInput.Start()
	if tel.transmitPipeline != nil {
		_ = tel.transmitPipeline.Start()
	}
	tel.pipelineLock.Unlock()
}

// OpenPipelines sets up the audio pipelines for an established call,
// matching the Python __open_pipelines method. It creates the transmit
// mixer, audio input, transmit pipeline, receive link source, and
// signals ESTABLISHED. It can be called in either Established or
// Connecting state — the caller side opens pipelines when it receives
// CONNECTING (state=Connecting), while the responder side opens them
// in Answer() (state=Established).
func (tel *Telephone) OpenPipelines() {
	tel.pipelineLock.Lock()

	tel.mu.Lock()
	state := tel.state
	micDevice := tel.micDevice
	transmitGain := tel.transmitGain
	transmitCodec := tel.transmitCodec
	targetFrameMs := tel.targetFrameTimeMs
	useAGC := tel.useAGC
	tel.mu.Unlock()

	log.Printf("OpenPipelines: state=%v, micDevice=%v, transmitCodec=%T, targetFrameMs=%v, useAGC=%v, packetizer=%v",
		state, micDevice, transmitCodec, targetFrameMs, useAGC, tel.packetizer != nil)

	if state != StateEstablished && state != StateConnecting {
		log.Printf("OpenPipelines: state=%v is not Established or Connecting, returning early", state)
		tel.pipelineLock.Unlock()
		return
	}

	pktz := tel.packetizer

	tel.prepareDiallingPipelinesLocked()

	if useAGC {
		tel.filters = []filters.Filter{
			filters.NewBandPass(250, 8500),
			filters.NewAGC(-15.0, 12.0, 0.0001, 0.002, 0.001),
		}
	} else {
		tel.filters = []filters.Filter{
			filters.NewBandPass(250, 8500),
		}
	}

	tel.transmitMixer = mixer.NewMixer(targetFrameMs, 0, nil, nil, transmitGain)

	tel.audioInput = sources.NewLineSource(
		micDevice, targetFrameMs,
		codecs.NullCodec{},
		tel.transmitMixer,
		tel.filters,
		transmitGain, 0.225, 0.075,
	)

	if pktz != nil {
		var err error
		tel.transmitPipeline, err = pipeline.NewPipeline(
			tel.transmitMixer, transmitCodec, pktz,
		)
		if err != nil {
			log.Printf("OpenPipelines: NewPipeline failed: %v", err)
		}
	}
	tel.pipelineLock.Unlock()
}

// StartPipelines starts all audio pipelines for an active call,
// matching the Python __start_pipelines method.
func (tel *Telephone) StartPipelines() {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()

	log.Printf("StartPipelines: receiveMixer=%v, transmitMixer=%v, audioInput=%v, transmitPipeline=%v, receivePipeline=%v, audioOutput=%v, packetizer=%v",
		tel.receiveMixer != nil, tel.transmitMixer != nil, tel.audioInput != nil,
		tel.transmitPipeline != nil, tel.receivePipeline != nil, tel.audioOutput != nil,
		tel.packetizer != nil)

	if tel.receiveMixer != nil {
		_ = tel.receiveMixer.Start()
	}
	if tel.transmitMixer != nil {
		_ = tel.transmitMixer.Start()
	}
	if tel.audioInput != nil {
		if err := tel.audioInput.Start(); err != nil {
			log.Printf("StartPipelines: audioInput.Start failed: %v", err)
		}
	}
	if tel.transmitPipeline != nil {
		_ = tel.transmitPipeline.Start()
	}
}

// StopPipelines stops all audio pipelines,
// matching the Python __stop_pipelines method.
func (tel *Telephone) StopPipelines() {
	tel.pipelineLock.Lock()
	defer tel.pipelineLock.Unlock()

	if tel.receiveMixer != nil {
		_ = tel.receiveMixer.Stop()
	}
	if tel.transmitMixer != nil {
		_ = tel.transmitMixer.Stop()
	}
	if tel.audioInput != nil {
		_ = tel.audioInput.Stop()
	}
	if tel.receivePipeline != nil {
		_ = tel.receivePipeline.Stop()
	}
	if tel.transmitPipeline != nil {
		_ = tel.transmitPipeline.Stop()
	}
}

// SetRingingCallback sets the callback invoked when an incoming call is ringing.
func (tel *Telephone) SetRingingCallback(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.ringingCallback = fn
}

// SetEstablishedCallback sets the callback invoked when a call is established.
func (tel *Telephone) SetEstablishedCallback(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.establishedCallback = fn
}

// SetEndedCallback sets the callback invoked when a call ends.
func (tel *Telephone) SetEndedCallback(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.endedCallback = fn
}

// SetBusyCallback sets the callback invoked when the remote is busy.
func (tel *Telephone) SetBusyCallback(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.busyCallback = fn
}

// SetRejectedCallback sets the callback invoked when the remote rejects the call.
func (tel *Telephone) SetRejectedCallback(fn func()) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.rejectedCallback = fn
}

// SignallingReceived processes incoming signalling codes from a remote peer,
// matching the Python signalling_received method. It handles state transitions
// for BUSY, REJECTED, AVAILABLE, RINGING, CONNECTING, ESTABLISHED, and
// PREFERRED_PROFILE signals, triggering pipeline operations and callbacks.
// The signalFunc is used to send PREFERRED_PROFILE back to the remote when
// RINGING is received (matching Python line 708).
func (tel *Telephone) SignallingReceived(signals []int, signalFunc func(int) error) {
	for _, signal := range signals {
		switch {
		case signal == SignallingBusy:
			tel.mu.Lock()
			tel.state = StateBusy
			cb := tel.busyCallback
			tel.mu.Unlock()

			tel.DisableDialTone()
			tel.PlayBusyTone()
			tel.Hangup()
			if cb != nil {
				cb()
			}

		case signal == SignallingRejected:
			tel.mu.Lock()
			tel.state = StateRejected
			cb := tel.rejectedCallback
			tel.mu.Unlock()

			tel.DisableDialTone()
			tel.PlayBusyTone()
			tel.Hangup()
			if cb != nil {
				cb()
			}

		case signal == SignallingAvailable:
			tel.mu.Lock()
			tel.state = StateIdle
			tel.mu.Unlock()

		case signal == SignallingRinging:
			tel.mu.Lock()
			tel.state = StateRinging
			isOutgoing := !tel.incoming
			profile := tel.currentProfile
			cb := tel.ringingCallback
			tel.mu.Unlock()

			tel.prepareDiallingPipelinesLocked()
			if isOutgoing {
				tel.ActivateDialTone()
				if signalFunc != nil {
					_ = signalFunc(SignallingPreferredProfile + int(profile))
				}
			}
			if cb != nil {
				cb()
			}

		case signal == SignallingConnecting:
			tel.mu.Lock()
			tel.state = StateConnecting
			tel.mu.Unlock()

			tel.ResetDiallingPipelines()
			tel.OpenPipelines()

		case signal == SignallingEstablished:
			tel.mu.Lock()
			tel.state = StateEstablished
			isOutgoing := !tel.incoming
			ll := tel.lowLatency
			cb := tel.establishedCallback
			tel.mu.Unlock()

			if isOutgoing {
				tel.StartPipelines()
				tel.DisableDialTone()
			}
			tel.pipelineLock.Lock()
			ao := tel.audioOutput
			tel.pipelineLock.Unlock()
			if ll && ao != nil {
				ao.EnableLowLatency()
			}
			if cb != nil {
				cb()
			}

		case signal >= SignallingPreferredProfile:
			profile := byte(signal - SignallingPreferredProfile)
			tel.mu.Lock()
			isEstablished := tel.state == StateEstablished
			tel.mu.Unlock()

			if isEstablished {
				tel.SwitchProfile(profile, signalFunc)
			} else {
				tel.pipelineLock.Lock()
				tel.selectCallProfile(profile)
				tel.pipelineLock.Unlock()
			}
		}
	}
}

// SwitchProfile switches the audio profile during an active call,
// matching the Python switch_profile method. It reconfigures the
// transmit pipeline with the new codec and frame time. If signalFunc
// is non-nil, it sends PREFERRED_PROFILE+profile to the remote peer
// (matching Python line 490).
func (tel *Telephone) SwitchProfile(profile byte, signalFunc func(int) error) {
	tel.mu.Lock()
	if tel.state != StateEstablished {
		tel.mu.Unlock()
		return
	}
	if tel.currentProfile == profile {
		tel.mu.Unlock()
		return
	}
	tel.currentProfile = profile
	tel.mu.Unlock()

	if signalFunc != nil {
		_ = signalFunc(SignallingPreferredProfile + int(profile))
	}

	tel.pipelineLock.Lock()
	tel.selectCallCodecs(profile)
	tel.selectCallFrameTime(profile)
	tel.pipelineLock.Unlock()

	tel.ReconfigureTransmitPipeline()
}

// DialToneActive reports whether the dial tone is currently active.
func (tel *Telephone) DialToneActive() bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	return tel.dialToneActive
}

// SetDialToneActive sets the dial tone active state.
func (tel *Telephone) SetDialToneActive(active bool) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.dialToneActive = active
}

// BlockedList returns the list of blocked caller identity hashes.
func (tel *Telephone) BlockedList() []string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	result := make([]string, len(tel.blockedList))
	copy(result, tel.blockedList)
	return result
}

// SetBlockedList sets the list of blocked caller identity hashes.
// Blocked callers are rejected even if they would otherwise be allowed.
func (tel *Telephone) SetBlockedList(list []string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.blockedList = list
}

// AllowList returns the list of explicitly allowed caller identity hashes.
func (tel *Telephone) AllowList() []string {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	result := make([]string, len(tel.allowList))
	copy(result, tel.allowList)
	return result
}

// SetAllowList sets the list of explicitly allowed caller identity hashes.
// This is only effective when the allowed policy is not AllowAll or AllowNone.
func (tel *Telephone) SetAllowList(list []string) {
	tel.mu.Lock()
	defer tel.mu.Unlock()
	tel.allowList = list
}

// IsCallerAllowed reports whether a caller with the given identity hash is
// permitted to establish a call. It checks the blocked list first, then
// applies the allowed policy (AllowAll, AllowNone, or explicit allow list).
func (tel *Telephone) IsCallerAllowed(identityHash string) bool {
	tel.mu.Lock()
	defer tel.mu.Unlock()

	for _, blocked := range tel.blockedList {
		if blocked == identityHash {
			return false
		}
	}

	switch tel.allowed {
	case AllowAll:
		return true
	case AllowNone:
		if len(tel.allowList) == 0 {
			return false
		}
		for _, allowed := range tel.allowList {
			if allowed == identityHash {
				return true
			}
		}
		return false
	default:
		for _, allowed := range tel.allowList {
			if allowed == identityHash {
				return true
			}
		}
		return false
	}
}

// IncomingLinkEstablished handles an incoming RNS link establishment.
// If the line is busy (active call or externally busy), it signals BUSY
// and calls teardownFunc. Otherwise it marks the call as incoming and
// signals AVAILABLE to the remote peer.
func (tel *Telephone) IncomingLinkEstablished(signalFunc func(int) error, teardownFunc func()) {
	tel.mu.Lock()
	if tel.state != StateIdle || tel.externalBusy {
		tel.mu.Unlock()
		_ = signalFunc(SignallingBusy)
		if teardownFunc != nil {
			teardownFunc()
		}
		return
	}

	tel.incoming = true
	tel.mu.Unlock()

	_ = signalFunc(SignallingAvailable)
}

// CallerIdentified handles a caller whose identity has been verified.
// If the line is busy, it signals BUSY and calls teardownFunc.
// If the caller is not allowed, it signals BUSY and calls teardownFunc.
// Otherwise it transitions to Ringing state, resets dialling pipelines,
// signals RINGING, and activates the ringtone. It returns true if the
// caller is accepted (ringing), or false if rejected (busy/not allowed).
func (tel *Telephone) CallerIdentified(identityHash string, signalFunc func(int) error, teardownFunc func()) bool {
	tel.mu.Lock()
	if tel.state != StateIdle || tel.externalBusy {
		tel.mu.Unlock()
		_ = signalFunc(SignallingBusy)
		if teardownFunc != nil {
			teardownFunc()
		}
		return false
	}

	if !tel.isCallerAllowedLocked(identityHash) {
		tel.mu.Unlock()
		_ = signalFunc(SignallingBusy)
		if teardownFunc != nil {
			teardownFunc()
		}
		return false
	}

	tel.state = StateRinging
	tel.incoming = true
	cb := tel.ringingCallback
	autoAnswerDelay := tel.autoAnswer
	autoAnswerFunc := tel.autoAnswerFunc
	tel.mu.Unlock()

	tel.ResetDiallingPipelines()
	_ = signalFunc(SignallingRinging)
	tel.ActivateRingTone()

	if cb != nil {
		cb()
	}

	if autoAnswerDelay > 0 && autoAnswerFunc != nil {
		go func() {
			time.Sleep(autoAnswerDelay)
			autoAnswerFunc()
		}()
	}

	return true
}

// isCallerAllowedLocked checks whether a caller is allowed, assuming
// the mutex is already held.
func (tel *Telephone) isCallerAllowedLocked(identityHash string) bool {
	for _, blocked := range tel.blockedList {
		if blocked == identityHash {
			return false
		}
	}

	switch tel.allowed {
	case AllowAll:
		return true
	case AllowNone:
		if len(tel.allowList) == 0 {
			return false
		}
		for _, allowed := range tel.allowList {
			if allowed == identityHash {
				return true
			}
		}
		return false
	default:
		for _, allowed := range tel.allowList {
			if allowed == identityHash {
				return true
			}
		}
		return false
	}
}
