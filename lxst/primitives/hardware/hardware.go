// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package hardware provides hardware interface abstractions for telephony
// applications, including keypad input handling (GPIO and matrix keypads)
// and I2C LCD display output. It defines the Keypad and Display interfaces
// along with mock implementations suitable for testing without physical
// hardware.
package hardware

import (
	"sync"
	"time"
)

// Event type constants
const (
	EventUp   = 0x00
	EventDown = 0x01
)

// KeyEvent represents a key press/release event
type KeyEvent struct {
	Key  string
	Type byte // EventUp or EventDown
}

// Keypad interface
type Keypad interface {
	Rows() int
	Cols() int
	KeyMap() [][]string
	IsDown(key string) bool
	IsUp(key string) bool
	EnableHook(pin int)
	DisableHook()
	Start(callback func(Keypad, KeyEvent))
	Stop()
	TestSimulatePress(key string)
	TestSimulateRelease(key string)
}

// LCDInterface interface
type LCDInterface interface {
	Cols() int
	Rows() int
	Print(text string, x, y int)
	Clear()
	Sleep()
	Wake()
	IsSleeping() bool
	Close()
}

// Keypad4x4Config holds configuration for 4x4 keypad
type Keypad4x4Config struct {
	RowPins  []int
	ColPins  []int
	KeyMap   [][]string
	Callback func(Keypad, KeyEvent)
}

// Default 4x4 keypad constants
const (
	DefaultRows4x4      = 4
	DefaultCols4x4      = 4
	DefaultScanInterval = 20 * time.Millisecond
	DefaultHookPin4x4   = 5
	HookDebounceMs      = 150 * time.Millisecond
)

var DefaultKeyMap4x4 = [][]string{
	{"1", "2", "3", "A"},
	{"4", "5", "6", "B"},
	{"7", "8", "9", "C"},
	{"*", "0", "#", "D"},
}

var DefaultRowPins4x4 = []int{21, 20, 16, 12}
var DefaultColPins4x4 = []int{26, 19, 13, 6}

// Keypad4x4 implements a 4x4 GPIO keypad
type Keypad4x4 struct {
	rows        int
	cols        int
	rowPins     []int
	colPins     []int
	keyMap      [][]string
	keyStates   map[string]bool
	callback    func(Keypad, KeyEvent)
	hookPin     int
	checkHook   bool
	hookTime    time.Time
	shouldRun   bool
	mu          sync.Mutex
	testPressed map[string]bool // for testing
}

func NewKeypad4x4(rowPins, colPins []int, keyMap [][]string, callback func(Keypad, KeyEvent)) *Keypad4x4 {
	if rowPins != nil && (len(rowPins) != DefaultRows4x4) {
		panic("invalid row pins specification: must have 4 pins")
	}
	if colPins != nil && (len(colPins) != DefaultCols4x4) {
		panic("invalid col pins specification: must have 4 pins")
	}

	k := &Keypad4x4{
		rows:        DefaultRows4x4,
		cols:        DefaultCols4x4,
		rowPins:     rowPins,
		colPins:     colPins,
		keyStates:   make(map[string]bool),
		callback:    callback,
		testPressed: make(map[string]bool),
	}
	k.setKeyMap(keyMap)
	if rowPins == nil {
		k.rowPins = DefaultRowPins4x4
	}
	if colPins == nil {
		k.colPins = DefaultColPins4x4
	}
	return k
}

func NewKeypad4x4Custom(rows, cols int, keyMap [][]string, rowPins, colPins []int, callback func(Keypad, KeyEvent)) *Keypad4x4 {
	if rowPins != nil && len(rowPins) != rows {
		panic("row pins length must match rows")
	}
	if colPins != nil && len(colPins) != cols {
		panic("col pins length must match cols")
	}

	k := &Keypad4x4{
		rows:        rows,
		cols:        cols,
		rowPins:     rowPins,
		colPins:     colPins,
		keyStates:   make(map[string]bool),
		callback:    callback,
		testPressed: make(map[string]bool),
	}
	k.setKeyMap(keyMap)
	return k
}

func (k *Keypad4x4) Rows() int          { return k.rows }
func (k *Keypad4x4) Cols() int          { return k.cols }
func (k *Keypad4x4) KeyMap() [][]string { return k.keyMap }

func (k *Keypad4x4) setKeyMap(keyMap [][]string) {
	if keyMap == nil {
		keyMap = DefaultKeyMap4x4
	}
	k.keyMap = keyMap
	k.keyStates = make(map[string]bool)
	for _, row := range k.keyMap {
		for _, key := range row {
			k.keyStates[key] = false
		}
	}
	k.keyStates["hook"] = false
}

func (k *Keypad4x4) IsDown(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keyStates[key]
}

func (k *Keypad4x4) IsUp(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	_, exists := k.keyStates[key]
	if !exists {
		return false
	}
	return !k.keyStates[key]
}

