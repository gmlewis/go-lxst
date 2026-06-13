// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package hardware provides hardware interfaces (keypads, displays) for telephony applications.
package hardware

import (
	"testing"
	"time"
)

func TestKeypad4x4_DefaultMap(t *testing.T) {
	t.Parallel()
	k := NewKeypad4x4(nil, nil, nil, nil)
	if k.Rows() != 4 || k.Cols() != 4 {
		t.Fatalf("expected 4x4, got %dx%d", k.Rows(), k.Cols())
	}
	expected := [][]string{
		{"1", "2", "3", "A"},
		{"4", "5", "6", "B"},
		{"7", "8", "9", "C"},
		{"*", "0", "#", "D"},
	}
	if !equalKeyMaps(k.KeyMap(), expected) {
		t.Errorf("key map mismatch\nexpected: %v\ngot:      %v", expected, k.KeyMap())
	}
}

func TestKeypad5x5_DefaultMap(t *testing.T) {
	t.Parallel()
	k := NewKeypad5x5(nil, nil, nil, nil)
	if k.Rows() != 5 || k.Cols() != 5 {
		t.Fatalf("expected 5x5, got %dx%d", k.Rows(), k.Cols())
	}
	expected := [][]string{
		{"P", "R", "M", "-", "+"},
		{"1", "2", "3", "A", "B"},
		{"4", "5", "6", "C", "D"},
		{"7", "8", "9", "E", "F"},
		{"*", "0", "#", "N", "K"},
	}
	if !equalKeyMaps(k.KeyMap(), expected) {
		t.Errorf("key map mismatch\nexpected: %v\ngot:      %v", expected, k.KeyMap())
	}
}

func TestKeypad_CustomKeyMap(t *testing.T) {
	t.Parallel()
	custom := [][]string{
		{"A", "B", "C"},
		{"D", "E", "F"},
	}
	k := NewKeypad4x4Custom(2, 3, custom, []int{1, 2}, []int{3, 4, 5}, nil)
	if k.Rows() != 2 || k.Cols() != 3 {
		t.Fatalf("expected 2x3, got %dx%d", k.Rows(), k.Cols())
	}
	if !equalKeyMaps(k.KeyMap(), custom) {
		t.Errorf("custom key map not applied")
	}
}

func TestKeypad_IsUpIsDown(t *testing.T) {
	t.Parallel()
	k := NewKeypad4x4(nil, nil, nil, nil)
	if !k.IsUp("1") {
		t.Error("key '1' should be up initially")
	}
	if k.IsDown("1") {
		t.Error("key '1' should not be down initially")
	}
	// Invalid key should return false for both
	if k.IsDown("INVALID") || k.IsUp("INVALID") {
		t.Error("invalid key should return false for both IsDown and IsUp")
	}

	// Simulate key down via callback
	var events []KeyEvent
	k.Start(func(keypad Keypad, event KeyEvent) {
		events = append(events, event)
	})
	time.Sleep(10 * time.Millisecond) // allow goroutine to start

	// Manually trigger a "scan" by calling the internal test method
	k.TestSimulatePress("1")
	time.Sleep(10 * time.Millisecond)

	if !k.IsDown("1") {
		t.Error("key '1' should be down after press")
	}
	if k.IsUp("1") {
		t.Error("key '1' should not be up after press")
	}

	k.Stop()
}

func TestKeypad_Callback(t *testing.T) {
	t.Parallel()
	var events []KeyEvent
	k := NewKeypad4x4(nil, nil, nil, func(_ Keypad, e KeyEvent) {
		events = append(events, e)
	})
	k.Start(func(keypad Keypad, e KeyEvent) {
		events = append(events, e)
	})
	defer k.Stop()

	k.TestSimulatePress("5")
	k.TestSimulateRelease("5")
	time.Sleep(20 * time.Millisecond)

	if len(events) != 2 {
		t.Fatalf("expected 2 events (down, up), got %d: %v", len(events), events)
	}
	if events[0].Key != "5" || events[0].Type != EventDown {
		t.Errorf("first event: got %+v, want key=5 type=Down", events[0])
	}
	if events[1].Key != "5" || events[1].Type != EventUp {
		t.Errorf("second event: got %+v, want key=5 type=Up", events[1])
	}
}

