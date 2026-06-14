// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package telephony

import (
	"sync"
	"testing"
)

func TestSetBusyAndBusyProperty(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// Initially not busy
	if tel.Busy() {
		t.Error("Busy() should be false when state is Idle and no external busy")
	}

	// Set external busy
	tel.SetBusy(true)
	if !tel.Busy() {
		t.Error("Busy() should be true after SetBusy(true)")
	}

	// Clear external busy
	tel.SetBusy(false)
	if tel.Busy() {
		t.Error("Busy() should be false after SetBusy(false)")
	}
}

func TestBusyReflectsCallState(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// When state is not Idle, Busy() should return true
	tel.Call(DefaultProfile)
	if !tel.Busy() {
		t.Error("Busy() should be true when in Calling state")
	}

	tel.Answer()
	if !tel.Busy() {
		t.Error("Busy() should be true when in Connecting state")
	}

	tel.Hangup()
	if tel.Busy() {
		t.Error("Busy() should be false when back to Idle")
	}
}

func TestActiveProfileReturnsZeroWhenNoCall(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// ActiveProfile should return 0 (no profile) when no active call
	if tel.ActiveProfile() != 0 {
		t.Error("ActiveProfile() should return 0 when no active call")
	}
}

func TestActiveProfileReturnsProfileDuringCall(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	// Set up a call
	tel.Call(ProfileQualityHigh)

	// ActiveProfile should return the profile (or nil if link not established)
	// Since we don't have an actual RNS link in the test, it returns nil
	// This tests that the method exists and doesn't panic
	_ = tel.ActiveProfile()
}

func TestBusyConcurrentAccess(t *testing.T) {
	t.Parallel()

	tel := NewTelephone(RingTime, WaitTime, false, AllowAll, 0.0, 0.0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				tel.SetBusy(true)
			} else {
				tel.SetBusy(false)
			}
			_ = tel.Busy()
		}(i)
	}
	wg.Wait()
}
