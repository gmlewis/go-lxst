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
	"strconv"
	"strings"
	"sync"
)

const (
	gpioBasePath     = "/sys/class/gpio"
	gpioExportPath   = gpioBasePath + "/export"
	gpioUnexportPath = gpioBasePath + "/unexport"
)

// LinuxGPIO implements GPIODriver using Linux sysfs GPIO interface.
type LinuxGPIO struct {
	mu       sync.Mutex
	exported map[int]bool
}

// NewLinuxGPIO creates a new Linux GPIO driver.
func NewLinuxGPIO() *LinuxGPIO {
	return &LinuxGPIO{
		exported: make(map[int]bool),
	}
}

func (g *LinuxGPIO) Setup(pin int, mode GPIOMode) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.ensureExported(pin); err != nil {
		return fmt.Errorf("exporting pin %d: %w", pin, err)
	}

	pinPath := filepath.Join(gpioBasePath, fmt.Sprintf("gpio%d", pin))
	directionPath := filepath.Join(pinPath, "direction")

	var direction string
	switch mode {
	case GPIOModeInput:
		direction = "in"
	case GPIOModeOutput:
		direction = "out"
	default:
		return fmt.Errorf("invalid GPIO mode: %d", mode)
	}

	if err := os.WriteFile(directionPath, []byte(direction), 0o644); err != nil {
		return fmt.Errorf("setting direction for pin %d: %w", pin, err)
	}

	return nil
}

func (g *LinuxGPIO) SetOutput(pin int, high bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	pinPath := filepath.Join(gpioBasePath, fmt.Sprintf("gpio%d", pin))
	valuePath := filepath.Join(pinPath, "value")

	var value string
	if high {
		value = "1"
	} else {
		value = "0"
	}

	if err := os.WriteFile(valuePath, []byte(value), 0o644); err != nil {
		return fmt.Errorf("setting value for pin %d: %w", pin, err)
	}

	return nil
}

func (g *LinuxGPIO) ReadInput(pin int) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	pinPath := filepath.Join(gpioBasePath, fmt.Sprintf("gpio%d", pin))
	valuePath := filepath.Join(pinPath, "value")

	data, err := os.ReadFile(valuePath)
	if err != nil {
		return false, fmt.Errorf("reading pin %d: %w", pin, err)
	}

	value := strings.TrimSpace(string(data))
	return value == "1", nil
}

func (g *LinuxGPIO) SetPullUpDown(pin int, pull PullUpDown) error {
	pullPath := filepath.Join(gpioBasePath, fmt.Sprintf("gpio%d", pin), "pull")

	var pullStr string
	switch pull {
	case PullNone:
		pullStr = "none"
	case PullUp:
		pullStr = "up"
	case PullDown:
		pullStr = "down"
	default:
		return fmt.Errorf("invalid pull mode: %d", pull)
	}

	if err := os.WriteFile(pullPath, []byte(pullStr), 0o644); err != nil {
		return fmt.Errorf("setting pull for pin %d: %w", pin, err)
	}

	return nil
}

func (g *LinuxGPIO) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for pin := range g.exported {
		if err := g.unexportPin(pin); err != nil {
			return err
		}
	}
	g.exported = make(map[int]bool)
	return nil
}

func (g *LinuxGPIO) ensureExported(pin int) error {
	if g.exported[pin] {
		return nil
	}

	if err := g.exportPin(pin); err != nil {
		return err
	}

	g.exported[pin] = true
	return nil
}

func (g *LinuxGPIO) exportPin(pin int) error {
	return os.WriteFile(gpioExportPath, []byte(strconv.Itoa(pin)), 0o644)
}

func (g *LinuxGPIO) unexportPin(pin int) error {
	return os.WriteFile(gpioUnexportPath, []byte(strconv.Itoa(pin)), 0o644)
}

func init() {
	// Register Linux GPIO driver as default
	defaultGPIODriver = NewLinuxGPIO()
}