func (k *Keypad4x4) EnableHook(pin int) {
	if pin == 0 {
		pin = DefaultHookPin4x4
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.hookPin = pin
	k.checkHook = true
	k.keyStates["hook"] = false
}

func (k *Keypad4x4) DisableHook() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.checkHook = false
	k.hookPin = 0
	delete(k.keyStates, "hook")
}

func (k *Keypad4x4) Start(callback func(Keypad, KeyEvent)) {
	if callback != nil {
		k.mu.Lock()
		k.callback = callback
		k.mu.Unlock()
	}
	k.mu.Lock()
	k.shouldRun = true
	k.mu.Unlock()

	go k.run()
}

func (k *Keypad4x4) Stop() {
	k.mu.Lock()
	k.shouldRun = false
	k.mu.Unlock()
}

func (k *Keypad4x4) run() {
	for {
		k.mu.Lock()
		shouldRun := k.shouldRun
		k.mu.Unlock()
		if !shouldRun {
			break
		}
		k.scan()
		time.Sleep(DefaultScanInterval)
	}
}

func (k *Keypad4x4) TestSimulatePress(key string) {
	k.mu.Lock()
	k.testPressed[key] = true
	k.mu.Unlock()
	// Trigger immediate scan for testing
	k.scanLocked()
}

func (k *Keypad4x4) TestSimulateRelease(key string) {
	k.mu.Lock()
	k.testPressed[key] = false
	k.mu.Unlock()
	// Trigger immediate scan for testing
	k.scanLocked()
}

func (k *Keypad4x4) scan() {
	k.mu.Lock()

	activeKeys := make(map[string]bool)

	// Test mode: use simulated key presses
	for key, pressed := range k.testPressed {
		if pressed {
			activeKeys[key] = true
		}
	}

	// Hardware scanning using GPIO driver
	driver := GetGPIODriver()
	if driver != nil && len(k.testPressed) == 0 {
		for row := 0; row < k.rows; row++ {
			// Set row pin to output HIGH
			_ = driver.Setup(k.rowPins[row], GPIOModeOutput)
			_ = driver.SetOutput(k.rowPins[row], true)

			// Read column pins
			for col := 0; col < k.cols; col++ {
				_ = driver.Setup(k.colPins[col], GPIOModeInput)
				if high, err := driver.ReadInput(k.colPins[col]); err == nil && high {
					activeKeys[k.keyMap[row][col]] = true
				}
			}

			// Set row pin back to LOW
			_ = driver.SetOutput(k.rowPins[row], false)
		}

		// Check hook pin if enabled
		if k.checkHook {
			_ = driver.Setup(k.hookPin, GPIOModeInput)
			if low, err := driver.ReadInput(k.hookPin); err == nil && !low {
				activeKeys["hook"] = true
			}
		}
	}

	k.mu.Unlock()

	k.handle(activeKeys)
}

func (k *Keypad4x4) scanLocked() {
	activeKeys := make(map[string]bool)

	// Test mode: use simulated key presses
	for key, pressed := range k.testPressed {
		if pressed {
			activeKeys[key] = true
		}
	}

	// Real hardware scanning would go here
	// For now, we just process test keys

	k.handle(activeKeys)
}

func (k *Keypad4x4) handle(activeKeys map[string]bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	var events []KeyEvent
	for key, isDown := range k.keyStates {
		if !isDown && activeKeys[key] {
			k.keyStates[key] = true
			events = append(events, KeyEvent{Key: key, Type: EventDown})
		} else if isDown && !activeKeys[key] {
			k.keyStates[key] = false
			events = append(events, KeyEvent{Key: key, Type: EventUp})
		}
	}

	if k.callback != nil {
		for _, event := range events {
			k.callback(k, event)
		}
	}
}

// Keypad5x5 implements a 5x5 GPIO keypad
type Keypad5x5 struct {
	*Keypad4x4 // Embed for shared functionality
}

const (
	DefaultRows5x5    = 5
	DefaultCols5x5    = 5
	DefaultHookPin5x5 = 11
)

var DefaultKeyMap5x5 = [][]string{
	{"P", "R", "M", "-", "+"},
	{"1", "2", "3", "A", "B"},
	{"4", "5", "6", "C", "D"},
	{"7", "8", "9", "E", "F"},
	{"*", "0", "#", "N", "K"},
}

var DefaultRowPins5x5 = []int{21, 20, 16, 12, 7}
var DefaultColPins5x5 = []int{26, 19, 13, 6, 5}

