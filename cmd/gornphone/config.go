// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// PhoneConfig holds the parsed configuration for the rnphone utility.
type PhoneConfig struct {
	Telephone TelephoneConfig
	Phonebook map[string]PhonebookEntry
	Hardware  HardwareConfig
}

// TelephoneConfig holds telephone-specific settings.
type TelephoneConfig struct {
	Ringtone       string
	Speaker        string
	Microphone     string
	Ringer         string
	AllowedCallers string
	AllowPhonebook bool
	AllowedList    []string
	BlockedList    []string
}

// PhonebookEntry represents a single phonebook entry.
type PhonebookEntry struct {
	Hash  string
	Alias string
}

// HardwareConfig holds hardware device settings.
type HardwareConfig struct {
	Keypad        string
	Display       string
	KeypadHookPin int
}

// DefaultConfig returns a PhoneConfig with default values.
func DefaultConfig() *PhoneConfig {
	return &PhoneConfig{
		Telephone: TelephoneConfig{
			AllowedCallers: "all",
		},
		Phonebook: make(map[string]PhonebookEntry),
	}
}

var hexPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ParseConfig parses rnphone INI-style configuration from bytes.
func ParseConfig(data []byte) (*PhoneConfig, error) {
	cfg := DefaultConfig()
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[] ")
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "telephone":
			parseTelephoneConfig(cfg, key, value)
		case "phonebook":
			parsePhonebookEntry(cfg, key, value)
		case "hardware":
			parseHardwareConfig(cfg, key, value)
		}
	}

	if cfg.Phonebook == nil {
		cfg.Phonebook = make(map[string]PhonebookEntry)
	}

	return cfg, scanner.Err()
}

func parseTelephoneConfig(cfg *PhoneConfig, key, value string) {
	switch key {
	case "ringtone":
		cfg.Telephone.Ringtone = value
	case "speaker":
		cfg.Telephone.Speaker = value
	case "microphone":
		cfg.Telephone.Microphone = value
	case "ringer":
		cfg.Telephone.Ringer = value
	case "allowed_callers":
		lower := strings.ToLower(value)
		if lower == "all" {
			cfg.Telephone.AllowedCallers = "all"
		} else if lower == "none" {
			cfg.Telephone.AllowedCallers = "none"
		} else if lower == "phonebook" {
			cfg.Telephone.AllowedCallers = "phonebook"
			cfg.Telephone.AllowPhonebook = true
		} else {
			cfg.Telephone.AllowedCallers = "list"
			for _, h := range strings.Split(value, ",") {
				h = strings.TrimSpace(h)
				if hexPattern.MatchString(h) {
					cfg.Telephone.AllowedList = append(cfg.Telephone.AllowedList, h)
				}
			}
		}
	case "blocked_callers":
		for _, h := range strings.Split(value, ",") {
			h = strings.TrimSpace(h)
			if hexPattern.MatchString(h) {
				cfg.Telephone.BlockedList = append(cfg.Telephone.BlockedList, h)
			}
		}
	}
}

func parsePhonebookEntry(cfg *PhoneConfig, name, value string) {
	parts := strings.SplitN(value, ",", 2)
	hash := strings.TrimSpace(parts[0])

	if !hexPattern.MatchString(hash) {
		return
	}

	entry := PhonebookEntry{Hash: hash}
	if len(parts) > 1 {
		alias := strings.TrimSpace(parts[1])
		for _, c := range alias {
			if c >= '0' && c <= '9' {
				continue
			}
			alias = ""
			break
		}
		if alias != "" {
			entry.Alias = alias
		}
	}

	if cfg.Phonebook == nil {
		cfg.Phonebook = make(map[string]PhonebookEntry)
	}
	cfg.Phonebook[name] = entry
}

