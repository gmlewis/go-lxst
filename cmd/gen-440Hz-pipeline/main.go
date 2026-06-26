// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// gen-440Hz-pipeline simulates the fixed packet path used by gornphone-echo:
// encode with a 24kHz Opus encoder → prepend codec header byte → decode
// with a matching 24kHz decoder (the fix) → play through PortAudio.
//
// Usage:
//
//	go run ./cmd/gen-440Hz-pipeline
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
	frameMs := 60.0

	// Create encoder and decoder at the same profile (VOICE_MEDIUM, 24kHz).
	// This matches the fix: LinkSource now uses the negotiated codec
	// instead of creating a fresh 8kHz decoder from CodecTypeFromHeader.
	encoder, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewOpus encoder failed: %v\n", err)
		os.Exit(1)
	}
	decoder, err := opus.NewOpus(opus.PROFILE_VOICE_MEDIUM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewOpus decoder failed: %v\n", err)
		os.Exit(1)
	}
	sampleRate := encoder.PreferredSampleRate()
	spf := int(float64(sampleRate) * frameMs / 1000.0)

	fmt.Printf("Pipeline test (fixed): encode@%vHz → header byte → decode@%vHz → play\n",
		sampleRate, decoder.PreferredSampleRate())
	fmt.Printf("  Tone: %v Hz, spf=%v (%.0fms)\n", freq, spf, frameMs)
	fmt.Println("Press Ctrl-C to stop.")

	// Create PortAudio player at the codec's native sample rate.
	backend := platforms.NewBackendWithDevice(sampleRate, channels, 32, "")
	fmt.Printf("  Backend: %T\n", backend)

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

		// Generate tone at the codec's sample rate.
		tone := make([][]float32, spf)
		for i := 0; i < spf; i++ {
			tone[i] = []float32{float32(gain * math.Sin(phase))}
			phase += phaseInc
		}
		phase = math.Mod(phase, 2.0*math.Pi)

		// Encode with Opus.
		encoded := encoder.Encode(tone)
		if len(encoded) == 0 {
			fmt.Fprintf(os.Stderr, "Encode returned empty\n")
			os.Exit(1)
		}

		// Prepend codec header byte (what Packetizer does).
		_ = append([]byte{0x01}, encoded...)

		// Decode with the matching decoder (24kHz, not 8kHz).
		decoded := decoder.Decode(encoded, channels)
		if len(decoded) == 0 {
			fmt.Fprintf(os.Stderr, "Decode returned empty\n")
			os.Exit(1)
		}

		// Play the decoded audio through PortAudio.
		if err := player.Play(decoded); err != nil {
			fmt.Fprintf(os.Stderr, "Play failed: %v\n", err)
			os.Exit(1)
		}
	}
}
