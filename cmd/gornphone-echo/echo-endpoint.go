// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/codecs"
	"github.com/gmlewis/go-lxst/lxst/mixer"
	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

const (
	appName       = "lxst"
	primitiveName = "telephony"
)

// EchoEndpoint manages RNS Destination and Link handling for the echo service.
// It auto-answers incoming calls, generates a continuous test tone, and
// echoes back all received audio after a configurable delay.
type EchoEndpoint struct {
	mu            sync.Mutex
	logger        *rns.Logger
	identity      *rns.Identity
	destination   *rns.Destination
	transport     rns.Transport
	activeLink    *rns.Link
	shouldRun     bool
	lastAnnounce  time.Time
	announceIntvl time.Duration

	codec   codecs.Codec
	profile byte
	frameMs float64
	delay   time.Duration
	freq    float64
	gain    float64

	// Audio pipeline components (created on answer)
	transmitMixer *mixer.Mixer
	packetizer    *network.Packetizer
	linkSource    *network.LinkSource
	echoSource    *EchoSource
}

// NewEchoEndpoint creates a new EchoEndpoint bound to the given identity and transport.
func NewEchoEndpoint(identity *rns.Identity, ts rns.Transport, logger *rns.Logger, codec codecs.Codec, profile byte, frameMs float64, delay time.Duration, freq, gain float64) (*EchoEndpoint, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity must not be nil")
	}
	if ts == nil {
		return nil, fmt.Errorf("transport must not be nil")
	}

	tep := &EchoEndpoint{
		logger:        logger,
		identity:      identity,
		transport:     ts,
		codec:         codec,
		profile:       profile,
		frameMs:       frameMs,
		delay:         delay,
		freq:          freq,
		gain:          gain,
		announceIntvl: 3 * time.Hour,
	}

	dest, err := rns.NewDestination(ts, identity, rns.DestinationIn, rns.DestinationSingle, appName, primitiveName)
	if err != nil {
		return nil, fmt.Errorf("creating destination: %w", err)
	}
	dest.SetProofStrategy(rns.ProveNone)
	dest.SetLinkEstablishedCallback(tep.incomingLinkEstablished)
	tep.destination = dest

	return tep, nil
}

func (tep *EchoEndpoint) logf(format string, args ...any) {
	if tep.logger != nil {
		tep.logger.Notice(format, args...)
	}
}

func (tep *EchoEndpoint) logDebugf(format string, args ...any) {
	if tep.logger != nil {
		tep.logger.Debug(format, args...)
	}
}

// DestinationHash returns the hex-encoded destination hash.
func (tep *EchoEndpoint) DestinationHash() string {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.destination.HexHash
}

// Announce broadcasts the echo destination on the network.
func (tep *EchoEndpoint) Announce() error {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	if tep.destination == nil {
		return fmt.Errorf("destination not initialized")
	}
	if err := tep.destination.Announce(nil); err != nil {
		return fmt.Errorf("announce failed: %w", err)
	}
	tep.lastAnnounce = time.Now()
	return nil
}

// StartJobs launches the background re-announce loop.
func (tep *EchoEndpoint) StartJobs() {
	tep.mu.Lock()
	tep.shouldRun = true
	tep.mu.Unlock()
	go tep.jobsLoop()
}

// StopJobs stops the background loop.
func (tep *EchoEndpoint) StopJobs() {
	tep.mu.Lock()
	tep.shouldRun = false
	tep.mu.Unlock()
}

func (tep *EchoEndpoint) jobsLoop() {
	for {
		tep.mu.Lock()
		running := tep.shouldRun
		tep.mu.Unlock()
		if !running {
			return
		}
		tep.mu.Lock()
		intvl := tep.announceIntvl / 10
		tep.mu.Unlock()
		if intvl < 100*time.Millisecond {
			intvl = 100 * time.Millisecond
		}
		if intvl > 30*time.Second {
			intvl = 30 * time.Second
		}
		time.Sleep(intvl)
	}
}

