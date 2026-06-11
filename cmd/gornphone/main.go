// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements the gornphone CLI utility.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gmlewis/go-lxst/lxst"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
)

var (
	version = "0.1.0"
)

func main() {
	listDevices := flag.Bool("l", false, "list available audio devices")
	showVersion := flag.Bool("version", false, "show version")
	configDir := flag.String("config", "", "path to config directory")
	profileFlag := flag.Int("profile", int(telephony.DefaultProfile), "audio profile (hex)")
	gainFlag := flag.Float64("gain", 0.0, "receive gain in dB")
	micFlag := flag.String("mic", "", "microphone device name")
	speakerFlag := flag.String("speaker", "", "speaker device name")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gornphone %s\n", version)
		os.Exit(0)
	}

	if *listDevices {
		listAudioDevices()
		os.Exit(0)
	}

	profile := byte(*profileFlag)
	if !isValidProfile(profile) {
		fmt.Fprintf(os.Stderr, "Invalid profile: 0x%02x. Use one of:\n", profile)
		for _, p := range telephony.AvailableProfiles {
			fmt.Fprintf(os.Stderr, "  0x%02x (%s, %s, %.0fms)\n", p, telephony.ProfileName(p), telephony.ProfileAbbreviation(p), telephony.GetFrameTime(p))
		}
		os.Exit(1)
	}

	fmt.Printf("gornphone %s\n", version)
	fmt.Printf("Profile: %s (%s)\n", telephony.ProfileName(profile), telephony.ProfileAbbreviation(profile))
	fmt.Printf("Frame time: %.0fms\n", telephony.GetFrameTime(profile))

	codec, err := telephony.GetCodec(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating codec: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Codec: %T\n", codec)

	tel := telephony.NewTelephone(
		telephony.RingTime,
		telephony.WaitTime,
		true,
		telephony.AllowAll,
		*gainFlag,
		0.0,
	)

	if *micFlag != "" {
		tel.SetMicDevice(*micFlag)
	}
	if *speakerFlag != "" {
		tel.SetSpeakerDevice(*speakerFlag)
	}

	fmt.Printf("Audio input:  %s\n", defaultStr(tel.MicDevice(), "default"))
	fmt.Printf("Audio output: %s\n", defaultStr(tel.SpeakerDevice(), "default"))
	fmt.Printf("Auto-answer: %v\n", tel.AutoAnswer())
	fmt.Println()

	if *configDir != "" {
		fmt.Printf("Config directory: %s\n", *configDir)
	}

	fmt.Println("Available commands:")
	fmt.Println("  p - phonebook")
	fmt.Println("  r - redial last called")
	fmt.Println("  i - show identity")
	fmt.Println("  a - announce on network")
	fmt.Println("  q - quit")
	fmt.Println("  h - help")
	fmt.Println()
	fmt.Println("Enter identity hash to call, or command:")
	fmt.Print("> ")
}

func listAudioDevices() {
	fmt.Println("\nAvailable audio devices:")
	backend := lxst.NewBackend(48000, 2, 32)
	if backend != nil {
		for _, mic := range backend.AllMicrophones() {
			fmt.Printf("  Input  : %s\n", mic)
		}
		for _, spk := range backend.AllSpeakers() {
			fmt.Printf("  Output : %s\n", spk)
		}
	}
}

func isValidProfile(profile byte) bool {
	for _, p := range telephony.AvailableProfiles {
		if p == profile {
			return true
		}
	}
	return false
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// formatHex formats a byte slice as hex with colons.
func formatHex(data []byte) string {
	parts := make([]string, len(data))
	for i, b := range data {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}
