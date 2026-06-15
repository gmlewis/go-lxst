// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gmlewis/go-lxst/lxst/network"
	"github.com/gmlewis/go-lxst/lxst/primitives/telephony"
	"github.com/gmlewis/go-reticulum/rns"
)

const (
	appName       = "lxst"
	primitiveName = "telephony"
)

// TelephoneEndpoint manages RNS Destination and Link handling for a telephone.
type TelephoneEndpoint struct {
	mu            sync.Mutex
	identity      *rns.Identity
	destination   *rns.Destination
	transport     rns.Transport
	allowed       byte
	blocked       [][]byte
	allowList     [][]byte
	lastAnnounce  time.Time
	announceIntvl time.Duration
	onRinging     func(remoteIdentity *rns.Identity)
	onEstablished func(remoteIdentity *rns.Identity)
	onEnded       func(remoteIdentity *rns.Identity)
	onBusy        func(remoteIdentity *rns.Identity)
	onRejected    func(remoteIdentity *rns.Identity)
	activeLink    *rns.Link
	audioPipeline *AudioPipeline
	shouldRun     bool
}

// NewTelephoneEndpoint creates a new TelephoneEndpoint bound to the given identity and transport.
func NewTelephoneEndpoint(identity *rns.Identity, ts rns.Transport) (*TelephoneEndpoint, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity must not be nil")
	}
	if ts == nil {
		return nil, fmt.Errorf("transport must not be nil")
	}

	tep := &TelephoneEndpoint{
		identity:      identity,
		transport:     ts,
		allowed:       rns.AllowAll,
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

// Destination returns the underlying RNS Destination.
func (tep *TelephoneEndpoint) Destination() *rns.Destination {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.destination
}

// IdentityHash returns the hex-encoded identity hash.
func (tep *TelephoneEndpoint) IdentityHash() string {
	return tep.identity.HexHash
}

// DestinationHash returns the hex-encoded destination hash.
func (tep *TelephoneEndpoint) DestinationHash() string {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.destination.HexHash
}

// Announce broadcasts the telephone destination on the network.
func (tep *TelephoneEndpoint) Announce() error {
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

// NeedsAnnounce reports whether the announce interval has elapsed.
func (tep *TelephoneEndpoint) NeedsAnnounce() bool {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return time.Since(tep.lastAnnounce) >= tep.announceIntvl
}

// StartJobs launches the background job loop that periodically re-announces.
func (tep *TelephoneEndpoint) StartJobs() {
	tep.mu.Lock()
	tep.shouldRun = true
	tep.mu.Unlock()

	go tep.jobsLoop()
}

// StopJobs stops the background job loop.
func (tep *TelephoneEndpoint) StopJobs() {
	tep.mu.Lock()
	tep.shouldRun = false
	tep.mu.Unlock()
}

func (tep *TelephoneEndpoint) jobsLoop() {
	for {
		tep.mu.Lock()
		running := tep.shouldRun
		tep.mu.Unlock()
		if !running {
			return
		}

		if tep.NeedsAnnounce() {
			_ = tep.Announce()
		}

		time.Sleep(tep.JobInterval())
	}
}

// JobInterval returns the polling interval for the background job loop.
func (tep *TelephoneEndpoint) JobInterval() time.Duration {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	intvl := tep.announceIntvl / 10
	if intvl < 100*time.Millisecond {
		intvl = 100 * time.Millisecond
	}
	if intvl > 30*time.Second {
		intvl = 30 * time.Second
	}
	return intvl
}

// SetAllowed sets the allowed callers policy.
func (tep *TelephoneEndpoint) SetAllowed(allowed byte) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.allowed = allowed
}

// SetBlocked sets the blocked callers list.
func (tep *TelephoneEndpoint) SetBlocked(blocked [][]byte) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.blocked = blocked
}

// SetAllowList sets the specific allowed callers list.
func (tep *TelephoneEndpoint) SetAllowList(list [][]byte) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.allowList = list
}

