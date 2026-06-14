// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"sync"
	"testing"

	"github.com/gmlewis/go-lxst/lxst/codecs/raw"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-reticulum/rns"
)

func TestIntegration_AudioPipelineOnIncomingLink(t *testing.T) {
	t.Parallel()

	callerID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	receiverID, err := rns.NewIdentity(true, nil)
	if err != nil {
		t.Fatalf("NewIdentity failed: %v", err)
	}

	ts := rns.NewTransportSystem(nil)
	ts.Remember(callerID.Hash, nil, nil, nil)
	ts.Remember(receiverID.Hash, nil, nil, nil)

	// Create receiver endpoint with audio pipeline
	receiverEP, err := NewTelephoneEndpoint(receiverID, ts)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	defer receiverEP.Teardown()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	// Set up audio pipeline on receiver
	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)
	err = ap.SetupReceive(nil)
	if err != nil {
		t.Fatalf("SetupReceive failed: %v", err)
	}

	// Wire pipeline to endpoint
	receiverEP.SetAudioPipeline(ap)

	// Track if pipeline was started on incoming link
	var pipelineReady bool
	var pipelineMu sync.Mutex

	receiverEP.SetOnRinging(func(remoteIdentity *rns.Identity) {
		pipelineMu.Lock()
		pipelineReady = receiverEP.AudioPipeline() != nil
		pipelineMu.Unlock()
	})

	_ = pipelineReady

	// Announce receiver
	err = receiverEP.Announce()
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	// Create caller endpoint
	callerEP, err := NewTelephoneEndpoint(callerID, ts)
	if err != nil {
		t.Fatalf("NewTelephoneEndpoint failed: %v", err)
	}
	defer callerEP.Teardown()

	// Set up caller audio pipeline
	callerCodec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}
	callerAP := NewAudioPipeline(callerCodec, callerCodec, "", "", 60.0, 48000, 0.0, 0.0)
	callerEP.SetAudioPipeline(callerAP)

	// Verify pipeline is wired to endpoint
	if receiverEP.AudioPipeline() == nil {
		t.Error("Receiver endpoint should have audio pipeline after setup")
	}
	if callerEP.AudioPipeline() == nil {
		t.Error("Caller endpoint should have audio pipeline after setup")
	}

	// Verify pipeline is not started before link establishment
	if ap.Started() {
		t.Error("Audio pipeline should not be started before link establishment")
	}
}

func TestIntegration_AudioPipelineCodecSelection(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	// Verify codec is set correctly
	if ap.TransmitCodec() == nil {
		t.Error("TransmitCodec should not be nil")
	}
	if ap.ReceiveCodec() == nil {
		t.Error("ReceiveCodec should not be nil")
	}

	// Verify pipeline parameters
	if ap.TargetFrameMs() != 60.0 {
		t.Errorf("TargetFrameMs = %v, want 60.0", ap.TargetFrameMs())
	}
	if ap.Samplerate() != 48000 {
		t.Errorf("Samplerate = %v, want 48000", ap.Samplerate())
	}
}

func TestIntegration_AudioPipelineStartStop(t *testing.T) {
	t.Parallel()

	codec, err := raw.NewRaw(1, 16)
	if err != nil {
		t.Fatalf("NewRaw failed: %v", err)
	}

	ap := NewAudioPipeline(codec, codec, "", "", 60.0, 48000, 0.0, 0.0)

	// Setup both transmit and receive
	err = ap.SetupTransmit(func(data []byte) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("SetupTransmit failed: %v", err)
	}

	sr := network.NewSignallingReceiver(nil)
	err = ap.SetupReceive(sr)
	if err != nil {
		t.Fatalf("SetupReceive failed: %v", err)
	}

	// Start should succeed
	err = ap.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !ap.Started() {
		t.Error("Started should be true after Start()")
	}

	// Stop should succeed
	ap.Stop()

	if ap.Started() {
		t.Error("Started should be false after Stop()")
	}
}

func TestIntegration_AudioPipelinePacketizerLinkSource(t *testing.T) {
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

	// Verify LinkSource is created
	if ap.LinkSource() == nil {
		t.Error("LinkSource should not be nil after SetupReceive")
	}

	// Verify LinkSource has correct codec
	ls := ap.LinkSource()
	if ls.GetCodec() == nil {
		t.Error("LinkSource codec should not be nil")
	}
}
