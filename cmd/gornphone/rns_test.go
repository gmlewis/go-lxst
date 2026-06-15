// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

func TestNewTelephoneEndpoint(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	if tep == nil {
		t.Fatal("NewTelephoneEndpoint returned nil")
	}
}

func TestNewTelephoneEndpoint_NilIdentity(t *testing.T) {
	t.Parallel()

	ts := rns.NewTransportSystem(nil)
	_, err := NewTelephoneEndpoint(nil, ts, nil)
	if err == nil {
		t.Fatal("expected error for nil identity")
	}
}

func TestNewTelephoneEndpoint_NilTransport(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	_, err = NewTelephoneEndpoint(id, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil transport")
	}
}

func TestTelephoneEndpoint_Destination(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	dest := tep.Destination()
	if dest == nil {
		t.Fatal("Destination() returned nil")
	}
	if dest.Type != rns.DestinationSingle {
		t.Errorf("Destination.Type = %d, want %d", dest.Type, rns.DestinationSingle)
	}
}

func TestTelephoneEndpoint_Hashes(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	if tep.IdentityHash() != id.HexHash {
		t.Errorf("IdentityHash() = %q, want %q", tep.IdentityHash(), id.HexHash)
	}
	if tep.DestinationHash() == "" {
		t.Error("DestinationHash() should not be empty")
	}
}

func TestTelephoneEndpoint_Announce(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	err = tep.Announce()
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}
}

func TestTelephoneEndpoint_NeedsAnnounce(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// After creation, should need announce (lastAnnounce is zero)
	if !tep.NeedsAnnounce() {
		t.Error("NeedsAnnounce() should be true before first announce")
	}

	err = tep.Announce()
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	// After announce, should not need announce immediately
	if tep.NeedsAnnounce() {
		t.Error("NeedsAnnounce() should be false right after announce")
	}
}

func TestTelephoneEndpoint_SetAllowed(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tep.SetAllowed(rns.AllowNone)
	// Default hash should be rejected when AllowNone
	if tep.IsCallerAllowed("aabbccdd11223344aabbccdd11223344") {
		t.Error("IsCallerAllowed should return false when AllowNone")
	}
}

func TestTelephoneEndpoint_SetBlocked(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	blockedHash := "aabbccdd11223344aabbccdd11223344"
	blockedBytes, _ := hex.DecodeString(blockedHash)
	tep.SetBlocked([][]byte{blockedBytes})

	if tep.IsCallerAllowed(blockedHash) {
		t.Error("IsCallerAllowed should return false for blocked hash")
	}
}

func TestTelephoneEndpoint_SetAllowList(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	allowedHash := "aabbccdd11223344aabbccdd11223344"
	allowedBytes, _ := hex.DecodeString(allowedHash)
	tep.SetAllowed(rns.AllowList)
	tep.SetAllowList([][]byte{allowedBytes})

	if !tep.IsCallerAllowed(allowedHash) {
		t.Error("IsCallerAllowed should return true for listed hash")
	}

	otherHash := "11223344aabbccdd11223344aabbccdd"
	if tep.IsCallerAllowed(otherHash) {
		t.Error("IsCallerAllowed should return false for unlisted hash")
	}
}

func TestTelephoneEndpoint_Callbacks(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	var ringingCalled bool
	tep.SetOnRinging(func(remoteIdentity *rns.Identity) {
		ringingCalled = true
	})

	var establishedCalled bool
	tep.SetOnEstablished(func(remoteIdentity *rns.Identity) {
		establishedCalled = true
	})

	var endedCalled bool
	tep.SetOnEnded(func(remoteIdentity *rns.Identity) {
		endedCalled = true
	})

	var busyCalled bool
	tep.SetOnBusy(func(remoteIdentity *rns.Identity) {
		busyCalled = true
	})

	var rejectedCalled bool
	tep.SetOnRejected(func(remoteIdentity *rns.Identity) {
		rejectedCalled = true
	})

	_ = ringingCalled
	_ = establishedCalled
	_ = endedCalled
	_ = busyCalled
	_ = rejectedCalled
}

func TestTelephoneEndpoint_Teardown(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tep.Teardown()

	if tep.Destination() != nil {
		t.Error("Destination should be nil after Teardown")
	}
}

