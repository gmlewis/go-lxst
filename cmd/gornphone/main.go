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
	"time"

	"github.com/gmlewis/go-lxst/lxst"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

var (
	version = "0.1.0"
)

type verbosity int

func (v *verbosity) String() string { return fmt.Sprintf("%d", *v) }

func (v *verbosity) Set(_ string) error {
	*v++
	return nil
}

func main() {
	log.SetFlags(0)

	listDevices := flag.Bool("l", false, "list available audio devices")
	showVersion := flag.Bool("version", false, "show version")
	configDir := flag.String("config", "", "path to config directory")
	rnsConfigDir := flag.String("rnsconfig", "", "path to Reticulum config directory")
	serviceFlag := flag.Bool("service", false, "run as service (no interactive prompt)")
	systemdFlag := flag.Bool("systemd", false, "display systemd unit file and exit")
	var verbose verbosity
	flag.Var(&verbose, "v", "increase verbosity (-v, -vv, -vvv)")
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

	if *systemdFlag {
		printSystemdUnit()
		os.Exit(0)
	}

	profile := byte(*profileFlag)
	if !isValidProfile(profile) {
		var buf strings.Builder
		fmt.Fprintf(&buf, "Invalid profile: 0x%02x. Use one of:\n", profile)
		for _, p := range telephony.AvailableProfiles {
			fmt.Fprintf(&buf, "  0x%02x (%s, %s, %.0fms)\n",
				p, telephony.ProfileName(p), telephony.ProfileAbbreviation(p),
				telephony.GetFrameTime(p))
		}
		log.Fatal(buf.String())
	}

	if *configDir == "" {
		*configDir = defaultConfigDir()
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
		log.Fatalf("Error creating codec: %v", err)
	}
	fmt.Printf("Codec: %T\n", codec)

	fmt.Printf("Audio input:  %s\n", defaultStr(tel.MicDevice(), "default"))
	fmt.Printf("Audio output: %s\n", defaultStr(tel.SpeakerDevice(), "default"))
	fmt.Printf("Auto-answer: %v\n", tel.AutoAnswer())
	fmt.Println()

	identity, err := loadOrCreateIdentity(*configDir + "/identity")
	if err != nil {
		log.Fatalf("Error loading identity: %v", err)
	}
	fmt.Printf("Identity hash: %s\n", prettyHex(identity.HexHash))
	fmt.Println()

	if *configDir != "" || cfg != nil {
		fmt.Printf("Config directory: %s\n", *configDir)
	}

	phone := NewPhone(cfg)

	// Initialize RNS transport and endpoint
	rnsConfig := *rnsConfigDir
	if rnsConfig == "" {
		rnsConfig = defaultRNSConfigDir()
	}

	logPath := fmt.Sprintf("/tmp/gornphone-%v.log", time.Now().UnixMilli())
	rnsLogger := rns.NewLogger()
	rnsLogger.SetLogFilePath(logPath)
	rnsLogger.SetLogDest(rns.LogDestFile)

	logFile, err := os.Create(logPath)
	if err != nil {
		log.Fatalf("Error creating log file: %v", err)
	}
	log.SetOutput(logFile)

	ts := rns.NewTransportSystem(rnsLogger)

	reticulum, err := rns.NewReticulumWithLogger(ts, rnsConfig, rnsLogger)
	if err != nil {
		log.Fatalf("Error initializing Reticulum: %v", err)
	}
	_ = reticulum

	fmt.Printf("RNS log: %s\n", logPath)

	endpoint, err := NewTelephoneEndpoint(identity, ts)
	if err != nil {
		log.Fatalf("Error creating telephone endpoint: %v", err)
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

	if err := endpoint.Announce(); err != nil {
		log.Printf("Failed to announce on startup: %v", err)
	}

	endpoint.StartJobs()
	defer endpoint.StopJobs()

	if *serviceFlag {
		// Service mode: run announce loop, handle calls automatically
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
	} else {
		// Interactive mode
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
}

func loadOrCreateConfig(configDir string) *PhoneConfig {
	configPath := configDir + "/config"
	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		cfg = DefaultConfig()
		_ = os.MkdirAll(configDir, 0o755)
		_ = SaveConfigFile(configPath, cfg)
	}
	return cfg
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

func printSystemdUnit() {
	username := os.Getenv("USER")
	if username == "" {
		username = "root"
	}

	fmt.Print(`To install gornphone as a system service, paste the
systemd unit configuration below into a new file at:

/etc/systemd/system/gornphone.service

Then enable the service at boot by running:

sudo systemctl enable gornphone

--- begin systemd unit snipped ---

`)
	fmt.Printf(systemdUnitTemplate, username, username, username)
	fmt.Println("---  end systemd unit snipped  ---")
}

const systemdUnitTemplate = `# This systemd unit allows installing gornphone
# as a system service on Linux-based devices
[Unit]
Description=Reticulum Telephone Service
After=sound.target

[Service]
# Wait 30 seconds for WiFi and audio
# hardware to initialise.
ExecStartPre=/bin/sleep 30
Type=simple
Environment="DISPLAY=:0"
Environment="XAUTHORITY=/home/%s/.Xauthority"
Environment="XDG_RUNTIME_DIR=/run/user/1000"
Restart=always
RestartSec=5
User=%s
ExecStart=/home/%s/.local/bin/gornphone --service -vvv

[Install]
WantedBy=graphical.target
`
