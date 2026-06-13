// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements a tone generator example that produces audio
// tones at specified frequencies and plays them through the default
// audio output device.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	codec2pkg "github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/generators"
	"github.com/gmlewis/go-lxst/lxst/sinks"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func main() {
	log.SetFlags(0)

	freq := flag.Float64("f", 440.0, "frequency in Hz")
	gain := flag.Float64("g", 0.1, "gain (linear, 0.0-1.0)")
	duration := flag.Float64("d", 3.0, "duration in seconds (0 = until Ctrl+C)")
	frameMs := flag.Float64("t", 80.0, "frame time in ms")
	channels := flag.Int("c", 1, "number of channels (1=mono, 2=stereo)")
	codec2Mode := flag.Int("codec2", 0, "codec2 mode (0=none, 700, 1200, 1300, 1400, 1600, 2400, 3200)")
	easeIn := flag.Float64("ease", 20.0, "ease-in time in ms")
	listDevices := flag.Bool("l", false, "list audio devices")
	flag.Parse()

	if *listDevices {
		listAudioDevices()
		return
	}

	fmt.Printf("Tone Generator\n")
	fmt.Printf("  Frequency: %.1f Hz\n", *freq)
	fmt.Printf("  Gain:      %.2f (linear)\n", *gain)
	fmt.Printf("  Duration:  %.1f s\n", *duration)
	fmt.Printf("  Frame:     %.1f ms\n", *frameMs)
	fmt.Printf("  Channels:  %d\n", *channels)

	var codec codecs.Codec
	if *codec2Mode != 0 {
		c2, err := codec2pkg.NewCodec2(*codec2Mode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating codec2: %v\n", err)
			os.Exit(1)
		}
		codec = c2
		fmt.Printf("  Codec:     Codec2 mode %d\n", *codec2Mode)
	} else {
		codec = codecs.NullCodec{}
		fmt.Printf("  Codec:     NullCodec (raw PCM)\n")
	}

	sink := sinks.NewLineSink("", true, false)
	defer func() {
		if err := sink.Stop(); err != nil {
			log.Printf("Error stopping sink: %v", err)
		}
	}()

	var localSink sources.LocalSource = sink

	tone := generators.NewToneSource(
		*freq,
		*gain,
		true,
		*easeIn,
		*frameMs,
		codec,
		localSink,
		*channels,
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Playing tone... Press Ctrl+C to stop.")

	if err := tone.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting tone: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := tone.Stop(); err != nil {
			log.Printf("Error stopping tone: %v", err)
		}
	}()

	if *duration > 0 {
		select {
		case <-sigCh:
		case <-time.After(time.Duration(*duration * float64(time.Second))):
		}
	} else {
		<-sigCh
	}

	fmt.Println("\nStopped.")
}

func listAudioDevices() {
	fmt.Println("Listing audio devices:")
	backend := sinks.NewLineSink("", true, false)
	speakers := backend.AvailableSpeakers()
	fmt.Printf("  Speakers (%d):\n", len(speakers))
	for _, s := range speakers {
		fmt.Printf("    - %s\n", s)
	}
	_ = backend
}
