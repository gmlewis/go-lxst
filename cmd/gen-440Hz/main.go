// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// gen-440Hz generates a steady 440 Hz sine wave on the default audio
// output device using the PortAudio backend directly. No LXST pipeline,
// no codecs, no mixers — just raw audio through PortAudio.
//
// Usage:
//
//	go run ./cmd/gen-440Hz
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmlewis/go-lxst/lxst/platforms"
)

func main() {
	sampleRate := 48000
	channels := 1
	bitDepth := 32
	freq := 440.0
	gain := 0.3
	spf := 480 // 10ms frames at 48kHz

	fmt.Printf("Generating %v Hz sine wave: %v Hz sample rate, %v ch, spf=%v\n",
		freq, sampleRate, channels, spf)
	fmt.Println("Press Ctrl-C to stop.")

	backend := platforms.NewBackendWithDevice(sampleRate, channels, bitDepth, "")
	fmt.Printf("Backend: %T\n", backend)

	player, err := backend.GetPlayer(spf, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetPlayer failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = player.Close()
		_ = backend.ReleasePlayer()
	}()

	// Handle Ctrl-C for clean shutdown.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	phase := 0.0
	phaseInc := 2.0 * math.Pi * freq / float64(sampleRate)

	for {
		select {
		case <-sig:
			fmt.Println("\nStopped.")
			return
		default:
		}

		frame := make([][]float32, spf)
		for i := 0; i < spf; i++ {
			frame[i] = []float32{float32(gain * math.Sin(phase))}
			phase += phaseInc
		}
		phase = math.Mod(phase, 2.0*math.Pi)

		if err := player.Play(frame); err != nil {
			fmt.Fprintf(os.Stderr, "Play failed: %v\n", err)
			os.Exit(1)
		}
	}
}