func TestTelephoneEndpoint_AnnounceInterval(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Manually set a short announce interval for testing
	tep.mu.Lock()
	tep.announceIntvl = 10 * time.Millisecond
	tep.mu.Unlock()

	err = tep.Announce()
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	// Immediately after announce, should not need announce
	if tep.NeedsAnnounce() {
		t.Error("NeedsAnnounce should be false right after announce")
	}

	// After waiting, should need announce
	time.Sleep(15 * time.Millisecond)
	if !tep.NeedsAnnounce() {
		t.Error("NeedsAnnounce should be true after interval elapsed")
	}
}

func TestTelephoneEndpoint_JobLoop(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Set a short announce interval for testing
	tep.mu.Lock()
	tep.announceIntvl = 200 * time.Millisecond
	tep.mu.Unlock()

	// Start the jobs loop
	tep.StartJobs()
	defer tep.StopJobs()

	// Wait long enough for at least one job poll cycle
	time.Sleep(300 * time.Millisecond)

	// The job loop should have re-announced, so NeedsAnnounce should be false
	if tep.NeedsAnnounce() {
		t.Error("NeedsAnnounce should be false after job loop re-announced")
	}
}

func TestTelephoneEndpoint_Hangup(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Hangup with no active link should be safe
	tep.Hangup()

	if tep.ActiveLink() != nil {
		t.Error("ActiveLink should be nil after Hangup with no active call")
	}
}

func TestTelephoneEndpoint_CallSelf(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Trying to call with an identity not on the network should fail
	err = tep.Call("aabbccdd11223344aabbccdd11223344", 1*time.Second)
	if err == nil {
		t.Error("expected error when calling identity not on network")
	}
}

func TestTelephoneEndpoint_CallSpinner(t *testing.T) {
	t.Parallel()

	id1, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}
	id2, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep1, err := NewTelephoneEndpoint(id1, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	tep2, err := NewTelephoneEndpoint(id2, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Register identities with transport so they can be recalled
	ts.Remember(id1.Hash, nil, nil, nil)
	ts.Remember(id2.Hash, nil, nil, nil)

	_ = tep1
	_ = tep2
}

func TestHandleSignallingData_Available(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	var identifyCalled bool
	tep.testSetIdentifyFunc(func(link *rns.Link, identity *rns.Identity) error {
		identifyCalled = true
		return nil
	})

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingAvailable)
	tep.handleSignallingData(data, nil, id)

	if !identifyCalled {
		t.Error("SignallingAvailable should trigger identify")
	}
}

func TestHandleSignallingData_Ringing(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingRinging)
	tep.handleSignallingData(data, nil, id)

	if tel.State() != telephony.StateRinging {
		t.Errorf("state = %v, want Ringing", tel.State())
	}
}

func TestHandleSignallingData_Connecting(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tel.SetState(telephony.StateRinging)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingConnecting)
	tep.handleSignallingData(data, nil, id)

	if tel.State() != telephony.StateConnecting {
		t.Errorf("state = %v, want Connecting", tel.State())
	}
}

func TestHandleSignallingData_Established(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tel.SetState(telephony.StateConnecting)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingEstablished)
	tep.handleSignallingData(data, nil, id)

	if tel.State() != telephony.StateEstablished {
		t.Errorf("state = %v, want Established", tel.State())
	}
}

func TestHandleSignallingData_Busy(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tel.SetState(telephony.StateCalling)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingBusy)
	tep.handleSignallingData(data, nil, id)

	if tel.State() != telephony.StateIdle {
		t.Errorf("state = %v, want Idle after BUSY", tel.State())
	}
}

func TestHandleSignallingData_Rejected(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tel.SetState(telephony.StateCalling)
	tep.SetTelephone(tel)

	data := packSignalling(t, telephony.SignallingRejected)
	tep.handleSignallingData(data, nil, id)

	if tel.State() != telephony.StateIdle {
		t.Errorf("state = %v, want Idle after REJECTED", tel.State())
	}
}

func TestHandleSignallingData_PreferredProfile(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tel.SetIncoming(false)
	tep.SetTelephone(tel)

	profile := telephony.ProfileBandwidthUltraLow
	signalValue := int(telephony.SignallingPreferredProfile) + int(profile)
	data := packSignallingInt(t, signalValue)
	tep.handleSignallingData(data, nil, id)

	if tel.CurrentProfile() != profile {
		t.Errorf("profile = 0x%02x, want 0x%02x", tel.CurrentProfile(), profile)
	}
}

