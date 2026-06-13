// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *PhoneConfig
		wantErr bool
	}{
		{
			name: "empty config",
			input: `# This is a comment
[telephone]
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
				},
			},
		},
		{
			name: "full config",
			input: `[telephone]
    ringtone = ringer.opus
    speaker = Built-in Output
    microphone = Built-in Input
    ringer = Headphones
    allowed_callers = all
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					Ringtone:        "ringer.opus",
					Speaker:         "Built-in Output",
					Microphone:      "Built-in Input",
					Ringer:          "Headphones",
					AllowedCallers:  "all",
				},
			},
		},
		{
			name: "none allowed",
			input: `[telephone]
    allowed_callers = none
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "none",
				},
			},
		},
		{
			name: "phonebook allowed",
			input: `[telephone]
    allowed_callers = phonebook
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers:  "phonebook",
					AllowPhonebook:  true,
				},
			},
		},
		{
			name: "specific allowed callers",
			input: `[telephone]
    allowed_callers = b8d80b1b7a9d3147880b366995422a45, fcfb80d4cd3aab7c8710541fb2317974
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "list",
					AllowedList:    []string{"b8d80b1b7a9d3147880b366995422a45", "fcfb80d4cd3aab7c8710541fb2317974"},
				},
			},
		},
		{
			name: "blocked callers",
			input: `[telephone]
    blocked_callers = f3e8c3359b39d36f3baff0a616a73d3e, 5d2d14619dfa0ff06278c17347c14331
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
					BlockedList:    []string{"f3e8c3359b39d36f3baff0a616a73d3e", "5d2d14619dfa0ff06278c17347c14331"},
				},
			},
		},
		{
			name: "phonebook entries",
			input: `[phonebook]
    Mary = f3e8c3359b39d36f3baff0a616a73d3e
    Jake = b8d80b1b7a9d3147880b366995422a45
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
				},
				Phonebook: map[string]PhonebookEntry{
					"Mary": {Hash: "f3e8c3359b39d36f3baff0a616a73d3e"},
					"Jake": {Hash: "b8d80b1b7a9d3147880b366995422a45"},
				},
			},
		},
		{
			name: "phonebook with alias",
			input: `[phonebook]
    Rudy = 5d2d14619dfa0ff06278c17347c14331, 241
    Josh = fcfb80d4cd3aab7c8710541fb2317974, 7907
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
				},
				Phonebook: map[string]PhonebookEntry{
					"Rudy": {Hash: "5d2d14619dfa0ff06278c17347c14331", Alias: "241"},
					"Josh": {Hash: "fcfb80d4cd3aab7c8710541fb2317974", Alias: "7907"},
				},
			},
		},
		{
			name: "hardware config",
			input: `[hardware]
    keypad = gpio_4x4
    display = i2c_lcd1602
    keypad_hook_pin = 5
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
				},
				Hardware: HardwareConfig{
					Keypad:        "gpio_4x4",
					Display:       "i2c_lcd1602",
					KeypadHookPin: 5,
				},
			},
		},
		{
			name: "single blocked caller",
			input: `[telephone]
    blocked_callers = f3e8c3359b39d36f3baff0a616a73d3e
`,
			want: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
					BlockedList:    []string{"f3e8c3359b39d36f3baff0a616a73d3e"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseConfig([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				t.Fatal("ParseConfig() returned nil")
			}
			if got.Telephone.Ringtone != tt.want.Telephone.Ringtone {
				t.Errorf("Ringtone = %q, want %q", got.Telephone.Ringtone, tt.want.Telephone.Ringtone)
			}
			if got.Telephone.Speaker != tt.want.Telephone.Speaker {
				t.Errorf("Speaker = %q, want %q", got.Telephone.Speaker, tt.want.Telephone.Speaker)
			}
			if got.Telephone.Microphone != tt.want.Telephone.Microphone {
				t.Errorf("Microphone = %q, want %q", got.Telephone.Microphone, tt.want.Telephone.Microphone)
			}
			if got.Telephone.Ringer != tt.want.Telephone.Ringer {
				t.Errorf("Ringer = %q, want %q", got.Telephone.Ringer, tt.want.Telephone.Ringer)
			}
			if got.Telephone.AllowedCallers != tt.want.Telephone.AllowedCallers {
				t.Errorf("AllowedCallers = %q, want %q", got.Telephone.AllowedCallers, tt.want.Telephone.AllowedCallers)
			}
			if got.Telephone.AllowPhonebook != tt.want.Telephone.AllowPhonebook {
				t.Errorf("AllowPhonebook = %v, want %v", got.Telephone.AllowPhonebook, tt.want.Telephone.AllowPhonebook)
			}
			if len(got.Telephone.AllowedList) != len(tt.want.Telephone.AllowedList) {
				t.Errorf("AllowedList len = %d, want %d", len(got.Telephone.AllowedList), len(tt.want.Telephone.AllowedList))
			} else {
				for i, v := range got.Telephone.AllowedList {
					if v != tt.want.Telephone.AllowedList[i] {
						t.Errorf("AllowedList[%d] = %q, want %q", i, v, tt.want.Telephone.AllowedList[i])
					}
				}
			}
			if len(got.Telephone.BlockedList) != len(tt.want.Telephone.BlockedList) {
				t.Errorf("BlockedList len = %d, want %d", len(got.Telephone.BlockedList), len(tt.want.Telephone.BlockedList))
			} else {
				for i, v := range got.Telephone.BlockedList {
					if v != tt.want.Telephone.BlockedList[i] {
						t.Errorf("BlockedList[%d] = %q, want %q", i, v, tt.want.Telephone.BlockedList[i])
					}
				}
			}
			if len(got.Phonebook) != len(tt.want.Phonebook) {
				t.Errorf("Phonebook len = %d, want %d", len(got.Phonebook), len(tt.want.Phonebook))
			} else {
				for k, v := range tt.want.Phonebook {
					gotEntry, ok := got.Phonebook[k]
					if !ok {
						t.Errorf("Phonebook missing key %q", k)
						continue
					}
					if gotEntry.Hash != v.Hash {
						t.Errorf("Phonebook[%q].Hash = %q, want %q", k, gotEntry.Hash, v.Hash)
					}
					if gotEntry.Alias != v.Alias {
						t.Errorf("Phonebook[%q].Alias = %q, want %q", k, gotEntry.Alias, v.Alias)
					}
				}
			}
			if got.Hardware.Keypad != tt.want.Hardware.Keypad {
				t.Errorf("Hardware.Keypad = %q, want %q", got.Hardware.Keypad, tt.want.Hardware.Keypad)
			}
			if got.Hardware.Display != tt.want.Hardware.Display {
				t.Errorf("Hardware.Display = %q, want %q", got.Hardware.Display, tt.want.Hardware.Display)
			}
			if got.Hardware.KeypadHookPin != tt.want.Hardware.KeypadHookPin {
				t.Errorf("Hardware.KeypadHookPin = %d, want %d", got.Hardware.KeypadHookPin, tt.want.Hardware.KeypadHookPin)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()
	got := DefaultConfig()
	if got == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if got.Telephone.AllowedCallers != "all" {
		t.Errorf("AllowedCallers = %q, want %q", got.Telephone.AllowedCallers, "all")
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	configPath := filepath.Join(dir, "config")
	content := `[telephone]
    ringtone = soft.opus
    speaker = Built-in Output
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}
	if got.Telephone.Ringtone != "soft.opus" {
		t.Errorf("Ringtone = %q, want %q", got.Telephone.Ringtone, "soft.opus")
	}
	if got.Telephone.Speaker != "Built-in Output" {
		t.Errorf("Speaker = %q, want %q", got.Telephone.Speaker, "Built-in Output")
	}
}

