// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

package main

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

// TestIntegration_SignallingFlow verifies the complete signalling flow
// between two telephones using the Telephone state machine directly.
// The flow is:
//
//	Caller creates call → Caller outgoing link established →
//	Responder receives incoming link → AVAILABLE sent →
//	Caller identified → RINGING sent →
//	Caller receives RINGING → Responder answers → ESTABLISHED →
//	Caller receives ESTABLISHED → call active → hangup
func TestIntegration_SignallingFlow(t *testing.T) {
	t.Parallel()

	callerID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity for caller failed: %v", err)
	}

	receiverID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity for receiver failed: %v", err)
	}

	_ = callerID
	_ = receiverID

	// Create caller and receiver telephones
	callerTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	receiverTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	// Track state transitions via telephone callbacks
	var (
		receiverRinging     bool
		receiverEstablished bool
		receiverEnded       bool
	)

	receiverTel.SetRingingCallback(func() { receiverRinging = true })
	receiverTel.SetEstablishedCallback(func() { receiverEstablished = true })
	receiverTel.SetEndedCallback(func() { receiverEnded = true })

	var (
		callerEstablished bool
		callerEnded       bool
	)

	callerTel.SetEstablishedCallback(func() { callerEstablished = true })
	callerTel.SetEndedCallback(func() { callerEnded = true })

	// Verify initial state
	if receiverTel.State() != telephony.StateIdle {
		t.Errorf("receiver initial state = %v, want Idle", receiverTel.State())
	}
	if callerTel.State() != telephony.StateIdle {
		t.Errorf("caller initial state = %v, want Idle", callerTel.State())
	}

	// Simulate the caller initiating a call
	callerTel.Call(telephony.DefaultProfile)
	if callerTel.State() != telephony.StateCalling {
		t.Errorf("caller state after Call = %v, want Calling", callerTel.State())
	}

	// Simulate outgoing link established
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	if callerTel.State() != telephony.StateRinging {
		t.Errorf("caller state after OutgoingLinkEstablished = %v, want Ringing", callerTel.State())
	}

	// Simulate responder receiving incoming link
	receiverSignalFunc := func(signal byte) error { return nil }
	receiverTeardownFunc := func() {}

	receiverTel.IncomingLinkEstablished(receiverSignalFunc, receiverTeardownFunc)
	if receiverTel.State() != telephony.StateIdle {
		t.Errorf("receiver state after IncomingLinkEstablished = %v, want Idle (waiting for identification)", receiverTel.State())
	}

	// Simulate caller identification
	accepted := receiverTel.CallerIdentified(callerID.HexHash, receiverSignalFunc, receiverTeardownFunc)
	if !accepted {
		t.Error("CallerIdentified should accept the caller")
	}
	if receiverTel.State() != telephony.StateRinging {
		t.Errorf("receiver state after CallerIdentified = %v, want Ringing", receiverTel.State())
	}

	// Verify receiver ringing callback fired
	if !receiverRinging {
		t.Error("receiver ringing callback should have been called")
	}

	// Simulate caller receiving SignallingRinging
	callerTel.SignallingReceived([]byte{telephony.SignallingRinging})
	if callerTel.State() != telephony.StateRinging {
		t.Errorf("caller state after SignallingRinging = %v, want Ringing", callerTel.State())
	}

	// Simulate caller receiving SignallingConnecting
	callerTel.SignallingReceived([]byte{telephony.SignallingConnecting})
	if callerTel.State() != telephony.StateConnecting {
		t.Errorf("caller state after SignallingConnecting = %v, want Connecting", callerTel.State())
	}

	// Verify dialling pipelines were prepared
	if callerTel.AudioOutput() == nil {
		t.Error("caller should have audio output after SignallingConnecting")
	}

	// Simulate caller receiving SignallingEstablished
	callerTel.SignallingReceived([]byte{telephony.SignallingEstablished})
	if callerTel.State() != telephony.StateEstablished {
		t.Errorf("caller state after SignallingEstablished = %v, want Established", callerTel.State())
	}

	// Verify caller established callback was fired
	if !callerEstablished {
		t.Error("caller established callback should have been called")
	}

	// Verify both sides have receive mixers
	if callerTel.ReceiveMixer() == nil {
		t.Error("caller should have receive mixer after establishment")
	}

	// Now simulate the responder answering
	answerResult := receiverTel.Answer()
	if !answerResult {
		t.Error("Answer should succeed")
	}
	if receiverTel.State() != telephony.StateEstablished {
		t.Errorf("receiver state after Answer = %v, want Established", receiverTel.State())
	}

	// Verify receiver established callback was fired
	if !receiverEstablished {
		t.Error("receiver established callback should have been called")
	}

	// Verify receiver has receive mixer
	if receiverTel.ReceiveMixer() == nil {
		t.Error("receiver should have receive mixer after answer")
	}

	// Simulate hangup from caller side
	callerTel.Hangup()
	if callerTel.State() != telephony.StateIdle {
		t.Errorf("caller state after Hangup = %v, want Idle", callerTel.State())
	}

	// Verify caller ended callback
	if !callerEnded {
		t.Error("caller ended callback should have been called")
	}

	// Verify receiver ended callback after hangup
	receiverTel.Hangup()
	if !receiverEnded {
		t.Error("receiver ended callback should have been called")
	}
}

// TestIntegration_BusySignal verifies that a busy signal is sent
// when the receiver is already in a call, at the Telephone level.
func TestIntegration_BusySignal(t *testing.T) {
	t.Parallel()

	receiverTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	var busySignals []byte
	var teardownCalled bool

	// Set the receiver to busy state (already in a call)
	receiverTel.SetState(telephony.StateEstablished)

	// Try to signal incoming link - should be rejected with BUSY
	receiverTel.IncomingLinkEstablished(
		func(signal byte) error {
			busySignals = append(busySignals, signal)
			return nil
		},
		func() {
			teardownCalled = true
		},
	)

	// The telephone should have sent SignallingBusy via the signalFunc
	if len(busySignals) != 1 || busySignals[0] != telephony.SignallingBusy {
		t.Errorf("expected BUSY signal, got %v", busySignals)
	}

	// Teardown should have been called
	if !teardownCalled {
		t.Error("teardown should have been called when line is busy")
	}

	// State should still be Established
	if receiverTel.State() != telephony.StateEstablished {
		t.Errorf("receiver state = %v, want Established (busy)", receiverTel.State())
	}
}

// TestIntegration_RejectedSignal verifies that a rejected signal
// triggers the correct callbacks at the Telephone level.
func TestIntegration_RejectedSignal(t *testing.T) {
	t.Parallel()

	callerTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	var rejectedReceived bool

	callerTel.SetRejectedCallback(func() { rejectedReceived = true })

	// Set the caller to ringing state (outgoing call)
	callerTel.Call(telephony.DefaultProfile)
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	callerTel.SetIncoming(false)

	// Simulate receiving a rejected signal via HangupWithReason
	callerTel.HangupWithReason(telephony.SignallingRejected)

	if !rejectedReceived {
		t.Error("rejected callback should have been called")
	}

	if callerTel.State() != telephony.StateIdle {
		t.Errorf("caller state after rejected = %v, want Idle", callerTel.State())
	}
}