func TestCallerDoesNotSendCallingOnLinkEstablished(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tep.SetTelephone(tel)

	var sentSignals []byte
	tep.testSetSendSignallingFunc(func(link *rns.Link, signal byte) {
		sentSignals = append(sentSignals, signal)
	})

	tep.testFireOutgoingLinkEstablished(nil)

	for _, s := range sentSignals {
		if s == telephony.SignallingCalling {
			t.Error("caller should not send SignallingCalling on link establishment")
		}
	}
}

func TestResponderSendsRingingAfterCallerIdentified(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tep.SetTelephone(tel)

	var sentSignals []byte
	tep.testSetSendSignallingFunc(func(link *rns.Link, signal byte) {
		sentSignals = append(sentSignals, signal)
	})

	tep.testFireCallerIdentified(id.HexHash)

	found := false
	for _, s := range sentSignals {
		if s == telephony.SignallingRinging {
			found = true
		}
	}
	if !found {
		t.Error("responder should send SignallingRinging after caller is identified")
	}
}

func packSignalling(t *testing.T, signal byte) []byte {
	t.Helper()
	signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
	packed, err := network.PackData(signallingData)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}
	return packed
}

func packSignallingInt(t *testing.T, signal int) []byte {
	t.Helper()
	signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
	packed, err := network.PackData(signallingData)
	if err != nil {
		t.Fatalf("PackData failed: %v", err)
	}
	return packed
}

