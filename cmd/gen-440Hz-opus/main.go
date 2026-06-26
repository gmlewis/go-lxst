// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// gen-440Hz-opus generates a 440 Hz sine wave, encodes it with Opus,
// decodes it back, and plays the result through PortAudio. This tests
// whether the Opus encode→decode roundtrip introduces distortion.
//
// Usage:
//
//	go run ./cmd/gen-440Hz-opus
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmlewis/go-lxst/lxst/codecs/opus"
	"github.com/gmlewis/go-lxst/lxst/platforms"
)

func main() {
	channels := 1
	freq := 440.0
	gain := 0.3
	frameMs := 60.0 // matches Opus VOICE_MEDIUM frame time

	// Create encoder and decoder with the same profile.
	codec, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewOpus failed: %v\n", err)
		os.Exit(1)
	}
	sampleRate := codec.PreferredSampleRate()
	spf := int(float64(sampleRate) * frameMs / 1000.0)

	fmt.Printf("Opus roundtrip test: %v Hz tone, %v Hz codec rate, spf=%v (%.0fms)\n",
		freq, sampleRate, spf, frameMs)
	fmt.Println("Press Ctrl-C to stop.")

	// Create PortAudio player at the codec's native sample rate.
	backend := platforms.NewBackendWithDevice(sampleRate, channels, 32, "")
	fmt.Printf("Backend: %T (sample rate=%v)\n", backend, sampleRate)

	player, err := backend.GetPlayer(spf, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetPlayer failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = player.Close()
		_ = backend.ReleasePlayer()
	}()

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

		// Generate a tone frame [samples][channels] at the codec's sample rate.
		tone := make([][]float32, spf)
		for i := 0; i < spf; i++ {
			tone[i] = []float32{float32(gain * math.Sin(phase))}
			phase += phaseInc
		}
		phase = math.Mod(phase, 2.0*math.Pi)

		// Encode with Opus.
		encoded := codec.Encode(tone)
		if len(encoded) == 0 {
			fmt.Fprintf(os.Stderr, "Encode returned empty\n")
			os.Exit(1)
		}

		// Decode with Opus.
		decoded := codec.Decode(encoded, channels)
		if len(decoded) == 0 {
			fmt.Fprintf(os.Stderr, "Decode returned empty\n")
			os.Exit(1)
		}

		if len(decoded) != spf {
			fmt.Fprintf(os.Stderr, "Decode returned %v samples, expected %v\n",
				len(decoded), spf)
		}

		// Play the decoded audio.
		if err := player.Play(decoded); err != nil {
			fmt.Fprintf(os.Stderr, "Play failed: %v\n", err)
			os.Exit(1)
		}
	}
}
