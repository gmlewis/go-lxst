// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package hardware

// GPIODriver abstracts GPIO operations for cross-platform keypad support.
type GPIODriver interface {
	// Setup configures a GPIO pin with the given mode (input/output).
	Setup(pin int, mode GPIOMode) error
	// SetOutput sets a GPIO output pin to high or low.
	SetOutput(pin int, high bool) error
	// ReadInput reads the state of a GPIO input pin.
	ReadInput(pin int) (bool, error)
	// SetPullUpDown configures the pull-up/pull-down resistor for an input pin.
	SetPullUpDown(pin int, pull PullUpDown) error
	// Close releases any resources held by the driver.
	Close() error
}

// GPIOMode represents GPIO pin modes.
type GPIOMode int

const (
	GPIOModeInput  GPIOMode = 0
	GPIOModeOutput GPIOMode = 1
)

// PullUpDown represents pull-up/pull-down resistor configuration.
type PullUpDown int

const (
	PullNone PullUpDown = 0
	PullUp   PullUpDown = 1
	PullDown PullUpDown = 2
)

// I2CDriver abstracts I2C operations for cross-platform LCD support.
type I2CDriver interface {
	// WriteToDevice writes a single byte to the specified device address on the bus.
	WriteToDevice(addr int, data byte) error
	// Close releases any resources held by the driver.
	Close() error
}
