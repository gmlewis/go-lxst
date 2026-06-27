// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package main implements the gornphone-echo utility: a standalone
// audio echo service for debugging RNS telephone calls.
//
// gornphone-echo announces a unique identity on the RNS network,
// auto-answers any incoming call, generates a continuous test tone,
// and echoes back all received audio after a configurable delay.
// It uses no audio hardware — everything runs in-memory, making it
// ideal for debugging call setup and audio pipeline issues.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gmlewis/go-lxst/lxst"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
	"github.com/gmlewis/go-reticulum/rns/interfaces"
)

var version = lxst.VERSION

type verbosity int

func (v *verbosity) String() string   { return strconv.Itoa(int(*v)) }
func (v *verbosity) Set(string) error { *v++; return nil }
func (v *verbosity) IsBoolFlag() bool { return true }

// expandVerboseArgs converts -vv, -vvv, etc. into repeated -v flags
// so that Go's flag.Parse can handle them, matching Python argparse's
// action="count" behavior for -v, -vv, -vvv.
func expandVerboseArgs(args []string) []string {
	var out []string
	for _, arg := range args {
		if len(arg) >= 3 && arg[0] == '-' {
			vCount := 0
			for i := 1; i < len(arg) && arg[i] == 'v'; i++ {
				vCount++
			}
			if vCount >= 2 && vCount == len(arg)-1 {
				for i := 0; i < vCount; i++ {
					out = append(out, "-v")
				}
				continue
			}
		}
		out = append(out, arg)
	}
	return out
}

func main() {
	log.SetFlags(0)

	showVersion := flag.Bool("version", false, "show version")
	configDir := flag.String("config", "", "path to config directory for identity storage")
	rnsConfigDir := flag.String("rnsconfig", "", "path to Reticulum config directory")
	delayFlag := flag.Float64("delay", 0.5, "echo delay in seconds (floating point)")
	freqFlag := flag.Float64("freq", 440.0, "tone frequency in Hz")
	gainFlag := flag.Float64("gain", 0.15, "tone gain (0.0 to 1.0)")
	profileFlag := flag.Int("profile", int(telephony.DefaultProfile), "audio profile (hex)")
	listenFlag := flag.String("listen", "", "listen for local TCP connections (host:port)")
	connectFlag := flag.String("connect", "", "connect to local TCP server (host:port)")
	standaloneFlag := flag.Bool("standalone", false, "run standalone RNS (for testing on one machine)")
	var verbose verbosity
	flag.Var(&verbose, "v", "increase verbosity (-v, -vv, -vvv)")

	// Pre-process args: convert -vv, -vvv, etc. into repeated -v flags
	// so that flag.Parse sees "-v -v -v" instead of "-vvv".
	os.Args = expandVerboseArgs(os.Args)
	flag.Parse()

	if *showVersion {
		fmt.Printf("gornphone-echo %v\n", version)
		os.Exit(0)
	}

	profile := byte(*profileFlag)
	if !isValidProfile(profile) {
		fmt.Fprintf(os.Stderr, "Invalid profile: 0x%02x\n", profile)
		os.Exit(1)
	}

	delay := time.Duration(*delayFlag * float64(time.Second))

	if *configDir == "" {
		*configDir = defaultConfigDir()
	}
	if err := os.MkdirAll(*configDir, 0o755); err != nil {
		log.Fatalf("os.MkdirAll: %v", err)
	}

	codec, err := telephony.GetCodec(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating codec: %v\n", err)
		os.Exit(1)
	}

	frameMs := telephony.GetFrameTime(profile)

	fmt.Printf("gornphone-echo %v\n", version)
	fmt.Printf("Profile: %v (%v)\n", telephony.ProfileName(profile), telephony.ProfileAbbreviation(profile))
	fmt.Printf("Frame time: %.0fms\n", frameMs)
	fmt.Printf("Codec: %T\n", codec)
	fmt.Printf("Echo delay: %v\n", delay)
	fmt.Printf("Tone: %.1f Hz, gain %.2f (0.5s on, 2s off)\n", *freqFlag, *gainFlag)
	fmt.Println()

	identity, err := loadOrCreateIdentity(*configDir + "/identity")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Identity hash: <%v>\n", identity.HexHash)

	logPath := fmt.Sprintf("%v/gornphone-echo-%v.log", logTempDir(), time.Now().UnixMilli())

	// Redirect Go's log package to the log file so that debug
	// log.Printf calls from the mixer, network, sinks, and sources
	// don't pollute the terminal. Only application-level messages
	// printed via fmt go to stdout/stderr.
	log.SetOutput(&reopeningWriter{path: logPath})

	rnsLogger := rns.NewLogger()
	rnsLogger.SetLogFilePath(logPath)
	rnsLogger.SetLogCallback(func(logString string) {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("os.OpenFile: %v", err)
		}
		if _, err := fmt.Fprintln(f, logString); err != nil {
			log.Fatalf("fmt.Fprintln: %v", err)
		}
		if err := f.Close(); err != nil {
			log.Fatalf("f.Close: %v", err)
		}
		if strings.Contains(logString, "[Error]") || strings.Contains(logString, "[Critical]") {
			fmt.Fprintln(os.Stderr, logString)
		}
	})
	rnsLogger.SetLogDest(rns.LogCallback)

	fmt.Printf("RNS log:       %v\n", logPath)
	fmt.Println()

	// Map -v flags to RNS log levels:
	//   (none) = Notice (3)
	//   -v     = Info (4)
	//   -vv    = Verbose (5)
	//   -vvv   = Debug (6)
	//   -vvvv  = Extreme (7)
	logLevel := rns.LogNotice + int(verbose)
	if logLevel > rns.LogExtreme {
		logLevel = rns.LogExtreme
	}
	rnsLogger.SetLogLevel(logLevel)

	// Build RNS transport.
	rnsConfig := *rnsConfigDir
	if *standaloneFlag {
		rnsConfig = ensureStandaloneRNSConfig()
	}

	ts := rns.NewTransportSystem(rnsLogger)
	reticulum, err := rns.NewReticulumWithLogger(ts, rnsConfig, rnsLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing Reticulum: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = reticulum.Close()
	}()

	if rnsLogger.GetLogLevel() < rns.LogNotice {
		rnsLogger.SetLogLevel(rns.LogNotice)
	}

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

	// Optional local TCP interfaces for same-machine testing.
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

	// Create the telephone endpoint.
	endpoint, err := NewEchoEndpoint(identity, ts, rnsLogger, codec, profile, frameMs, delay, *freqFlag, *gainFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating echo endpoint: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Destination hash: %v\n", endpoint.DestinationHash())
	fmt.Println()

	// Announce on the network.
	if err := endpoint.Announce(); err != nil {
		fmt.Printf("Announce failed: %v\n", err)
	} else {
		fmt.Println("Announce sent. Waiting for incoming calls...")
	}

	endpoint.StartJobs()
	defer endpoint.StopJobs()

	// Run until interrupted.
	select {}
}