func (tep *EchoEndpoint) incomingLinkEstablished(link *rns.Link) {
	tep.logf("Incoming link established callback fired")
	tep.mu.Lock()
	tep.activeLink = link
	tep.mu.Unlock()

	// Send AVAILABLE so the caller knows to identify.
	tep.sendSignalling(link, telephony.SignallingAvailable)

	remoteIdentity := link.GetRemoteIdentity()
	if remoteIdentity == nil {
		tep.logf("No remote identity yet, waiting for identification")
		link.SetRemoteIdentifiedCallback(func(l *rns.Link, id *rns.Identity) {
			tep.logf("Remote identity identified: %v", id.HexHash)
			fmt.Printf("Incoming call from <%v>\n", id.HexHash)
			tep.callerIdentified(id.HexHash, l)
		})
	} else {
		hashHex := hex.EncodeToString(remoteIdentity.Hash)
		fmt.Printf("Incoming call from <%v>\n", hashHex)
		tep.callerIdentified(hashHex, link)
	}

	// Set packet callback for signalling.
	link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
		tep.handleSignallingData(data, link, tep.identity)
	})

	// Stop audio pipelines when the link closes (e.g. remote hangup).
	link.SetLinkClosedCallback(func(l *rns.Link) {
		tep.logf("Link closed, hanging up")
		tep.hangup()
	})
}

func (tep *EchoEndpoint) callerIdentified(hashHex string, link *rns.Link) {
	tep.mu.Lock()
	if tep.activeLink == nil {
		tep.mu.Unlock()
		return
	}
	// Guard against duplicate callback firings.
	if tep.echoSource != nil {
		tep.mu.Unlock()
		tep.logf("Caller identified: %v, but already answered, ignoring", hashHex)
		return
	}
	tep.mu.Unlock()

	tep.logf("Caller identified: %v, auto-answering", hashHex)

	// Send RINGING (AVAILABLE was already sent in incomingLinkEstablished).
	tep.sendSignalling(link, telephony.SignallingRinging)

	// Auto-answer immediately.
	tep.answer(link)
}

func (tep *EchoEndpoint) answer(link *rns.Link) bool {
	tep.logf("Answering call")

	signalFunc := func(signal int) error {
		tep.sendSignalling(link, signal)
		return nil
	}

	// Send CONNECTING.
	if err := signalFunc(telephony.SignallingConnecting); err != nil {
		tep.logf("answer: signalFunc Connecting failed: %v", err)
	}

	tep.mu.Lock()
	codec := tep.codec
	frameMs := tep.frameMs
	delay := tep.delay
	freq := tep.freq
	gain := tep.gain
	tep.mu.Unlock()

	// Create the packetizer for sending audio over the link.
	pktz := network.NewPacketizer(func(data []byte) error {
		p := rns.NewPacket(link, data)
		p.CreateReceipt = false
		if err := p.Pack(); err != nil {
			return err
		}
		return link.SendPacket(p)
	}, func() {
		tep.logf("Packetizer failure, terminating call")
		tep.hangup()
	})
	pktz.SetCodec(codec)

	tep.mu.Lock()
	tep.packetizer = pktz
	tep.mu.Unlock()

	// Create transmit mixer — this mixes tone + echo and encodes to the packetizer.
	// Use the codec's preferred sample rate so the Mixer generates frames
	// with the correct number of samples for the encoder.
	mixerSampleRate := 48000
	if sr, ok := codec.(interface{ PreferredSampleRate() int }); ok {
		mixerSampleRate = sr.PreferredSampleRate()
	}
	transmitMixer := mixer.NewMixer(frameMs, mixerSampleRate, codec, pktz, 0.0)

	// Create the EchoSource — generates tone AND buffers received audio for delayed echo.
	echoSrc := NewEchoSource(freq, gain, delay, frameMs, transmitMixer)
	echoSrc.sampleRate = float64(mixerSampleRate)

	tep.mu.Lock()
	tep.transmitMixer = transmitMixer
	tep.echoSource = echoSrc
	tep.mu.Unlock()

	// Create LinkSource to receive audio from the remote.
	linkSrc := network.NewLinkSource(nil, echoSrc)
	linkSrc.SetCodec(codec)
	echoSrc.SetLinkSource(linkSrc)

	tep.mu.Lock()
	tep.linkSource = linkSrc
	tep.mu.Unlock()

	// Send ESTABLISHED.
	if err := signalFunc(telephony.SignallingEstablished); err != nil {
		log.Fatalf("signalFunc: %v", err)
	}

	// Start everything.
	if err := transmitMixer.Start(); err != nil {
		log.Fatalf("transmitMixer.Start: %v", err)
	}
	if err := linkSrc.Start(); err != nil {
		log.Fatalf("linkSrc.Start: %v", err)
	}
	if err := echoSrc.Start(); err != nil {
		log.Fatalf("echoSrc.Start: %v", err)
	}

	// Replace packet callback to feed LinkSource AND handle signalling.
	link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
		tep.logDebugf("Echo received packet (len=%d)", len(data))
		linkSrc.ReceivePacket(data)
		tep.handleSignallingData(data, link, tep.identity)
	})

	fmt.Println("Call established. Echoing audio...")
	tep.logf("Call established, audio pipelines running (delay=%v, freq=%.1f, gain=%.2f)", delay, freq, gain)
	return true
}

