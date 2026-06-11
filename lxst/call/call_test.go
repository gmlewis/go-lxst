// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package call

import (
	"testing"
)

func TestCallEndpoint_New(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)
	if ce == nil {
		t.Fatal("NewCallEndpoint returned nil")
	}
	if !ce.AutoAnswer() {
		t.Error("AutoAnswer should default to true")
	}
}

func TestCallEndpoint_SetAutoAnswer(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)
	ce.SetAutoAnswer(false)
	if ce.AutoAnswer() {
		t.Error("AutoAnswer should be false after SetAutoAnswer(false)")
	}
	ce.SetAutoAnswer(true)
	if !ce.AutoAnswer() {
		t.Error("AutoAnswer should be true after SetAutoAnswer(true)")
	}
}

func TestCallEndpoint_IncomingCallCallback(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)

	var called bool
	err := ce.SetIncomingCallCallback(func(link any) {
		called = true
	})
	if err != nil {
		t.Fatalf("SetIncomingCallCallback failed: %v", err)
	}
	if ce.incomingCallCallback == nil {
		t.Error("Callback should be set")
	}
	_ = called
}

func TestCallEndpoint_HasActiveCall(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)
	if ce.HasActiveCall() {
		t.Error("Should not have active call initially")
	}
}

func TestCallEndpoint_Terminate_NoCall(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)
	err := ce.Terminate()
	if err == nil {
		t.Error("Expected error when terminating with no active call")
	}
}

func TestCallEndpoint_Terminate_WithCall(t *testing.T) {
	t.Parallel()

	ce := NewCallEndpoint(nil)
	ce.activeCall = "test-link"

	if !ce.HasActiveCall() {
		t.Error("Should have active call")
	}

	err := ce.Terminate()
	if err != nil {
		t.Fatalf("Terminate failed: %v", err)
	}

	if ce.HasActiveCall() {
		t.Error("Should not have active call after terminate")
	}
}