func isValidProfile(profile byte) bool {
	for _, p := range telephony.AvailableProfiles {
		if p == profile {
			return true
		}
	}
	return false
}

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err == nil {
		return home + "/.gornphone-echo"
	}
	return ".gornphone-echo"
}

func loadOrCreateIdentity(path string) (*rns.Identity, error) {
	id, err := rns.FromFile(path, nil)
	if err == nil && id != nil {
		return id, nil
	}
	id, err = rns.NewIdentity(true, nil)
	if err != nil {
		return nil, fmt.Errorf("creating identity: %w", err)
	}
	if err := id.ToFile(path); err != nil {
		return nil, fmt.Errorf("saving identity: %w", err)
	}
	return id, nil
}

func parseHostPort(addr, defaultHost string, defaultPort int) (string, int) {
	host, portStr, err := splitHostPort(addr)
	if err != nil {
		return defaultHost, defaultPort
	}
	port, err := atoi(portStr)
	if err != nil {
		return host, defaultPort
	}
	if host == "" {
		host = defaultHost
	}
	return host, port
}

func splitHostPort(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return "", "", fmt.Errorf("no port")
	}
	return addr[:i], addr[i+1:], nil
}

func atoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%v", &n)
	return n, err
}

func logTempDir() string {
	if runtime.GOOS == "windows" {
		return os.TempDir()
	}
	return "/tmp"
}

// ensureStandaloneRNSConfig creates a per-process RNS config with
// share_instance = No, inheriting interface definitions from the
// system ~/.reticulum/config.
func ensureStandaloneRNSConfig() string {
	rnsDir := fmt.Sprintf("%v/gornphone-echo-rns-%v", logTempDir(), time.Now().UnixMilli())
	configPath := rnsDir + "/config"
	if err := os.MkdirAll(rnsDir, 0o755); err != nil {
		log.Fatalf("os.MkdirAll: %v", err)
	}

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
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		log.Fatalf("os.WriteFile: %v", err)
	}
	return rnsDir
}

func setRNSConfigDirective(content, key, value string) string {
	lines := strings.Split(content, "\n")
	inReticulum := false
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[[[") || strings.HasPrefix(trimmed, "[[") {
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

// reopeningWriter is an io.Writer that opens the file on each Write
// call, matching the RNS logger's approach and surviving log rotation.
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
