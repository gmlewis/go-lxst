// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package telephony

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAnswer_RingingState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)

	var establishedCalled atomic.Int32
	tel.SetEstablishedCallback(func() {
		establishedCalled.Add(1)
	})

	if ok := tel.Answer(); !ok {
		t.Error("Answer should return true when in Ringing state")
	}
	if tel.State() != StateEstablished {
		t.Errorf("Expected Established state after Answer, got %v", tel.State())
	}
	if establishedCalled.Load() != 1 {
		t.Errorf("Expected established callback to be called once, got %d", establishedCalled.Load())
	}
}

func TestAnswer_NonRingingState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	if ok := tel.Answer(); ok {
		t.Error("Answer should return false when not in Ringing state")
	}
	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after failed Answer, got %v", tel.State())
	}
}

func TestAnswer_CallingState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)

	if ok := tel.Answer(); ok {
		t.Error("Answer should return false when in Calling state")
	}
}

func TestAnswer_EstablishedState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	if ok := tel.Answer(); ok {
		t.Error("Answer should return false when already Established")
	}
}

func TestHangup_FromEstablished(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)
	tel.SetProfile(ProfileQualityMedium)

	var endedCalled atomic.Int32
	tel.SetEndedCallback(func() {
		endedCalled.Add(1)
	})

	tel.Hangup()

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after Hangup, got %v", tel.State())
	}
	if endedCalled.Load() != 1 {
		t.Errorf("Expected ended callback to be called once, got %d", endedCalled.Load())
	}
}

func TestHangup_FromRinging(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)

	var endedCalled atomic.Int32
	tel.SetEndedCallback(func() {
		endedCalled.Add(1)
	})

	tel.Hangup()

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after Hangup, got %v", tel.State())
	}
	if endedCalled.Load() != 1 {
		t.Errorf("Expected ended callback to be called once, got %d", endedCalled.Load())
	}
}

func TestHangup_WithBusyReason(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)

	var busyCalled atomic.Int32
	tel.SetBusyCallback(func() {
		busyCalled.Add(1)
	})

	tel.HangupWithReason(SignallingBusy)

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after HangupWithReason, got %v", tel.State())
	}
	if busyCalled.Load() != 1 {
		t.Errorf("Expected busy callback to be called once, got %d", busyCalled.Load())
	}
}

func TestHangup_WithRejectedReason(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateRinging)
	tel.SetIncoming(true)

	var rejectedCalled atomic.Int32
	tel.SetRejectedCallback(func() {
		rejectedCalled.Add(1)
	})

	tel.HangupWithReason(SignallingRejected)

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after HangupWithReason(REJECTED), got %v", tel.State())
	}
	if rejectedCalled.Load() != 1 {
		t.Errorf("Expected rejected callback to be called once, got %d", rejectedCalled.Load())
	}
}

func TestHangup_WithNoReason(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	var endedCalled atomic.Int32
	tel.SetEndedCallback(func() {
		endedCalled.Add(1)
	})

	tel.HangupWithReason(0)

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after HangupWithReason(0), got %v", tel.State())
	}
	if endedCalled.Load() != 1 {
		t.Errorf("Expected ended callback to be called once, got %d", endedCalled.Load())
	}
}

func TestHangup_IdleState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// Hangup from idle should not panic
	tel.Hangup()

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state, got %v", tel.State())
	}
}

func TestSignal_UpdatesStateForAutoStatusCodes(t *testing.T) {
	tests := []struct {
		signal byte
		want   TelephoneState
	}{
		{SignallingCalling, StateCalling},
		{SignallingAvailable, StateIdle},
		{SignallingRinging, StateRinging},
		{SignallingConnecting, StateConnecting},
		{SignallingEstablished, StateEstablished},
	}

	for _, tt := range tests {
		tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
		tel.Signal(tt.signal, nil, true)
		if tel.State() != tt.want {
			t.Errorf("Signal(0x%02x): expected state %v, got %v", tt.signal, tt.want, tel.State())
		}
	}
}

func TestSignal_DoesNotUpdateStateForNonAutoCodes(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.Signal(SignallingBusy, nil, true)
	// BUSY is not an auto status code, so state should stay Idle
	if tel.State() != StateIdle {
		t.Errorf("Signal(BUSY): expected state Idle, got %v", tel.State())
	}
}