func parseHardwareConfig(cfg *PhoneConfig, key, value string) {
	switch key {
	case "keypad":
		cfg.Hardware.Keypad = value
	case "display":
		cfg.Hardware.Display = value
	case "keypad_hook_pin":
		pin, err := strconv.Atoi(value)
		if err == nil {
			cfg.Hardware.KeypadHookPin = pin
		}
	}
}

// IsCallerAllowed reports whether an identity hash is permitted to call.
func (cfg *PhoneConfig) IsCallerAllowed(hash string) bool {
	for _, blocked := range cfg.Telephone.BlockedList {
		if blocked == hash {
			return false
		}
	}

	switch cfg.Telephone.AllowedCallers {
	case "all":
		return true
	case "none":
		return false
	case "list":
		for _, allowed := range cfg.Telephone.AllowedList {
			if allowed == hash {
				return true
			}
		}
		return false
	case "phonebook":
		for _, entry := range cfg.Phonebook {
			if entry.Hash == hash {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// LookupHash finds a phonebook entry by its identity hash.
func (cfg *PhoneConfig) LookupHash(hash string) (name, alias string, ok bool) {
	for n, entry := range cfg.Phonebook {
		if entry.Hash == hash {
			return n, entry.Alias, true
		}
	}
	return "", "", false
}

// LookupAlias finds a phonebook entry by its numerical alias.
func (cfg *PhoneConfig) LookupAlias(alias string) (hash, name string, ok bool) {
	for n, entry := range cfg.Phonebook {
		if entry.Alias == alias {
			return entry.Hash, n, true
		}
	}
	return "", "", false
}

// LookupName finds a phonebook entry by its name (case-insensitive).
func (cfg *PhoneConfig) LookupName(name string) (hash, alias string, ok bool) {
	lower := strings.ToLower(name)
	for n, entry := range cfg.Phonebook {
		if strings.ToLower(n) == lower {
			return entry.Hash, entry.Alias, true
		}
	}
	return "", "", false
}

// LoadConfigFile reads and parses a config file from the given path.
func LoadConfigFile(path string) (*PhoneConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return ParseConfig(data)
}

// SaveConfigFile writes the config to the given path in INI format.
func SaveConfigFile(path string, cfg *PhoneConfig) error {
	var sb strings.Builder

	sb.WriteString("[telephone]\n")
	if cfg.Telephone.Ringtone != "" {
		fmt.Fprintf(&sb, "    ringtone = %s\n", cfg.Telephone.Ringtone)
	}
	if cfg.Telephone.Speaker != "" {
		fmt.Fprintf(&sb, "    speaker = %s\n", cfg.Telephone.Speaker)
	}
	if cfg.Telephone.Microphone != "" {
		fmt.Fprintf(&sb, "    microphone = %s\n", cfg.Telephone.Microphone)
	}
	if cfg.Telephone.Ringer != "" {
		fmt.Fprintf(&sb, "    ringer = %s\n", cfg.Telephone.Ringer)
	}
	if cfg.Telephone.AllowedCallers != "" {
		switch cfg.Telephone.AllowedCallers {
		case "all", "none", "phonebook":
			fmt.Fprintf(&sb, "    allowed_callers = %s\n", cfg.Telephone.AllowedCallers)
		case "list":
			fmt.Fprintf(&sb, "    allowed_callers = %s\n", strings.Join(cfg.Telephone.AllowedList, ", "))
		}
	}
	if len(cfg.Telephone.BlockedList) > 0 {
		fmt.Fprintf(&sb, "    blocked_callers = %s\n", strings.Join(cfg.Telephone.BlockedList, ", "))
	}

	if len(cfg.Phonebook) > 0 {
		sb.WriteString("\n[phonebook]\n")
		for name, entry := range cfg.Phonebook {
			if entry.Alias != "" {
				fmt.Fprintf(&sb, "    %s = %s, %s\n", name, entry.Hash, entry.Alias)
			} else {
				fmt.Fprintf(&sb, "    %s = %s\n", name, entry.Hash)
			}
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}