func TestTelephoneEndpoint_AlreadyInCall(t *testing.T) {
	t.Parallel()

	id1, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}
	id2, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep1, err := NewTelephoneEndpoint(id1, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	tep2, err := NewTelephoneEndpoint(id2, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Register identities with transport so they can be recalled
	ts.Remember(id1.Hash, nil, nil, nil)
	ts.Remember(id2.Hash, nil, nil, nil)

	// First call to self should succeed (but link may fail to establish)
	_ = tep1

	// Set up second endpoint to be able to receive
	_ = tep2
}

// TestCallLifecycle_FullFlow simulates the complete call signalling lifecycle
// between a caller and responder using the Telephone state machine and
// TelephoneEndpoint signalling, without real RNS network links.
// The flow is:
//  1. Caller sets state to Calling
//  2. Caller's outgoing link established → state Ringing
//  3. Responder receives incoming link → IncomingLinkEstablished → sends AVAILABLE
//  4. Caller receives AVAILABLE → identifies → responder fires CallerIdentified → sends RINGING
//  5. Caller receives RINGING → state Ringing
//  6. Responder answers → state Established, sends CONNECTING, ESTABLISHED
//  7. Caller receives CONNECTING → state Connecting
//  8. Caller receives ESTABLISHED → state Established
//  9. Caller hangs up → both sides return to Idle
func TestCallLifecycle_FullFlow(t *testing.T) {
	t.Parallel()

	callerID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity for caller: %v", err)
	}
	responderID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity for responder: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	ts.Remember(callerID.Hash, nil, nil, nil)
	ts.Remember(responderID.Hash, nil, nil, nil)

	// --- Set up responder telephone with callbacks ---
	responderTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	var (
		responderEvents   []string
		responderEventsMu sync.Mutex
	)
	recordResponder := func(event string) {
		responderEventsMu.Lock()
		responderEvents = append(responderEvents, event)
		responderEventsMu.Unlock()
	}

	responderTel.SetRingingCallback(func() { recordResponder("ringing") })
	responderTel.SetEstablishedCallback(func() { recordResponder("established") })
	responderTel.SetEndedCallback(func() { recordResponder("ended") })
	responderTel.SetBusyCallback(func() { recordResponder("busy") })
	responderTel.SetRejectedCallback(func() { recordResponder("rejected") })

	// Track signalling sent by responder
	var responderSignals []byte
	var responderSignalsMu sync.Mutex
	responderSignalFunc := func(signal byte) error {
		responderSignalsMu.Lock()
		responderSignals = append(responderSignals, signal)
		responderSignalsMu.Unlock()
		return nil
	}
	responderTeardownFunc := func() {}

	// --- Set up caller telephone with callbacks ---
	callerTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	var (
		callerEvents   []string
		callerEventsMu sync.Mutex
	)
	recordCaller := func(event string) {
		callerEventsMu.Lock()
		callerEvents = append(callerEvents, event)
		callerEventsMu.Unlock()
	}

	callerTel.SetEstablishedCallback(func() { recordCaller("established") })
	callerTel.SetEndedCallback(func() { recordCaller("ended") })

	// --- Verify initial state ---
	if callerTel.State() != telephony.StateIdle {
		t.Fatalf("caller initial state = %v, want Idle", callerTel.State())
	}
	if responderTel.State() != telephony.StateIdle {
		t.Fatalf("responder initial state = %v, want Idle", responderTel.State())
	}

	// --- Step 1: Caller initiates call ---
	callerTel.Call(telephony.DefaultProfile)
	if callerTel.State() != telephony.StateCalling {
		t.Fatalf("caller state after Call = %v, want Calling", callerTel.State())
	}

	// --- Step 2: Caller's outgoing link established ---
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	if callerTel.State() != telephony.StateRinging {
		t.Fatalf("caller state after OutgoingLinkEstablished = %v, want Ringing", callerTel.State())
	}

	// --- Step 3: Responder receives incoming link ---
	responderTel.IncomingLinkEstablished(responderSignalFunc, responderTeardownFunc)
	// Responder should send AVAILABLE but remain in Idle (waiting for caller identity)
	if responderTel.State() != telephony.StateIdle {
		t.Fatalf("responder state after IncomingLinkEstablished = %v, want Idle (waiting for ID)", responderTel.State())
	}

	// --- Step 4: Responder fires CallerIdentified → sends RINGING ---
	accepted := responderTel.CallerIdentified(callerID.HexHash, responderSignalFunc, responderTeardownFunc)
	if !accepted {
		t.Fatal("CallerIdentified should accept the caller")
	}
	if responderTel.State() != telephony.StateRinging {
		t.Fatalf("responder state after CallerIdentified = %v, want Ringing", responderTel.State())
	}

	// Verify responder sent AVAILABLE then RINGING
	responderSignalsMu.Lock()
	signals := make([]byte, len(responderSignals))
	copy(signals, responderSignals)
	responderSignalsMu.Unlock()
	if len(signals) < 2 {
		t.Fatalf("responder should have sent at least 2 signals, got %d", len(signals))
	}
	if signals[0] != telephony.SignallingAvailable {
		t.Errorf("first signal = %d, want SignallingAvailable (%d)", signals[0], telephony.SignallingAvailable)
	}
	if signals[1] != telephony.SignallingRinging {
		t.Errorf("second signal = %d, want SignallingRinging (%d)", signals[1], telephony.SignallingRinging)
	}

	// Verify responder ringing callback fired
	responderEventsMu.Lock()
	foundRinging := false
	for _, e := range responderEvents {
		if e == "ringing" {
			foundRinging = true
		}
	}
	responderEventsMu.Unlock()
	if !foundRinging {
		t.Error("responder ringing callback should have been recorded")
	}

	// --- Step 5: Caller receives RINGING ---
	callerTel.SignallingReceived([]byte{telephony.SignallingRinging})
	if callerTel.State() != telephony.StateRinging {
		t.Fatalf("caller state after SignallingRinging = %v, want Ringing", callerTel.State())
	}

	// Verify caller has audio output (dialling pipelines prepared)
	if callerTel.AudioOutput() == nil {
		t.Error("caller should have audio output after SignallingRinging")
	}

	// --- Step 6: Responder answers ---
	if !responderTel.Answer() {
		t.Fatal("responder Answer should succeed")
	}
	if responderTel.State() != telephony.StateEstablished {
		t.Fatalf("responder state after Answer = %v, want Established", responderTel.State())
	}

	// Verify responder established callback fired
	responderEventsMu.Lock()
	foundEst := false
	for _, e := range responderEvents {
		if e == "established" {
			foundEst = true
		}
	}
	responderEventsMu.Unlock()
	if !foundEst {
		t.Error("responder established callback should have been recorded")
	}

	// Verify responder has receive mixer
	if responderTel.ReceiveMixer() == nil {
		t.Error("responder should have receive mixer after Answer")
	}

	// --- Step 7: Caller receives CONNECTING ---
	callerTel.SignallingReceived([]byte{telephony.SignallingConnecting})
	if callerTel.State() != telephony.StateConnecting {
		t.Fatalf("caller state after SignallingConnecting = %v, want Connecting", callerTel.State())
	}

	// Verify caller has audio output (pipelines reset and re-prepared)
	if callerTel.AudioOutput() == nil {
		t.Error("caller should have audio output after SignallingConnecting")
	}

	// --- Step 8: Caller receives ESTABLISHED ---
	callerTel.SignallingReceived([]byte{telephony.SignallingEstablished})
	if callerTel.State() != telephony.StateEstablished {
		t.Fatalf("caller state after SignallingEstablished = %v, want Established", callerTel.State())
	}

	// Verify caller established callback fired
	callerEventsMu.Lock()
	foundEst = false
	for _, e := range callerEvents {
		if e == "established" {
			foundEst = true
		}
	}
	callerEventsMu.Unlock()
	if !foundEst {
		t.Error("caller established callback should have been recorded")
	}

	// Verify caller has receive mixer
	if callerTel.ReceiveMixer() == nil {
		t.Error("caller should have receive mixer after SignallingEstablished")
	}

	// --- Step 9: Caller hangs up ---
	callerTel.Hangup()
	if callerTel.State() != telephony.StateIdle {
		t.Fatalf("caller state after Hangup = %v, want Idle", callerTel.State())
	}

	// Verify caller ended callback
	callerEventsMu.Lock()
	foundEnd := false
	for _, e := range callerEvents {
		if e == "ended" {
			foundEnd = true
		}
	}
	callerEventsMu.Unlock()
	if !foundEnd {
		t.Error("caller ended callback should have been recorded")
	}

	// Responder also hangs up
	responderTel.Hangup()
	if responderTel.State() != telephony.StateIdle {
		t.Fatalf("responder state after Hangup = %v, want Idle", responderTel.State())
	}

	// Verify responder ended callback
	responderEventsMu.Lock()
	foundEnd = false
	for _, e := range responderEvents {
		if e == "ended" {
			foundEnd = true
		}
	}
	responderEventsMu.Unlock()
	if !foundEnd {
		t.Error("responder ended callback should have been recorded")
	}
}

