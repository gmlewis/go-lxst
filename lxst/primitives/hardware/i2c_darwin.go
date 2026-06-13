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

// ErrNoI2COnDarwin is returned when I2C operations are attempted on macOS.
var ErrNoI2COnDarwin = errors.New("I2C not available on macOS: use external USB-to-I2C adapter")

// DarwinI2C implements I2CDriver as a stub for macOS.
type DarwinI2C struct{}

// NewDarwinI2C creates a new Darwin I2C stub driver.
func NewDarwinI2C() *DarwinI2C {
	return &DarwinI2C{}
}

func (d *DarwinI2C) WriteToDevice(addr int, data byte) error {
	return fmt.Errorf("%w: addr=0x%02x data=0x%02x", ErrNoI2COnDarwin, addr, data)
}

func (d *DarwinI2C) Close() error {
	return nil
}

func init() {
	// Register Darwin I2C driver as default
	defaultI2CDriver = NewDarwinI2C()
}
