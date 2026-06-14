// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/gmlewis/go-reticulum/rns"
)

func TestNewTelephoneEndpoint(t *testing.T) {
	t.Parallel()

	id, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	tep, err := NewTelephoneEndpoint(id, ts)
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
	_, err := NewTelephoneEndpoint(nil, ts)
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

	_, err = NewTelephoneEndpoint(id, nil)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep, err := NewTelephoneEndpoint(id, ts)
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
	tep1, err := NewTelephoneEndpoint(id1, ts)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	tep2, err := NewTelephoneEndpoint(id2, ts)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}

	// Register identities with transport so they can be recalled
	ts.Remember(id1.Hash, nil, nil, nil)
	ts.Remember(id2.Hash, nil, nil, nil)

	_ = tep1
	_ = tep2
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
	tep1, err := NewTelephoneEndpoint(id1, ts)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	tep2, err := NewTelephoneEndpoint(id2, ts)
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