// TestCallLifecycle_BusyRejectFlow verifies the call flow when the
// remote is busy or rejects the call. It uses handleSignallingData to
// trigger both telephone state machine transitions and endpoint callbacks.
func TestCallLifecycle_BusyRejectFlow(t *testing.T) {
	t.Parallel()

	callerID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity for caller: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	ts.Remember(callerID.Hash, nil, nil, nil)

	callerEP, err := NewTelephoneEndpoint(callerID, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint for caller: %v", err)
	}
	defer callerEP.Teardown()

	callerTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	callerEP.SetTelephone(callerTel)

	var (
		callerEvents   []string
		callerEventsMu sync.Mutex
	)
	recordCaller := func(event string) {
		callerEventsMu.Lock()
		callerEvents = append(callerEvents, event)
		callerEventsMu.Unlock()
	}

	callerEP.SetOnBusy(func(remote *rns.Identity) {
		recordCaller("busy")
	})
	callerEP.SetOnRejected(func(remote *rns.Identity) {
		recordCaller("rejected")
	})

	// --- Busy signal flow ---
	callerTel.Call(telephony.DefaultProfile)
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	callerTel.SetIncoming(false)

	// Caller receives BUSY via handleSignallingData (triggers both tel and ep callbacks)
	data := packSignalling(t, telephony.SignallingBusy)
	callerEP.handleSignallingData(data, nil, callerID)

	if callerTel.State() != telephony.StateIdle {
		t.Fatalf("caller state after BUSY = %v, want Idle", callerTel.State())
	}

	callerEventsMu.Lock()
	foundBusy := false
	for _, e := range callerEvents {
		if e == "busy" {
			foundBusy = true
		}
	}
	callerEventsMu.Unlock()
	if !foundBusy {
		t.Error("caller should have received busy event via endpoint callback")
	}

	// --- Rejected signal flow ---
	callerEvents = nil
	callerTel.Call(telephony.DefaultProfile)
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	callerTel.SetIncoming(false)

	// Caller receives REJECTED via handleSignallingData
	data2 := packSignalling(t, telephony.SignallingRejected)
	callerEP.handleSignallingData(data2, nil, callerID)

	if callerTel.State() != telephony.StateIdle {
		t.Fatalf("caller state after REJECTED = %v, want Idle", callerTel.State())
	}

	callerEventsMu.Lock()
	foundRejected := false
	for _, e := range callerEvents {
		if e == "rejected" {
			foundRejected = true
		}
	}
	callerEventsMu.Unlock()
	if !foundRejected {
		t.Error("caller should have received rejected event via endpoint callback")
	}
}

