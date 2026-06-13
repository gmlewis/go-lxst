// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestNewPhone(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)
	if phone == nil {
		t.Fatal("NewPhone() returned nil")
	}
	if !phone.IsAvailable() {
		t.Errorf("IsAvailable() = false, want true")
	}
	if phone.State() != StateAvailable {
		t.Errorf("State() = %d, want %d", phone.State(), StateAvailable)
	}
}

func TestPhoneStateTransitions(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	if !phone.IsAvailable() {
		t.Fatal("phone should start available")
	}

	phone.Dial("aabbccdd11223344aabbccdd11223344")
	if !phone.CallIsConnecting() {
		t.Errorf("after Dial(), CallIsConnecting() = false, want true")
	}
	if phone.State() != StateConnecting {
		t.Errorf("State() = %d, want %d", phone.State(), StateConnecting)
	}

	phone.CallEstablished()
	if !phone.IsInCall() {
		t.Errorf("after CallEstablished(), IsInCall() = false, want true")
	}

	phone.Hangup()
	if !phone.IsAvailable() {
		t.Errorf("after Hangup(), IsAvailable() = false, want true")
	}
}

func TestPhoneIncomingCall(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Ringing("11223344aabbccdd11223344aabbccdd")
	if !phone.IsRinging() {
		t.Errorf("after Ringing(), IsRinging() = false, want true")
	}
	if phone.CallerHash() != "11223344aabbccdd11223344aabbccdd" {
		t.Errorf("CallerHash() = %q, want %q", phone.CallerHash(), "11223344aabbccdd11223344aabbccdd")
	}

	ok := phone.Answer()
	if !ok {
		t.Errorf("Answer() returned false, want true")
	}
	if !phone.CallIsConnecting() {
		t.Errorf("after Answer(), CallIsConnecting() = false, want true")
	}

	phone.CallEstablished()
	if !phone.IsInCall() {
		t.Errorf("after CallEstablished(), IsInCall() = false, want true")
	}
}

func TestPhoneReject(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Ringing("11223344aabbccdd11223344aabbccdd")
	if !phone.IsRinging() {
		t.Fatal("phone should be ringing")
	}

	phone.Reject()
	if !phone.IsAvailable() {
		t.Errorf("after Reject(), IsAvailable() = false, want true")
	}
}

func TestPhoneRedial(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Dial("aabbccdd11223344aabbccdd11223344")
	phone.Hangup()

	phone.Redial()
	if !phone.CallIsConnecting() {
		t.Errorf("after Redial(), CallIsConnecting() = false, want true")
	}
	if phone.LastDialledHash() != "aabbccdd11223344aabbccdd11223344" {
		t.Errorf("LastDialledHash() = %q, want %q", phone.LastDialledHash(), "aabbccdd11223344aabbccdd11223344")
	}
}

func TestPhoneCallerLookup(t *testing.T) {
	t.Parallel()
	cfg := &PhoneConfig{
		Telephone: TelephoneConfig{
			AllowedCallers: "all",
		},
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344", Alias: "100"},
		},
	}
	phone := NewPhone(cfg)

	phone.Ringing("aabbccdd11223344aabbccdd11223344")
	if phone.CallerName() != "Alice" {
		t.Errorf("CallerName() = %q, want %q", phone.CallerName(), "Alice")
	}
	if phone.CallerAlias() != "100" {
		t.Errorf("CallerAlias() = %q, want %q", phone.CallerAlias(), "100")
	}
}

func TestPhoneProcessInput(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	keepGoing := phone.ProcessInput("q")
	if keepGoing {
		t.Error("ProcessInput('q') returned true, want false")
	}
}

func TestPhoneProcessInputHelp(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	keepGoing := phone.ProcessInput("h")
	if !keepGoing {
		t.Error("ProcessInput('h') returned false, want true")
	}
}

func TestPhoneProcessInputDial(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	keepGoing := phone.ProcessInput("aabbccdd11223344aabbccdd11223344")
	if !keepGoing {
		t.Error("ProcessInput(hash) returned false, want true")
	}
	if !phone.CallIsConnecting() {
		t.Error("phone should be connecting after dialling")
	}
}

func TestPhoneProcessInputAliasDial(t *testing.T) {
	t.Parallel()
	cfg := &PhoneConfig{
		Telephone: TelephoneConfig{
			AllowedCallers: "all",
		},
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344", Alias: "100"},
		},
	}
	phone := NewPhone(cfg)

	keepGoing := phone.ProcessInput("100")
	if !keepGoing {
		t.Error("ProcessInput(alias) returned false, want true")
	}
	if !phone.CallIsConnecting() {
		t.Error("phone should be connecting after alias dial")
	}
}

func TestPhoneAnswerFromRinging(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Ringing("aabbccdd11223344aabbccdd11223344")
	phone.ProcessInput("")
	if !phone.CallIsConnecting() {
		t.Error("phone should be connecting after answering")
	}
}

func TestPhoneRejectFromRinging(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Ringing("aabbccdd11223344aabbccdd11223344")
	phone.ProcessInput("r")
	if !phone.IsAvailable() {
		t.Error("phone should be available after rejecting")
	}
}

func TestPhoneHangupFromCall(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	phone.Dial("aabbccdd11223344aabbccdd11223344")
	phone.CallEstablished()
	phone.ProcessInput("")
	if !phone.IsAvailable() {
		t.Error("phone should be available after hangup")
	}
}

func TestPhonePrintPhonebookEmpty(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)
	phone.PrintPhonebook()
}

func TestPhonePrintPhonebookWithEntries(t *testing.T) {
	t.Parallel()
	cfg := &PhoneConfig{
		Telephone: TelephoneConfig{
			AllowedCallers: "all",
		},
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344", Alias: "100"},
			"Bob":   {Hash: "11223344aabbccdd11223344aabbccdd"},
		},
	}
	phone := NewPhone(cfg)
	phone.PrintPhonebook()
}

func TestPhonePrintIdentity(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)
	phone.PrintIdentity("aabbccdd11223344aabbccdd11223344")
}

func TestPhonePrintDestination(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)
	phone.PrintDestination("aabbccdd11223344aabbccdd11223344")
}

func TestPhoneStatusString(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	phone := NewPhone(cfg)

	tests := []struct {
		state byte
		want  string
	}{
		{StateAvailable, "Available"},
		{StateConnecting, "Connecting"},
		{StateRinging, "Ringing"},
	}

	for _, tt := range tests {
		phone.SetState(tt.state)
		got := phone.StatusString()
		if got != tt.want {
			t.Errorf("StatusString() for state %d = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestPrettyHex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"aabbccdd11223344aabbccdd11223344", "aabbccdd:11223344:aabbccdd:11223344"},
		{"1234567890abcdef1234567890abcdef", "12345678:90abcdef:12345678:90abcdef"},
		{"short", "short"},
	}

	for _, tt := range tests {
		got := prettyHex(tt.input)
		if got != tt.want {
			t.Errorf("prettyHex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{"", ""},
		{"\thello\t", "hello"},
	}

	for _, tt := range tests {
		got := trimSpace(tt.input)
		if got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
