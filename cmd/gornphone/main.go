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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gmlewis/go-lxst/lxst"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
	"github.com/gmlewis/go-reticulum/rns/interfaces"
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
	startupMilli := time.Now().UnixMilli()
	logPath := fmt.Sprintf("/tmp/gornphone-%v.log", startupMilli)

	// Redirect Go's log package to the log file so that go-reticulum's
	// internal log.Printf calls don't pollute the terminal. We use a
	// reopening writer that opens the file on each Write call, matching
	// the RNS logger's approach so both survive log rotation.
	log.SetFlags(0)
	log.SetOutput(&reopeningWriter{path: logPath})

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
	listenFlag := flag.String("listen", "", "listen for local TCP connections (host:port, e.g. localhost:4242)")
	connectFlag := flag.String("connect", "", "connect to local TCP server (host:port, e.g. localhost:4242)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gornphone %v\n", version)
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
			fmt.Fprintf(&buf, "  0x%02x (%v, %v, %.0fms)\n",
				p, telephony.ProfileName(p), telephony.ProfileAbbreviation(p),
				telephony.GetFrameTime(p))
		}
		fmt.Fprintf(os.Stderr, "%v", buf.String())
		os.Exit(1)
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

	fmt.Printf("gornphone %v\n", version)
	fmt.Printf("Profile: %v (%v)\n", telephony.ProfileName(profile), telephony.ProfileAbbreviation(profile))
	fmt.Printf("Frame time: %.0fms\n", telephony.GetFrameTime(profile))

	codec, err := telephony.GetCodec(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating codec: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Codec: %T\n", codec)

	fmt.Printf("Audio input:  %v\n", defaultStr(tel.MicDevice(), "default"))
	fmt.Printf("Audio output: %v\n", defaultStr(tel.SpeakerDevice(), "default"))
	fmt.Printf("Auto-answer: %v\n", tel.AutoAnswer())
	fmt.Println()

	identity, err := loadOrCreateIdentity(*configDir + "/identity")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Identity hash: %v\n", formatHash(identity.HexHash))
	fmt.Println()

	if *configDir != "" || cfg != nil {
		fmt.Printf("Config directory: %v\n", *configDir)
	}

	rnsLogger := rns.NewLogger()
	rnsLogger.SetLogFilePath(logPath)
	rnsLogger.SetLogDest(rns.LogDestFile)
	rnsLogger.SetLogLevel(rns.LogInfo)
	logBoth(rnsLogger, "gornphone %v starting, log file: %v", version, logPath)

	phone := NewPhone(cfg, rnsLogger)

	// Initialize RNS transport and endpoint.
	// We pass "" so go-reticulum resolves the default config dir
	// (~/.reticulum), but override share_instance = No so each
	// gornphone runs its own standalone RNS stack with its own
	// destinations registered locally. Shared instance mode doesn't
	// work for gornphone because each instance's destinations are
	// registered on different TransportSystems, so incoming link
	// requests can't be routed to the correct destination.
	rnsConfig := *rnsConfigDir
	if rnsConfig == "" {
		rnsConfig = ensureStandaloneRNSConfig(startupMilli)
	}

	ts := rns.NewTransportSystem(rnsLogger)

	reticulum, err := rns.NewReticulumWithLogger(ts, rnsConfig, rnsLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing Reticulum: %v\n", err)
		os.Exit(1)
	}

	// Ensure log level is at least Info after RNS config may have changed it.
	// RNS config may set a lower log level, but gornphone needs Info for
	// call lifecycle messages.
	if rnsLogger.GetLogLevel() < rns.LogInfo {
		rnsLogger.SetLogLevel(rns.LogInfo)
	}
	logBoth(rnsLogger, "gornphone %v initialized, RNS config: %v", version, rnsConfig)

	defer func() {
		if err := reticulum.Close(); err != nil {
			logBoth(rnsLogger, "reticulum.Close: %v", err)
		}
	}()

	rnsConfigDisplay := rnsConfig
	if rnsConfigDisplay == "" {
		rnsConfigDisplay = "default (~/.reticulum)"
	}
	fmt.Printf("RNS config:   %v\n", rnsConfigDisplay)
	fmt.Printf("RNS log:       %v\n", logPath)

	switch {
	case reticulum.IsSharedInstance():
		fmt.Println("RNS mode:     shared instance server")
	case reticulum.IsConnectedToSharedInstance():
		fmt.Println("RNS mode:     connected to shared instance")
	default:
		fmt.Println("RNS mode:     standalone")
	}

	if *listenFlag != "" || *connectFlag != "" {
		handler := func(data []byte, iface interfaces.Interface) {
			ts.Inbound(data, iface)
		}
		if *listenFlag != "" {
			host, port := parseHostPort(*listenFlag, "localhost", 4242)
			srv, err := interfaces.NewTCPServerInterface("Local TCP Server", host, port, handler, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating local TCP server: %v\n", err)
				os.Exit(1)
			}
			ts.RegisterInterface(srv)
			fmt.Printf("Local TCP:    listening on %v:%v\n", host, port)
		}
		if *connectFlag != "" {
			host, port := parseHostPort(*connectFlag, "localhost", 4242)
			cli, err := interfaces.NewTCPClientInterface("Local TCP Client", host, port, false, handler)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating local TCP client: %v\n", err)
				os.Exit(1)
			}
			ts.RegisterInterface(cli)
			fmt.Printf("Local TCP:    connecting to %v:%v\n", host, port)
		}
	}

	endpoint, err := NewTelephoneEndpoint(identity, ts, rnsLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating telephone endpoint: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Destination hash: %v\n", endpoint.DestinationHash())
	endpoint.SetTelephone(tel)
	tel.SetProfile(profile)
	phone.SetEndpoint(endpoint)

	// Wire callbacks
	endpoint.SetOnRinging(func(remoteIdentity *rns.Identity) {
		hash := "<unknown>"
		if remoteIdentity != nil {
			hash = remoteIdentity.HexHash
		}
		phone.Ringing(hash)
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
		fmt.Printf("Announce failed: %v\n", err)
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
		return home + ".rnphone"
	}
	return ".rnphone"
}

// ensureStandaloneRNSConfig creates a per-gornphone RNS config directory
// with share_instance = No and inherits interface definitions from the
// system ~/.reticulum/config. Each gornphone runs its own standalone RNS
// stack so that its destinations are registered on its own TransportSystem
// and incoming link requests can be routed correctly. Shared instance mode
// doesn't work because each instance's destinations live on different
// TransportSystems and the server can't route link requests to a client's
// destinations.
func ensureStandaloneRNSConfig(startupMilli int64) string {
	rnsDir := fmt.Sprintf("/tmp/gornphone-rns-%v", startupMilli)
	configPath := rnsDir + "/config"

	_ = os.MkdirAll(rnsDir, 0o755)

	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	var content string
	if home != "" {
		systemConfigPath := home + "/.reticulum/config"
		if data, err := os.ReadFile(systemConfigPath); err == nil {
			content = string(data)
		}
	}

	if content == "" {
		content = `[reticulum]
  share_instance = No

[logging]
  loglevel = 4

[interfaces]
  [[Default Interface]]
    type = AutoInterface
    enabled = Yes
    name = Default Interface
`
	}

	content = setRNSConfigDirective(content, "share_instance", "No")

	_ = os.WriteFile(configPath, []byte(content), 0o644)
	return rnsDir
}

// setRNSConfigDirective replaces or adds a key=value directive in the
// [reticulum] section of an RNS config string.
func setRNSConfigDirective(content, key, value string) string {
	lines := strings.Split(content, "\n")
	inReticulum := false
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[[[") {
			continue
		}
		if strings.HasPrefix(trimmed, "[[") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[] ")
			inReticulum = section == "reticulum"
			continue
		}
		if inReticulum {
			if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
				continue
			}
			k := trimmed
			if eq := strings.Index(trimmed, "="); eq >= 0 {
				k = strings.TrimSpace(trimmed[:eq])
			}
			if k == key {
				indent := line[:strings.Index(line, trimmed)]
				lines[i] = indent + key + " = " + value
				replaced = true
			}
		}
	}
	if !replaced {
		lines = append([]string{"[reticulum]", "  " + key + " = " + value, ""}, lines...)
	}
	return strings.Join(lines, "\n")
}

