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

func TestIncomingLinkEstablished_IdleNotBusy(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	var signalled int
	var signalledCount int
	signalFunc := func(code int) error {
		signalled = code
		signalledCount++
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	tel.IncomingLinkEstablished(signalFunc, teardownFunc)

	if signalled != SignallingAvailable {
		t.Errorf("Expected SignallingAvailable (0x%02x), got 0x%02x", SignallingAvailable, signalled)
	}
	if signalledCount != 1 {
		t.Errorf("Expected 1 signal, got %v", signalledCount)
	}
	if teardownCalled {
		t.Error("Teardown should not be called when not busy")
	}
	if !tel.Incoming() {
		t.Error("Incoming should be true after IncomingLinkEstablished")
	}
}

func TestIncomingLinkEstablished_BusyDueToState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	tel.IncomingLinkEstablished(signalFunc, teardownFunc)

	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy (0x%02x), got 0x%02x", SignallingBusy, signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called when busy")
	}
}

func TestIncomingLinkEstablished_BusyDueToExternalBusy(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetBusy(true)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	tel.IncomingLinkEstablished(signalFunc, teardownFunc)

	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy (0x%02x), got 0x%02x", SignallingBusy, signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called when externally busy")
	}
}

func TestIncomingLinkEstablished_SignalsAvailableWhenNotBusy(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}

	tel.IncomingLinkEstablished(signalFunc, nil)

	if signalled != SignallingAvailable {
		t.Errorf("Expected SignallingAvailable, got 0x%02x", signalled)
	}
}

func TestCallerIdentified_Allowed(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	var signalled int
	var signalledCount int
	signalFunc := func(code int) error {
		signalled = code
		signalledCount++
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	var ringingCalled atomic.Int32
	tel.SetRingingCallback(func() {
		ringingCalled.Add(1)
	})

	if ok := tel.CallerIdentified("abcdef1234567890", signalFunc, teardownFunc); !ok {
		t.Error("CallerIdentified should return true for allowed caller")
	}
	if signalled != SignallingRinging {
		t.Errorf("Expected SignallingRinging (0x%02x), got 0x%02x", SignallingRinging, signalled)
	}
	if signalledCount != 1 {
		t.Errorf("Expected 1 signal, got %v", signalledCount)
	}
	if teardownCalled {
		t.Error("Teardown should not be called for allowed caller")
	}
	if tel.State() != StateRinging {
		t.Errorf("Expected Ringing state, got %v", tel.State())
	}
	if !tel.Incoming() {
		t.Error("Incoming should be true after CallerIdentified")
	}
	if ringingCalled.Load() != 1 {
		t.Errorf("Expected ringing callback to be called once, got %v", ringingCalled.Load())
	}
}

func TestCallerIdentified_BlockedList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetBlockedList([]string{"abcdef1234567890"})

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("abcdef1234567890", signalFunc, teardownFunc); ok {
		t.Error("CallerIdentified should return false for blocked caller")
	}
	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy, got 0x%02x", signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called for blocked caller")
	}
}

func TestCallerIdentified_AllowNone(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("somehash", signalFunc, teardownFunc); ok {
		t.Error("CallerIdentified should return false when AllowNone")
	}
	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy, got 0x%02x", signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called when AllowNone")
	}
}

func TestCallerIdentified_AllowList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)
	tel.SetAllowList([]string{"allowed_hash_1", "allowed_hash_2"})

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("allowed_hash_1", signalFunc, teardownFunc); !ok {
		t.Error("CallerIdentified should return true for caller in allow list")
	}
	if signalled != SignallingRinging {
		t.Errorf("Expected SignallingRinging, got 0x%02x", signalled)
	}
	if teardownCalled {
		t.Error("Teardown should not be called for allowed caller")
	}
}

func TestCallerIdentified_NotInAllowList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)
	tel.SetAllowList([]string{"allowed_hash_1"})

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("unknown_hash", signalFunc, teardownFunc); ok {
		t.Error("CallerIdentified should return false for caller not in allow list")
	}
	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy, got 0x%02x", signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called for caller not in allow list")
	}
}