func TestLCD_Basic(t *testing.T) {
	t.Parallel()
	lcd := NewLCD(nil)
	if lcd.Cols() != 16 || lcd.Rows() != 2 {
		t.Fatalf("expected 16x2, got %dx%d", lcd.Cols(), lcd.Rows())
	}
	if lcd.IsSleeping() {
		t.Error("LCD should not be sleeping initially")
	}
	lcd.Clear()
	lcd.Print("Hello", 0, 0)
	lcd.Print("World", 0, 1)
	lcd.Sleep()
	if !lcd.IsSleeping() {
		t.Error("LCD should be sleeping after Sleep()")
	}
	lcd.Wake()
	if lcd.IsSleeping() {
		t.Error("LCD should not be sleeping after Wake()")
	}
	lcd.Close()
}

func TestLCD_Bounds(t *testing.T) {
	t.Parallel()
	lcd := NewLCD(nil)
	defer lcd.Close()

	// Should not panic on out of bounds
	lcd.Print("Test", -1, -1)
	lcd.Print("Test", 20, 20)
	lcd.Print("Test", 15, 1)
}

func equalKeyMaps(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

type mockGPIODriver struct {
	pinModes map[int]GPIOMode
	outputs  map[int]bool
	inputs   map[int]bool
}

func (m *mockGPIODriver) Setup(pin int, mode GPIOMode) error {
	if m.pinModes == nil {
		m.pinModes = make(map[int]GPIOMode)
	}
	m.pinModes[pin] = mode
	return nil
}

func (m *mockGPIODriver) SetOutput(pin int, high bool) error {
	if m.outputs == nil {
		m.outputs = make(map[int]bool)
	}
	m.outputs[pin] = high
	return nil
}

func (m *mockGPIODriver) ReadInput(pin int) (bool, error) {
	if m.inputs == nil {
		return false, nil
	}
	return m.inputs[pin], nil
}

func (m *mockGPIODriver) SetPullUpDown(pin int, pull PullUpDown) error {
	return nil
}

func (m *mockGPIODriver) Close() error { return nil }

type mockKeypad struct {
	Keypad
}

type mockI2CDriver struct {
	written []byte
	addr    int
}

func (m *mockI2CDriver) WriteToDevice(addr int, data byte) error {
	m.addr = addr
	m.written = append(m.written, data)
	return nil
}

func (m *mockI2CDriver) Close() error { return nil }

func TestKeypad_UsesGPIODriver(t *testing.T) {
	t.Parallel()

	mock := &mockGPIODriver{}
	SetGPIODriver(mock)
	defer SetGPIODriver(nil)

	var events []KeyEvent
	k := NewKeypad4x4(nil, nil, nil, func(_ Keypad, e KeyEvent) {
		events = append(events, e)
	})

	// Simulate a key press by setting the mock input state
	mock.inputs = map[int]bool{DefaultColPins4x4[0]: true}
	k.TestSimulatePress("1")
	time.Sleep(10 * time.Millisecond)

	if !k.IsDown("1") {
		t.Error("key '1' should be down after press via GPIO driver")
	}

	k.Stop()
}

func TestLCD_UsesI2CDriver(t *testing.T) {
	t.Parallel()

	mock := &mockI2CDriver{}
	SetI2CDriver(mock)
	defer SetI2CDriver(nil)

	lcd := NewLCD(&LCDConfig{Address: 0x27, I2CBus: 1})
	lcd.Print("Hi", 0, 0)
	lcd.Close()

	// The mock should have received I2C writes (init + print)
	if len(mock.written) == 0 {
		t.Error("LCD should write to I2CDriver when available")
	}
}

func TestKeypad_Hook(t *testing.T) {
	t.Parallel()
	k := NewKeypad4x4(nil, nil, nil, nil)
	k.EnableHook(DefaultHookPin4x4)
	time.Sleep(5 * time.Millisecond)
	// Hook should be tracked and initially up (not pressed)
	if !k.IsUp("hook") {
		t.Error("hook should be up initially")
	}
	if k.IsDown("hook") {
		t.Error("hook should not be down initially")
	}
	k.DisableHook()
	// After disable, hook should no longer be tracked
	if k.IsDown("hook") || k.IsUp("hook") {
		t.Error("hook should not be tracked after DisableHook")
	}
}