func listAudioDevices() {
	fmt.Println("\nAvailable audio devices:")
	backend := lxst.NewBackend(48000, 2, 32)
	if backend != nil {
		for _, mic := range backend.AllMicrophones() {
			fmt.Printf("  Input  : %v\n", mic)
		}
		for _, spk := range backend.AllSpeakers() {
			fmt.Printf("  Output : %v\n", spk)
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

func parseHostPort(addr, defaultHost string, defaultPort int) (string, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return defaultHost, defaultPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, defaultPort
	}
	if host == "" {
		host = defaultHost
	}
	return host, port
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
Environment="XAUTHORITY=/home/%v/.Xauthority"
Environment="XDG_RUNTIME_DIR=/run/user/1000"
Restart=always
RestartSec=5
User=%v
ExecStart=/home/%v/.local/bin/gornphone --service -vvv

[Install]
WantedBy=graphical.target
`

// logBoth logs to the RNS logger.
func logBoth(logger *rns.Logger, format string, args ...any) {
	if logger != nil {
		logger.Info(format, args...)
	}
}

// reopeningWriter is an io.Writer that opens the file on each Write call.
// This matches the RNS logger's approach and survives log file rotation:
// after the RNS logger renames the file, the next Write opens the new one.
type reopeningWriter struct {
	path string
}

func (w *reopeningWriter) Write(p []byte) (n int, err error) {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	return f.Write(p)
}