func TestSetConnectTimeout(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	tel.SetConnectTimeout(10)
	if tel.ConnectTimeout() != 10 {
		t.Errorf("Expected ConnectTimeout 10, got %d", tel.ConnectTimeout())
	}
}

func TestSetAnnounceInterval(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	tel.SetAnnounceInterval(3600)
	if tel.AnnounceInterval() != 3600 {
		t.Errorf("Expected AnnounceInterval 3600, got %d", tel.AnnounceInterval())
	}
}

func TestSetAnnounceInterval_Minimum(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// Setting below minimum should clamp to minimum
	tel.SetAnnounceInterval(100)
	if tel.AnnounceInterval() < AnnounceIntervalMin {
		t.Errorf("AnnounceInterval should be at least %d, got %d", AnnounceIntervalMin, tel.AnnounceInterval())
	}
}

func TestSetAllowed_AllowAll(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowNone, 0.0, 0.0)
	tel.SetAllowed(AllowAll)
	if tel.IsCallerAllowed("any_hash") != true {
		t.Error("AllowAll should allow any caller")
	}
}

func TestSetAllowed_AllowNone(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetAllowed(AllowNone)
	if tel.IsCallerAllowed("any_hash") != false {
		t.Error("AllowNone should reject all callers")
	}
}

func TestOutgoingLinkEstablished(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	_ = func(code byte) error { return nil }

	tel.OutgoingLinkEstablished(nil)

	// Outgoing link established should transition to Ringing state
	// and prepare the signalling handler
	if tel.State() != StateRinging {
		t.Errorf("Expected Ringing state after OutgoingLinkEstablished, got %v", tel.State())
	}
}

func TestOutgoingLinkEstablished_NotIdle(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	signalFunc := func(code byte) error { return nil }

	tel.OutgoingLinkEstablished(signalFunc)

	// Should not change state if already in a call
	if tel.State() != StateEstablished {
		t.Errorf("Expected Established state (unchanged), got %v", tel.State())
	}
}

func TestLinkClosed(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	var endedCalled atomic.Int32
	tel.SetEndedCallback(func() {
		endedCalled.Add(1)
	})

	tel.LinkClosed()

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after LinkClosed, got %v", tel.State())
	}
	if endedCalled.Load() != 1 {
		t.Errorf("Expected ended callback to be called once, got %d", endedCalled.Load())
	}
}

func TestPacketizerFailure(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	tel.SetState(StateEstablished)

	var endedCalled atomic.Int32
	tel.SetEndedCallback(func() {
		endedCalled.Add(1)
	})

	tel.PacketizerFailure()

	if tel.State() != StateIdle {
		t.Errorf("Expected Idle state after PacketizerFailure, got %v", tel.State())
	}
	if endedCalled.Load() != 1 {
		t.Errorf("Expected ended callback to be called once, got %d", endedCalled.Load())
	}
}

func TestStartRingTimeout_NotRinging(t *testing.T) {
	origSleep := sleep
	defer func() { sleep = origSleep }()

	var sleepCalls []time.Duration
	sleep = func(d time.Duration) { sleepCalls = append(sleepCalls, d) }

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	// Not in Ringing state, so goroutine should exit immediately
	tel.SetState(StateCalling)

	tel.StartRingTimeout()

	// Give the goroutine a moment
	sleep(10 * time.Millisecond)

	// State should remain unchanged
	if tel.State() != StateCalling {
		t.Errorf("Expected Calling state, got %v", tel.State())
	}
}

func TestStartEstablishmentTimeout_AlreadyRinging(t *testing.T) {
	origSleep := sleep
	defer func() { sleep = origSleep }()

	sleep = func(d time.Duration) {}

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)
	// Already in Ringing state, so establishment timeout should exit
	tel.SetState(StateRinging)

	tel.StartEstablishmentTimeout()

	// Give the goroutine a moment
	sleep(50 * time.Millisecond)

	// State should remain Ringing
	if tel.State() != StateRinging {
		t.Errorf("Expected Ringing state, got %v", tel.State())
	}
}