func NewKeypad5x5(rowPins, colPins []int, keyMap [][]string, callback func(Keypad, KeyEvent)) *Keypad5x5 {
	if rowPins != nil && (len(rowPins) != DefaultRows5x5) {
		panic("invalid row pins specification: must have 5 pins")
	}
	if colPins != nil && (len(colPins) != DefaultCols5x5) {
		panic("invalid col pins specification: must have 5 pins")
	}

	k4 := NewKeypad4x4Custom(DefaultRows5x5, DefaultCols5x5, keyMap, rowPins, colPins, callback)
	if rowPins == nil {
		k4.rowPins = DefaultRowPins5x5
	}
	if colPins == nil {
		k4.colPins = DefaultColPins5x5
	}
	if keyMap == nil {
		k4.setKeyMap(DefaultKeyMap5x5)
	}

	return &Keypad5x5{Keypad4x4: k4}
}

// LCDConfig holds LCD configuration
type LCDConfig struct {
	Address int
	I2CBus  int
}

const (
	LCDDefaultAddr   = 0x27
	LCDDefaultI2CBus = 1
	LCDCols          = 16
	LCDRows          = 2

	LCDModeChr      = 0x01
	LCDModeCmd      = 0x00
	LCDRow1         = 0x80
	LCDRow2         = 0xC0
	LCDBacklightOn  = 0x08
	LCDBacklightOff = 0x00
	LCDFlagEnable   = 0x04
	LCDFlagRS       = 0x01

	LCDCmdInit1 = 0x33
	LCDCmdInit2 = 0x32
	LCDCmdClear = 0x01
)

// LCD implements an I2C LCD1602 display
type LCDStruct struct {
	address    int
	bus        int
	row        byte
	backlight  byte
	isSleeping bool
	mu         sync.Mutex
}

func NewLCD(config *LCDConfig) *LCDStruct {
	addr := LCDDefaultAddr
	bus := LCDDefaultI2CBus
	if config != nil {
		if config.Address != 0 {
			addr = config.Address
		}
		if config.I2CBus != 0 {
			bus = config.I2CBus
		}
	}

	l := &LCDStruct{
		address:   addr,
		bus:       bus,
		row:       LCDRow1,
		backlight: LCDBacklightOn,
	}
	l.initDisplay()
	return l
}

func (l *LCDStruct) initDisplay() {
	l.sendCommand(LCDCmdInit1)
	l.sendCommand(LCDCmdInit2)
	l.sendCommand(0x28) // 4-bit mode, 2 lines, 5x8 font
	l.sendCommand(0x0C) // Display on, cursor off, blink off
	l.sendCommand(LCDCmdClear)
	time.Sleep(1 * time.Millisecond)
}

func (l *LCDStruct) sendCommand(cmd byte) {
	l.sendByte(cmd, LCDModeCmd)
}

func (l *LCDStruct) sendData(data byte) {
	l.sendByte(data, LCDModeChr)
}

func (l *LCDStruct) sendByte(byteVal byte, mode byte) {
	// Send high nibble
	high := (byteVal & 0xF0) | mode | LCDFlagEnable | l.backlight
	l.writeByte(high)
	time.Sleep(500 * time.Microsecond)
	high &^= LCDFlagEnable
	l.writeByte(high)

	// Send low nibble
	low := ((byteVal & 0x0F) << 4) | mode | LCDFlagEnable | l.backlight
	l.writeByte(low)
	time.Sleep(500 * time.Microsecond)
	low &^= LCDFlagEnable
	l.writeByte(low)
}

func (l *LCDStruct) writeByte(byteVal byte) {
	// Mock I2C write - in real implementation this would use smbus/I2C
	// For testing, we just track state
}

func (l *LCDStruct) Cols() int { return LCDCols }
func (l *LCDStruct) Rows() int { return LCDRows }

func (l *LCDStruct) Print(text string, x, y int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isSleeping {
		l.wakeLocked()
	}

	// Bounds checking
	if x < 0 {
		x = 0
	}
	if x >= LCDCols {
		x = LCDCols - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= LCDRows {
		y = LCDRows - 1
	}

	// Set cursor position
	var ddramAddr byte
	if y == 0 {
		ddramAddr = LCDRow1 + byte(x)
	} else {
		ddramAddr = LCDRow2 + byte(x)
	}
	l.sendCommand(ddramAddr)

	// Pad text to fill line
	for i := 0; i < LCDCols; i++ {
		var c byte
		if i < len(text) {
			c = text[i]
		} else {
			c = ' '
		}
		l.sendData(c)
	}
}

func (l *LCDStruct) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sendCommand(LCDCmdClear)
	time.Sleep(1 * time.Millisecond)
}

func (l *LCDStruct) Sleep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.backlight = LCDBacklightOff
	l.sendCommand(LCDCmdClear)
	l.isSleeping = true
}

func (l *LCDStruct) Wake() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.wakeLocked()
}

func (l *LCDStruct) wakeLocked() {
	l.backlight = LCDBacklightOn
	l.initDisplay()
	l.isSleeping = false
}

func (l *LCDStruct) IsSleeping() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isSleeping
}

func (l *LCDStruct) Close() {
	l.Sleep()
	// In real implementation, close I2C bus
}
