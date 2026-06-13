// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/network"
)

func TestNewAudioPipeline(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)
	if ap == nil {
		t.Fatal("NewAudioPipeline returned nil")
	}
}

func TestAudioPipeline_SetupTransmit(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	err = ap.SetupTransmit(func(data []byte) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("SetupTransmit failed: %v", err)
	}

	if ap.Packetizer() == nil {
		t.Error("Packetizer should not be nil after SetupTransmit")
	}
}

func TestAudioPipeline_SetupReceive(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	sr := network.NewSignallingReceiver(nil)
	err = ap.SetupReceive(sr)
	if err != nil {
		t.Fatalf("SetupReceive failed: %v", err)
	}

	if ap.LinkSource() == nil {
		t.Error("LinkSource should not be nil after SetupReceive")
	}
}

func TestAudioPipeline_ReceivePacket(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	sr := network.NewSignallingReceiver(nil)
	err = ap.SetupReceive(sr)
	if err != nil {
		t.Fatalf("SetupReceive failed: %v", err)
	}

	ap.ReceivePacket([]byte{0x00})
}

func TestAudioPipeline_StopWithoutStart(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	ap.Stop()
}