// SetOnRinging sets the callback for incoming call notifications.
func (tep *TelephoneEndpoint) SetOnRinging(fn func(*rns.Identity)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.onRinging = fn
}

// SetOnEstablished sets the callback for call establishment.
func (tep *TelephoneEndpoint) SetOnEstablished(fn func(*rns.Identity)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.onEstablished = fn
}

// SetOnEnded sets the callback for call end.
func (tep *TelephoneEndpoint) SetOnEnded(fn func(*rns.Identity)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.onEnded = fn
}

// SetOnBusy sets the callback for busy signals.
func (tep *TelephoneEndpoint) SetOnBusy(fn func(*rns.Identity)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.onBusy = fn
}

// SetOnRejected sets the callback for rejected calls.
func (tep *TelephoneEndpoint) SetOnRejected(fn func(*rns.Identity)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.onRejected = fn
}

// SetAudioPipeline attaches an audio pipeline for transmit/receive audio.
func (tep *TelephoneEndpoint) SetAudioPipeline(ap *AudioPipeline) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.audioPipeline = ap
}

// AudioPipeline returns the attached audio pipeline, if any.
func (tep *TelephoneEndpoint) AudioPipeline() *AudioPipeline {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.audioPipeline
}

// IsCallerAllowed reports whether an identity hash is permitted to call.
func (tep *TelephoneEndpoint) IsCallerAllowed(hashHex string) bool {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.isCallerAllowedLocked(hashHex)
}

func (tep *TelephoneEndpoint) isCallerAllowedLocked(hashHex string) bool {
	for _, blocked := range tep.blocked {
		if hex.EncodeToString(blocked) == hashHex {
			return false
		}
	}

	switch tep.allowed {
	case rns.AllowAll:
		return true
	case rns.AllowNone:
		return false
	default:
		for _, allowed := range tep.allowList {
			if hex.EncodeToString(allowed) == hashHex {
				return true
			}
		}
		return false
	}
}

func (tep *TelephoneEndpoint) incomingLinkEstablished(link *rns.Link) {
	log.Printf("Incoming link established callback fired")
	tep.mu.Lock()
	remoteIdentity := link.GetRemoteIdentity()
	tep.activeLink = link
	onRinging := tep.onRinging
	onBusy := tep.onBusy
	ap := tep.audioPipeline
	tep.mu.Unlock()

	// If remote identity is not yet known (common for responder side),
	// set up the RemoteIdentified callback to ring when the caller
	// identifies itself, and send SignallingAvailable to request identification.
	if remoteIdentity == nil {
		link.SetRemoteIdentifiedCallback(func(l *rns.Link, id *rns.Identity) {
			log.Printf("Remote identity identified: %v", id.HexHash)
			hashHex := id.HexHash
			fmt.Printf("Incoming call from %v\n", prettyHex(hashHex))

			tep.mu.Lock()
			onRinging := tep.onRinging
			onBusy := tep.onBusy
			tep.mu.Unlock()

			if !tep.IsCallerAllowed(hashHex) {
				if onBusy != nil {
					onBusy(id)
				}
				l.Teardown()
				return
			}

			if onRinging != nil {
				onRinging(id)
			}
		})

		// Send SignallingAvailable through the link to request caller identification
		tep.sendSignalling(link, telephony.SignallingAvailable)

		// Don't call onRinging yet — wait for the caller to identify
	} else {
		hashHex := hex.EncodeToString(remoteIdentity.Hash)
		fmt.Printf("Incoming call from %v\n", prettyHex(hashHex))
		if !tep.IsCallerAllowed(hashHex) {
			if onBusy != nil {
				onBusy(remoteIdentity)
			}
			link.Teardown()
			return
		}

		if onRinging != nil {
			onRinging(remoteIdentity)
		}
	}

	// Start audio pipeline for incoming call
	if ap != nil {
		sr := network.NewSignallingReceiver(nil)
		if err := ap.SetupReceive(sr); err != nil {
			_ = fmt.Errorf("setting up receive pipeline: %w", err)
		}
		if err := ap.Start(); err != nil {
			_ = fmt.Errorf("starting receive pipeline: %w", err)
		}

		// Wire link to receive incoming packets
		link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			log.Printf("Responder received packet (len=%d)", len(data))
			ap.ReceivePacket(data)
		})
	}
}