func TestSaveConfig(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	configPath := filepath.Join(dir, "config")

	cfg := &PhoneConfig{
		Telephone: TelephoneConfig{
			Ringtone:        "ringer.opus",
			Speaker:         "Built-in Output",
			Microphone:      "Built-in Input",
			AllowedCallers:  "all",
		},
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344"},
		},
	}

	if err := SaveConfigFile(configPath, cfg); err != nil {
		t.Fatalf("SaveConfigFile() error = %v", err)
	}

	loaded, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if loaded.Telephone.Ringtone != "ringer.opus" {
		t.Errorf("Ringtone = %q, want %q", loaded.Telephone.Ringtone, "ringer.opus")
	}
	if loaded.Telephone.Speaker != "Built-in Output" {
		t.Errorf("Speaker = %q, want %q", loaded.Telephone.Speaker, "Built-in Output")
	}
	if loaded.Telephone.Microphone != "Built-in Input" {
		t.Errorf("Microphone = %q, want %q", loaded.Telephone.Microphone, "Built-in Input")
	}
	if loaded.Telephone.AllowedCallers != "all" {
		t.Errorf("AllowedCallers = %q, want %q", loaded.Telephone.AllowedCallers, "all")
	}
	if len(loaded.Phonebook) != 1 {
		t.Fatalf("Phonebook len = %d, want 1", len(loaded.Phonebook))
	}
	alice, ok := loaded.Phonebook["Alice"]
	if !ok {
		t.Fatal("Phonebook missing key Alice")
	}
	if alice.Hash != "aabbccdd11223344aabbccdd11223344" {
		t.Errorf("Alice.Hash = %q, want %q", alice.Hash, "aabbccdd11223344aabbccdd11223344")
	}
}

func TestIsCallerAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *PhoneConfig
		hash    string
		want    bool
	}{
		{
			name: "allow all",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{AllowedCallers: "all"},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: true,
		},
		{
			name: "allow none",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{AllowedCallers: "none"},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: false,
		},
		{
			name: "in allowed list",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "list",
					AllowedList:    []string{"aabbccdd11223344aabbccdd11223344"},
				},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: true,
		},
		{
			name: "not in allowed list",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "list",
					AllowedList:    []string{"aabbccdd11223344aabbccdd11223344"},
				},
			},
			hash: "11223344aabbccdd11223344aabbccdd",
			want: false,
		},
		{
			name: "in blocked list",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
					BlockedList:    []string{"aabbccdd11223344aabbccdd11223344"},
				},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: false,
		},
		{
			name: "allowed but blocked",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "all",
					BlockedList:    []string{"aabbccdd11223344aabbccdd11223344"},
				},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: false,
		},
		{
			name: "phonebook allowed",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "phonebook",
					AllowPhonebook: true,
				},
				Phonebook: map[string]PhonebookEntry{
					"Alice": {Hash: "aabbccdd11223344aabbccdd11223344"},
				},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: true,
		},
		{
			name: "phonebook allowed but not in phonebook",
			cfg: &PhoneConfig{
				Telephone: TelephoneConfig{
					AllowedCallers: "phonebook",
					AllowPhonebook: true,
				},
				Phonebook: map[string]PhonebookEntry{},
			},
			hash: "aabbccdd11223344aabbccdd11223344",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.IsCallerAllowed(tt.hash)
			if got != tt.want {
				t.Errorf("IsCallerAllowed(%q) = %v, want %v", tt.hash, got, tt.want)
			}
		})
	}
}

func TestPhonebookLookup(t *testing.T) {
	t.Parallel()

	cfg := &PhoneConfig{
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344", Alias: "100"},
			"Bob":   {Hash: "11223344aabbccdd11223344aabbccdd"},
		},
	}

	tests := []struct {
		name     string
		hash     string
		wantName string
		wantOk   bool
	}{
		{
			name:     "find alice",
			hash:     "aabbccdd11223344aabbccdd11223344",
			wantName: "Alice",
			wantOk:   true,
		},
		{
			name:     "find bob",
			hash:     "11223344aabbccdd11223344aabbccdd",
			wantName: "Bob",
			wantOk:   true,
		},
		{
			name:   "not found",
			hash:   "deadbeef00000000deadbeef00000000",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotAlias, gotOk := cfg.LookupHash(tt.hash)
			if gotOk != tt.wantOk {
				t.Errorf("LookupHash(%q) ok = %v, want %v", tt.hash, gotOk, tt.wantOk)
			}
			if gotOk && gotName != tt.wantName {
				t.Errorf("LookupHash(%q) name = %q, want %q", tt.hash, gotName, tt.wantName)
			}
			if gotOk && tt.wantName == "Alice" && gotAlias != "100" {
				t.Errorf("LookupHash(%q) alias = %q, want %q", tt.hash, gotAlias, "100")
			}
		})
	}
}

func TestPhonebookLookupByAlias(t *testing.T) {
	t.Parallel()

	cfg := &PhoneConfig{
		Phonebook: map[string]PhonebookEntry{
			"Alice": {Hash: "aabbccdd11223344aabbccdd11223344", Alias: "100"},
			"Bob":   {Hash: "11223344aabbccdd11223344aabbccdd", Alias: "200"},
		},
	}

	tests := []struct {
		name     string
		alias    string
		wantHash string
		wantName string
		wantOk   bool
	}{
		{
			name:     "find by alias 100",
			alias:    "100",
			wantHash: "aabbccdd11223344aabbccdd11223344",
			wantName: "Alice",
			wantOk:   true,
		},
		{
			name:     "find by alias 200",
			alias:    "200",
			wantHash: "11223344aabbccdd11223344aabbccdd",
			wantName: "Bob",
			wantOk:   true,
		},
		{
			name:   "alias not found",
			alias:  "999",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotHash, gotName, gotOk := cfg.LookupAlias(tt.alias)
			if gotOk != tt.wantOk {
				t.Errorf("LookupAlias(%q) ok = %v, want %v", tt.alias, gotOk, tt.wantOk)
			}
			if gotOk {
				if gotHash != tt.wantHash {
					t.Errorf("LookupAlias(%q) hash = %q, want %q", tt.alias, gotHash, tt.wantHash)
				}
				if gotName != tt.wantName {
					t.Errorf("LookupAlias(%q) name = %q, want %q", tt.alias, gotName, tt.wantName)
				}
			}
		})
	}
}

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gornphone-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
