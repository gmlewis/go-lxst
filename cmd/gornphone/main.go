// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements the gornphone CLI utility.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gmlewis/go-lxst/lxst"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

var (
	version = "0.1.0"
)

func main() {
	log.SetFlags(0)

	listDevices := flag.Bool("l", false, "list available audio devices")
	showVersion := flag.Bool("version", false, "show version")
	configDir := flag.String("config", "", "path to config directory")
	rnsConfigDir := flag.String("rnsconfig", "", "path to Reticulum config directory")
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
			fmt.Fprintf(os.Stderr, "  0x%02x (%s, %s, %.0fms)\n",
				p, telephony.ProfileName(p), telephony.ProfileAbbreviation(p),
				telephony.GetFrameTime(p))
		}
		os.Exit(1)
	}

	cfg := loadOrCreateConfig(*configDir)

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
	} else if cfg.Telephone.Microphone != "" {
		tel.SetMicDevice(cfg.Telephone.Microphone)
	}
	if *speakerFlag != "" {
		tel.SetSpeakerDevice(*speakerFlag)
	} else if cfg.Telephone.Speaker != "" {
		tel.SetSpeakerDevice(cfg.Telephone.Speaker)
	}
	if cfg.Telephone.Ringer != "" {
		tel.SetRingerDevice(cfg.Telephone.Ringer)
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

	fmt.Printf("Audio input:  %s\n", defaultStr(tel.MicDevice(), "default"))
	fmt.Printf("Audio output: %s\n", defaultStr(tel.SpeakerDevice(), "default"))
	fmt.Printf("Auto-answer: %v\n", tel.AutoAnswer())
	fmt.Println()

	identityHash := loadOrCreateIdentityForConfig(*configDir)
	fmt.Printf("Identity hash: %s\n", prettyHex(identityHash))
	fmt.Println()

	if *configDir != "" || cfg != nil {
		fmt.Printf("Config directory: %s\n", *configDir)
	}

	phone := NewPhone(cfg)

	// Initialize RNS transport and endpoint
	// Note: rnsConfigDir can be used with Reticulum's config loading when available
	_ = *rnsConfigDir // Store for future use
	ts := rns.NewTransportSystem(nil)
	identity, err := rns.FromFile(*configDir+"/identity", ts.GetLogger())
	if err != nil {
		identity, err = rns.NewIdentity(true, ts.GetLogger())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating identity: %v\n", err)
			os.Exit(1)
		}
		_ = os.MkdirAll(*configDir, 0o755)
		_ = identity.ToFile(*configDir + "/identity")
	}

	endpoint, err := NewTelephoneEndpoint(identity, ts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating telephone endpoint: %v\n", err)
		os.Exit(1)
	}
	phone.SetEndpoint(endpoint)

	// Wire callbacks
	endpoint.SetOnRinging(func(remoteIdentity *rns.Identity) {
		phone.Ringing(remoteIdentity.HexHash)
	})
	endpoint.SetOnEstablished(func(remoteIdentity *rns.Identity) {
		phone.CallEstablished()
	})
	endpoint.SetOnEnded(func(remoteIdentity *rns.Identity) {
		phone.Hangup()
	})
	endpoint.SetOnBusy(func(remoteIdentity *rns.Identity) {
		phone.Hangup()
	})

	phone.Start()
	phone.printHelp()
	fmt.Println()
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if !phone.ProcessInput(input) {
			break
		}
		fmt.Print("> ")
	}
}

func loadOrCreateConfig(configDir string) *PhoneConfig {
	if configDir == "" {
		configDir = defaultConfigDir()
	}

	configPath := configDir + "/config"
	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		cfg = DefaultConfig()
		_ = os.MkdirAll(configDir, 0o755)
		_ = SaveConfigFile(configPath, cfg)
	}
	return cfg
}

func loadOrCreateIdentityForConfig(configDir string) string {
	if configDir == "" {
		configDir = defaultConfigDir()
	}
	identityPath := configDir + "/identity"
	id, err := loadOrCreateIdentity(identityPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
		os.Exit(1)
	}
	return id.HexHash
}

func defaultRNSConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	configDir := home + "/.reticulum"
	if _, err := os.Stat(configDir); err == nil {
		return configDir
	}
	return ""
}

func defaultConfigDir() string {
	if _, err := os.Stat("/etc/rnphone/config"); err == nil {
		return "/etc/rnphone"
	}
	home, err := os.UserHomeDir()
	if err == nil {
		configDir := home + "/.config/rnphone"
		if _, err := os.Stat(configDir + "/config"); err == nil {
			return configDir
		}
		return home + "/.rnphone"
	}
	return ".rnphone"
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