func TestCallerIdentified_BusyDueToState(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetState(StateCalling)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("somehash", signalFunc, teardownFunc); ok {
		t.Error("CallerIdentified should return false when busy")
	}
	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy, got 0x%02x", signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called when busy")
	}
}

func TestCallerIdentified_BusyDueToExternalBusy(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetBusy(true)

	var signalled int
	signalFunc := func(code int) error {
		signalled = code
		return nil
	}
	var teardownCalled bool
	teardownFunc := func() { teardownCalled = true }

	if ok := tel.CallerIdentified("somehash", signalFunc, teardownFunc); ok {
		t.Error("CallerIdentified should return false when externally busy")
	}
	if signalled != SignallingBusy {
		t.Errorf("Expected SignallingBusy, got 0x%02x", signalled)
	}
	if !teardownCalled {
		t.Error("Teardown should be called when externally busy")
	}
}

func TestCallerIdentified_ResetsDiallingPipelines(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetProfile(ProfileQualityMedium)

	signalFunc := func(code int) error { return nil }

	if ok := tel.CallerIdentified("somehash", signalFunc, nil); !ok {
		t.Fatal("CallerIdentified should return true for allowed caller")
	}

	// After CallerIdentified, dialling pipelines should have been created
	// (by ResetDiallingPipelines)
	if tel.ReceiveMixer() == nil {
		t.Error("ReceiveMixer should be created after CallerIdentified")
	}
	if tel.DialTone() == nil {
		t.Error("DialTone should be created after CallerIdentified")
	}
}

func TestCallerIdentified_AutoAnswer(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 5*time.Second, AllowAll, 0.0, 0.0)

	signalFunc := func(code int) error { return nil }

	if ok := tel.CallerIdentified("somehash", signalFunc, nil); !ok {
		t.Fatal("CallerIdentified should return true for allowed caller")
	}

	if tel.AutoAnswer() != 5*time.Second {
		t.Error("AutoAnswer should be 5s")
	}
}

func TestIsCallerAllowed_AllowAll(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	if !tel.IsCallerAllowed("any_hash") {
		t.Error("AllowAll should allow any caller")
	}
}

func TestIsCallerAllowed_AllowNone(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)

	if tel.IsCallerAllowed("any_hash") {
		t.Error("AllowNone should reject all callers")
	}
}

func TestIsCallerAllowed_BlockedList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)
	tel.SetBlockedList([]string{"blocked_hash"})

	if tel.IsCallerAllowed("blocked_hash") {
		t.Error("Blocked caller should not be allowed")
	}
	if !tel.IsCallerAllowed("allowed_hash") {
		t.Error("Non-blocked caller should be allowed with AllowAll")
	}
}

func TestIsCallerAllowed_AllowList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)
	tel.SetAllowList([]string{"allowed_hash"})

	if !tel.IsCallerAllowed("allowed_hash") {
		t.Error("Caller in allow list should be allowed")
	}
	if tel.IsCallerAllowed("not_in_list") {
		t.Error("Caller not in allow list should be rejected with AllowNone + list")
	}
}

func TestIsCallerAllowed_BlockListOverridesAllowList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)
	tel.SetAllowList([]string{"hash1"})
	tel.SetBlockedList([]string{"hash1"})

	// Blocked list takes priority
	if tel.IsCallerAllowed("hash1") {
		t.Error("Blocked caller should not be allowed even if in allow list")
	}
}

func TestSetBlockedList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowAll, 0.0, 0.0)

	list := []string{"hash1", "hash2"}
	tel.SetBlockedList(list)

	bl := tel.BlockedList()
	if len(bl) != 2 {
		t.Errorf("Expected 2 blocked entries, got %v", len(bl))
	}
}

func TestSetAllowList(t *testing.T) {
	tel := NewTelephone(RingTime, WaitTime, 0, AllowNone, 0.0, 0.0)

	list := []string{"hash1", "hash2", "hash3"}
	tel.SetAllowList(list)

	al := tel.AllowList()
	if len(al) != 3 {
		t.Errorf("Expected 3 allow entries, got %v", len(al))
	}
}
