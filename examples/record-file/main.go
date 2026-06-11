// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements a file recording example that captures audio
// from the default microphone and saves it to a file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gmlewis/go-lxst/lxst/primitives/recorders"
)

func main() {
	log.SetFlags(0)

	output := flag.String("o", "recording.opus", "output file path")
	device := flag.String("d", "", "preferred microphone device name")
	profile := flag.Int("profile", 0, "codec profile (0=voice low, hex)")
	gain := flag.Float64("g", 0.0, "gain in dB")
	duration := flag.Float64("d", 0, "duration in seconds (0 = until Ctrl+C)")
	flag.Parse()

	fmt.Printf("Record File\n")
	fmt.Printf("  Output:  %s\n", *output)
	fmt.Printf("  Device:  %s\n", defaultStr(*device, "default"))
	fmt.Printf("  Profile: 0x%02x\n", *profile)
	fmt.Printf("  Gain:    %.1f dB\n", *gain)

	rec := recorders.NewFileRecorder(*output, *device, *profile, *gain, 0.1, 0.0)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Recording... Press Ctrl+C to stop.")

	if err := rec.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting recording: %v\n", err)
		os.Exit(1)
	}

	if *duration > 0 {
		select {
		case <-sigCh:
		case <-time.After(time.Duration(*duration * float64(time.Second))):
		}
	} else {
		<-sigCh
	}

	rec.Stop()
	fmt.Printf("\nRecording saved to: %s\n", *output)
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}