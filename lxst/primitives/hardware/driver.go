// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package hardware

import "sync"

var (
	defaultGPIODriver GPIODriver
	defaultI2CDriver  I2CDriver
	driverMu          sync.RWMutex
)

// SetGPIODriver sets the default GPIO driver for the application.
func SetGPIODriver(driver GPIODriver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	defaultGPIODriver = driver
}

// GetGPIODriver returns the default GPIO driver.
func GetGPIODriver() GPIODriver {
	driverMu.RLock()
	defer driverMu.RUnlock()
	return defaultGPIODriver
}

// SetI2CDriver sets the default I2C driver for the application.
func SetI2CDriver(driver I2CDriver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	defaultI2CDriver = driver
}

// GetI2CDriver returns the default I2C driver.
func GetI2CDriver() I2CDriver {
	driverMu.RLock()
	defer driverMu.RUnlock()
	return defaultI2CDriver
}
