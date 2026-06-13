// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build linux

package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	i2cBasePath = "/dev"
)

// LinuxI2C implements I2CDriver using Linux I2C device interface.
type LinuxI2C struct {
	mu      sync.Mutex
	busPath string
	fd      *os.File
	bus     int
}

// NewLinuxI2C creates a new Linux I2C driver for the specified bus.
func NewLinuxI2C(bus int) *LinuxI2C {
	return &LinuxI2C{
		busPath: filepath.Join(i2cBasePath, fmt.Sprintf("i2c-%d", bus)),
		bus:     bus,
	}
}

func (d *LinuxI2C) WriteToDevice(addr int, data byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.fd == nil {
		if err := d.open(); err != nil {
			return err
		}
	}

	// Use i2c-dev interface for writing
	// This is a simplified implementation; real I2C would use ioctl
	buf := []byte{byte(addr), data}
	_, err := d.fd.Write(buf)
	if err != nil {
		return fmt.Errorf("I2C write to address 0x%02x: %w", addr, err)
	}

	return nil
}

func (d *LinuxI2C) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.fd != nil {
		err := d.fd.Close()
		d.fd = nil
		return err
	}
	return nil
}

func (d *LinuxI2C) open() error {
	var err error
	d.fd, err = os.OpenFile(d.busPath, os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening I2C bus %d: %w", d.bus, err)
	}
	return nil
}

func init() {
	// Register Linux I2C driver as default
	defaultI2CDriver = NewLinuxI2C(1)
}
