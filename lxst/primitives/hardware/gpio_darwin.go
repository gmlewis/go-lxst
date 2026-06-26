// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build darwin

package hardware

import (
	"errors"
	"fmt"
)

// ErrNoGPIOOnDarwin is returned when GPIO operations are attempted on macOS.
var ErrNoGPIOOnDarwin = errors.New("GPIO not available on macOS: use external USB-to-GPIO adapter")

// DarwinGPIO implements GPIODriver as a stub for macOS.
type DarwinGPIO struct{}

// NewDarwinGPIO creates a new Darwin GPIO stub driver.
func NewDarwinGPIO() *DarwinGPIO {
	return &DarwinGPIO{}
}

func (g *DarwinGPIO) Setup(pin int, mode GPIOMode) error {
	return fmt.Errorf("%w: pin=%v mode=%v", ErrNoGPIOOnDarwin, pin, mode)
}

func (g *DarwinGPIO) SetOutput(pin int, high bool) error {
	return fmt.Errorf("%w: pin=%v high=%v", ErrNoGPIOOnDarwin, pin, high)
}

func (g *DarwinGPIO) ReadInput(pin int) (bool, error) {
	return false, fmt.Errorf("%w: pin=%v", ErrNoGPIOOnDarwin, pin)
}

func (g *DarwinGPIO) SetPullUpDown(pin int, pull PullUpDown) error {
	return fmt.Errorf("%w: pin=%v pull=%v", ErrNoGPIOOnDarwin, pin, pull)
}

func (g *DarwinGPIO) Close() error {
	return nil
}

func init() {
	// Register Darwin GPIO driver as default
	defaultGPIODriver = NewDarwinGPIO()
}
