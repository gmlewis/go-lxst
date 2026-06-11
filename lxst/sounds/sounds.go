// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package sounds provides embedded audio resources for the LXST audio
// processing library. It contains built-in sound files (ringer, soft) that
// are embedded at compile time using Go's embed directive and accessible
// via the GetSound function.
package sounds

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
)

//go:embed ringer.opus soft.opus
var soundFS embed.FS

var soundNames []string

func init() {
	entries, err := soundFS.ReadDir(".")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".opus") {
			name := strings.TrimSuffix(e.Name(), ".opus")
			soundNames = append(soundNames, name)
		}
	}
	sort.Strings(soundNames)
}

// GetSound returns an io.Reader for the embedded sound identified by name
// (without file extension). Valid names include "ringer" and "soft".
// Returns an error if the requested sound does not exist.
func GetSound(name string) (io.Reader, error) {
	filename := name + ".opus"
	data, err := soundFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("sound %q not found: %w", name, err)
	}
	return bytes.NewReader(data), nil
}

// ListSounds returns the names of all available embedded sounds,
// sorted alphabetically.
func ListSounds() []string {
	result := make([]string, len(soundNames))
	copy(result, soundNames)
	return result
}
