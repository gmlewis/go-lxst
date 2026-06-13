// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements a file playback example that reads audio files
// (WAV, MP3, FLAC, Ogg Vorbis) and plays them through the default
// audio output device.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	codec2pkg "github.com/gmlewis/go-lxst/lxst/codecs/codec2"
	"github.com/gmlewis/go-lxst/lxst/codecs/flac"
	"github.com/gmlewis/go-lxst/lxst/codecs/mp3"
	"github.com/gmlewis/go-lxst/lxst/codecs/vorbis"
	"github.com/gmlewis/go-lxst/lxst/sources"
)

func main() {
	log.SetFlags(0)

	loop := flag.Bool("loop", false, "loop playback")
	frameMs := flag.Float64("t", 100.0, "frame time in ms")
	codec2Mode := flag.Int("codec2", 0, "codec2 mode (0=none, 700, 1200, etc.)")
	listDevices := flag.Bool("l", false, "list audio devices")
	flag.Parse()

	if *listDevices {
		fmt.Println("Audio device listing not yet available in play_file.")
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: play_file [options] <audio-file>\n")
		fmt.Fprintf(os.Stderr, "Supported formats: .wav, .mp3, .flac, .ogg\n")
		os.Exit(1)
	}

	filePath := args[0]
	ext := strings.ToLower(filepath.Ext(filePath))

	var codec codecs.Codec
	if *codec2Mode != 0 {
		c2, err := codec2pkg.NewCodec2(*codec2Mode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating codec2: %v\n", err)
			os.Exit(1)
		}
		codec = c2
		fmt.Printf("  Codec: Codec2 mode %d\n", *codec2Mode)
	} else {
		codec = codecs.NullCodec{}
		fmt.Printf("  Codec: NullCodec (raw PCM)\n")
	}

	var src sources.Source
	var err error

	switch ext {
	case ".wav":
		src, err = sources.NewOpusFileSource(filePath, *frameMs, *loop, codec, nil, false)
	case ".mp3":
		src, err = mp3.NewMP3FileSource(filePath, *frameMs, *loop, codec, nil, false)
	case ".flac":
		src, err = flac.NewFLACFileSource(filePath, *frameMs, *loop, codec, nil, false)
	case ".ogg":
		src, err = vorbis.NewVorbisFileSource(filePath, *frameMs, *loop, codec, nil, false)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported file format: %s\n", ext)
		fmt.Fprintf(os.Stderr, "Supported: .wav, .mp3, .flac, .ogg\n")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Playing: %s\n", filePath)
	fmt.Printf("  Loop:    %v\n", *loop)
	fmt.Printf("  Frame:   %.1f ms\n", *frameMs)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Press Ctrl+C to stop.")

	if err := src.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting playback: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := src.Stop(); err != nil {
			log.Printf("Error stopping playback: %v", err)
		}
	}()

	// Wait for signal or completion
	if *loop {
		<-sigCh
	} else {
		select {
		case <-sigCh:
		case <-time.After(30 * time.Second):
			// Safety timeout for non-looping files
		}
	}

	fmt.Println("\nStopped.")
}