// handleSignallingData processes incoming signalling packets.
func (tep *EchoEndpoint) handleSignallingData(data []byte, link *rns.Link, identity *rns.Identity) {
	unpacked, err := network.UnpackData(data)
	if err != nil {
		return
	}
	m, ok := unpacked.(map[byte]any)
	if !ok {
		return
	}
	signalling := m[network.FieldSignalling]
	if signalling == nil {
		return
	}
	arr, ok := signalling.([]any)
	if !ok {
		return
	}
	for _, s := range arr {
		signalVal := toInt(s)
		switch {
		case signalVal >= telephony.SignallingPreferredProfile:
			profile := byte(signalVal - telephony.SignallingPreferredProfile)
			tep.logf("Received preferred profile: 0x%02x", profile)
			tep.mu.Lock()
			tep.profile = profile
			tep.codec = getCodecForProfile(profile)
			if tep.packetizer != nil {
				tep.packetizer.SetCodec(tep.codec)
			}
			if tep.transmitMixer != nil {
				if err := tep.transmitMixer.SetCodec(tep.codec); err != nil {
					tep.logf("handleSignallingData: transmitMixer.SetCodec failed: %v", err)
				}
			}
			tep.mu.Unlock()

		case signalVal == telephony.SignallingBusy:
			tep.logf("Received SignallingBusy, hanging up")
			tep.hangup()

		case signalVal == telephony.SignallingRejected:
			tep.logf("Received SignallingRejected, hanging up")
			tep.hangup()

		case signalVal == telephony.SignallingAvailable:
			tep.logf("Received SignallingAvailable, identifying to remote")
			if identity != nil && link != nil {
				if err := link.Identify(identity); err != nil {
					tep.logf("identify failed: %v", err)
				}
			}

		case signalVal == telephony.SignallingCalling:
			tep.logf("Received SignallingCalling")

		default:
			tep.logf("Received signal: %d", signalVal)
		}
	}
}

func (tep *EchoEndpoint) sendSignalling(link *rns.Link, signal int) {
	if link == nil {
		return
	}
	signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
	packed, err := network.PackData(signallingData)
	if err != nil {
		return
	}
	p := rns.NewPacket(link, packed)
	p.CreateReceipt = false
	if err := p.Pack(); err != nil {
		tep.logf("sendSignalling: Pack failed: %v", err)
		return
	}
	if err := link.SendPacket(p); err != nil {
		tep.logf("sendSignalling: SendPacket failed: %v", err)
	}
}

func (tep *EchoEndpoint) hangup() {
	tep.mu.Lock()
	link := tep.activeLink
	// pktz := tep.packetizer
	ls := tep.linkSource
	es := tep.echoSource
	tm := tep.transmitMixer
	tep.activeLink = nil
	tep.packetizer = nil
	tep.linkSource = nil
	tep.echoSource = nil
	tep.transmitMixer = nil
	tep.mu.Unlock()

	if es != nil {
		if err := es.Stop(); err != nil {
			log.Fatalf("es.Stop: %v", err)
		}
	}
	if ls != nil {
		if err := ls.Stop(); err != nil {
			log.Fatalf("ls.Stop: %v", err)
		}
	}
	if tm != nil {
		if err := tm.Stop(); err != nil {
			log.Fatalf("tm.Stop: %v", err)
		}
	}
	if link != nil {
		link.Teardown()
	}
	fmt.Println("Call ended.")
}

// toInt converts a msgpack-decoded value to int.
func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int8:
		return int(val)
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint8:
		return int(val)
	case uint16:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	default:
		return 0
	}
}

// getCodecForProfile returns the codec for a given profile byte.
func getCodecForProfile(profile byte) codecs.Codec {
	c, err := telephony.GetCodec(profile)
	if err != nil {
		c, err := telephony.GetCodec(telephony.DefaultProfile)
		if err != nil {
			log.Printf("getCodecForProfile: default codec failed: %v", err)
		}
		return c
	}
	return c
}