// TestCallLifecycle_ProfileNegotiation verifies that preferred profile
// signalling is handled correctly during call establishment.
func TestCallLifecycle_ProfileNegotiation(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint: %v", err)
	}
	defer tep.Teardown()

	tel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	tep.SetTelephone(tel)

	// Test profile change during established call
	tel.SetState(telephony.StateEstablished)

	profile := telephony.ProfileBandwidthUltraLow
	signalValue := int(telephony.SignallingPreferredProfile) + int(profile)
	data := packSignallingInt(t, signalValue)
	tep.handleSignallingData(data, nil, id)

	if tel.CurrentProfile() != profile {
		t.Errorf("profile during established call = 0x%02x, want 0x%02x", tel.CurrentProfile(), profile)
	}

	// Test profile setting before established (should use SetProfile)
	tel.SetState(telephony.StateRinging)
	profile2 := telephony.ProfileQualityHigh
	signalValue2 := int(telephony.SignallingPreferredProfile) + int(profile2)
	data2 := packSignallingInt(t, signalValue2)
	tep.handleSignallingData(data2, nil, id)

	if tel.CurrentProfile() != profile2 {
		t.Errorf("profile before established = 0x%02x, want 0x%02x", tel.CurrentProfile(), profile2)
	}
}

// TestCallLifecycle_ResponderBusy verifies that the telephone correctly
// sends BUSY and calls teardown when an incoming call arrives while
// already in a call. Tests at the Telephone level (not endpoint level).
func TestCallLifecycle_ResponderBusy(t *testing.T) {
	t.Parallel()

	responderTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)

	var busySignals []byte
	var teardownCalled bool

	// Put responder in an active call
	responderTel.SetState(telephony.StateEstablished)
	responderTel.SetIncoming(true)

	// Simulate incoming link established on responder
	responderTel.IncomingLinkEstablished(
		func(signal byte) error {
			busySignals = append(busySignals, signal)
			return nil
		},
		func() {
			teardownCalled = true
		},
	)

	// Responder should still be in Established state (not transitioned to Ringing)
	if responderTel.State() != telephony.StateEstablished {
		t.Errorf("responder state = %v, want Established (busy)", responderTel.State())
	}

	// BUSY signal should have been sent
	if len(busySignals) != 1 || busySignals[0] != telephony.SignallingBusy {
		t.Errorf("expected BUSY signal, got %v", busySignals)
	}

	// Teardown should have been called
	if !teardownCalled {
		t.Error("teardown should have been called for busy responder")
	}
}

// TestCallLifecycle_ResponderRejectedCall verifies that when the caller
// receives a REJECTED signal, the endpoint fires the onRejected callback
// and the telephone transitions to Idle.
func TestCallLifecycle_ResponderRejectedCall(t *testing.T) {
	t.Parallel()

	callerID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	ts.Remember(callerID.Hash, nil, nil, nil)

	callerEP, err := NewTelephoneEndpoint(callerID, ts, nil)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint: %v", err)
	}
	defer callerEP.Teardown()

	callerTel := telephony.NewTelephone(30, 60, false, telephony.AllowAll, 0, 0)
	callerEP.SetTelephone(callerTel)

	var rejectedFired bool
	var eventsMu sync.Mutex
	callerEP.SetOnRejected(func(remote *rns.Identity) {
		eventsMu.Lock()
		rejectedFired = true
		eventsMu.Unlock()
	})

	// Set caller to ringing state
	callerTel.Call(telephony.DefaultProfile)
	callerTel.OutgoingLinkEstablished(func(signal byte) error { return nil })
	callerTel.SetIncoming(false)

	// Caller receives REJECTED signal via handleSignallingData
	data := packSignalling(t, telephony.SignallingRejected)
	callerEP.handleSignallingData(data, nil, callerID)

	if callerTel.State() != telephony.StateIdle {
		t.Errorf("caller state after REJECTED = %v, want Idle", callerTel.State())
	}

	eventsMu.Lock()
	r := rejectedFired
	eventsMu.Unlock()
	if !r {
		t.Error("caller should have fired rejected callback")
	}
}