// sendSignalling sends a signalling byte through the link as a raw
// data packet, matching Python's signal() method. The receiver's
// PacketCallback will receive the packet data.
func (tep *TelephoneEndpoint) sendSignalling(link *rns.Link, signal byte) {
	signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
	packed, err := network.PackData(signallingData)
	if err != nil {
		log.Printf("sendSignalling: pack failed: %v", err)
		return
	}
	p := rns.NewPacket(link, packed)
	p.CreateReceipt = false
	log.Printf("sendSignalling: sending signal %d (len=%d)", signal, len(packed))
	if err := p.Pack(); err != nil {
		log.Printf("sendSignalling: pack packet failed: %v", err)
		return
	}
	if err := link.SendPacket(p); err != nil {
		log.Printf("sendSignalling: send failed: %v", err)
	}
}

// Teardown cleans up the endpoint.
func (tep *TelephoneEndpoint) Teardown() {
	tep.mu.Lock()
	defer tep.mu.Unlock()

	if tep.activeLink != nil {
		tep.activeLink.Teardown()
		tep.activeLink = nil
	}
	tep.destination = nil
}

// Call initiates an outgoing call to the given identity hash.
// It requests a path if needed, then creates an RNS Link.
func (tep *TelephoneEndpoint) Call(identityHash string, timeout time.Duration) error {
	tep.mu.Lock()
	if tep.activeLink != nil {
		tep.mu.Unlock()
		return fmt.Errorf("already in a call")
	}
	ts := tep.transport
	tep.mu.Unlock()

	identityBytes, err := hex.DecodeString(identityHash)
	if err != nil {
		return fmt.Errorf("invalid identity hash: %w", err)
	}

	// Compute the destination hash from the identity hash and app name,
	// matching Python's RNS.Destination.hash_from_name_and_identity.
	tempID, err := rns.NewIdentity(false, ts.GetLogger())
	if err != nil {
		return fmt.Errorf("creating temp identity: %w", err)
	}
	tempID.Hash = identityBytes
	tempID.HexHash = identityHash
	destHash := rns.CalculateHash(tempID, appName, primitiveName)

	// Request a path if we don't have one, with spinner (matching Python rnphone dial())
	if !ts.HasPath(destHash) {
		fmt.Printf("No path to %x, requesting...\n", destHash)
		if err := ts.RequestPath(destHash); err != nil {
			return fmt.Errorf("requesting path: %w", err)
		}
		spinner := []string{".", "..", "...", "....", ".....", "......", "......."}
		index := 0
		fmt.Print("Discovering path")
		deadline := time.Now().Add(timeout)
		for !ts.HasPath(destHash) && time.Now().Before(deadline) {
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("%s", spinner[index%len(spinner)])
			index++
		}
		fmt.Print("\r" + strings.Repeat(" ", 40) + "\r")
		if !ts.HasPath(destHash) {
			return fmt.Errorf("path request timed out (is the remote phone announced and reachable?)")
		}
	}
	fmt.Printf("Path found to %x (hops=%v)\n", destHash, ts.HopsTo(destHash))

	remoteID := ts.Recall(destHash)
	if remoteID == nil {
		return fmt.Errorf("identity not found on network (Recall returned nil)")
	}
	log.Printf("Identity recalled from network")

	callDest, err := rns.NewDestination(ts, remoteID, rns.DestinationOut, rns.DestinationSingle, appName, primitiveName)
	if err != nil {
		return fmt.Errorf("creating call destination: %w", err)
	}
	log.Printf("Call destination created")

	link, err := rns.NewLink(ts, callDest)
	if err != nil {
		return fmt.Errorf("creating link: %w", err)
	}
	log.Printf("Link object created")

	tep.mu.Lock()
	tep.activeLink = link
	tep.mu.Unlock()

	link.SetLinkEstablishedCallback(func(l *rns.Link) {
		log.Printf("Outgoing link established callback fired")
		tep.mu.Lock()
		onEstablished := tep.onEstablished
		ap := tep.audioPipeline
		identity := tep.identity
		tep.mu.Unlock()

		// Set up packet callback to handle incoming signalling
		l.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			log.Printf("Caller received packet (len=%d)", len(data))

			if ap != nil {
				ap.ReceivePacket(data)
			}

			// Try to handle signalling
			unpacked, err := network.UnpackData(data)
			if err != nil {
				return
			}
			m, ok := unpacked.(map[byte]any)
			if !ok {
				return
			}
			if signalling, exists := m[network.FieldSignalling]; exists {
				log.Printf("Caller received signalling data: %v", signalling)
				switch v := signalling.(type) {
				case []any:
					for _, s := range v {
						if b, ok := s.(byte); ok && b == telephony.SignallingAvailable {
							log.Printf("Received SignallingAvailable, identifying to remote")
							if identity != nil {
								if err := l.Identify(identity); err != nil {
									log.Printf("identify failed: %v", err)
								}
							}
						}
					}
				}
			}
		})

		// Send SignallingCalling to indicate we're calling
		tep.sendSignalling(l, telephony.SignallingCalling)

		if ap != nil {
			if err := ap.SetupTransmit(func(data []byte) error {
				msg := &rawMessage{data: data}
				_, err := l.GetChannel().Send(msg)
				return err
			}, nil); err != nil {
				log.Printf("setting up transmit pipeline: %v", err)
			}
			if err := ap.Start(); err != nil {
				log.Printf("starting transmit pipeline: %v", err)
			}
		}

		if onEstablished != nil {
			remote := l.GetRemoteIdentity()
			onEstablished(remote)
		}
	})

	link.SetLinkClosedCallback(func(l *rns.Link) {
		log.Printf("Link closed callback fired")
		tep.mu.Lock()
		tep.activeLink = nil
		onEnded := tep.onEnded
		ap := tep.audioPipeline
		tep.mu.Unlock()

		if ap != nil {
			ap.Stop()
		}

		if onEnded != nil {
			remote := l.GetRemoteIdentity()
			onEnded(remote)
		}
	})

	fmt.Print("Establishing link")
	if err := link.Establish(); err != nil {
		tep.mu.Lock()
		tep.activeLink = nil
		tep.mu.Unlock()
		return fmt.Errorf("establishing link: %w", err)
	}

	// Wait for link handshake to complete
	spinner := []string{".", "..", "...", "....", ".....", "......", "......."}
	index := 0
	deadline := time.Now().Add(timeout)
	for link.GetStatus() != rns.LinkActive && time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		fmt.Printf("%s", spinner[index%len(spinner)])
		index++
	}

	if link.GetStatus() != rns.LinkActive {
		// Clear spinner line and print error
		fmt.Print("\r" + strings.Repeat(" ", 40) + "\r")
		return fmt.Errorf("link handshake timed out")
	}

	// Link is active — print newline to finish the line so that
	// the callback's "Call established" output starts on a fresh line.
	fmt.Println()

	return nil
}

// ActiveLink returns the current active link, if any.
func (tep *TelephoneEndpoint) ActiveLink() *rns.Link {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.activeLink
}

// Hangup terminates the current active call.
func (tep *TelephoneEndpoint) Hangup() {
	tep.mu.Lock()
	defer tep.mu.Unlock()

	if tep.activeLink != nil {
		tep.activeLink.Teardown()
		tep.activeLink = nil
	}
}

// rawMessage implements rns.Message for sending raw byte data through channels.
type rawMessage struct {
	data []byte
}

func (m *rawMessage) GetMsgType() uint16 { return 0 }

func (m *rawMessage) Pack() ([]byte, error) {
	return m.data, nil
}

func (m *rawMessage) Unpack(data []byte) error {
	m.data = data
	return nil
}
