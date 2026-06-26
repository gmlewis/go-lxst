// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"fmt"
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
	mu               sync.Mutex
	logger           *rns.Logger
	identity         *rns.Identity
	destination      *rns.Destination
	transport        rns.Transport
	telephone        *telephony.Telephone
	allowed          byte
	blocked          [][]byte
	allowList        [][]byte
	lastAnnounce     time.Time
	announceIntvl    time.Duration
	onRinging        func(remoteIdentity *rns.Identity)
	onEstablished    func(remoteIdentity *rns.Identity)
	onEnded          func(remoteIdentity *rns.Identity)
	onBusy           func(remoteIdentity *rns.Identity)
	onRejected       func(remoteIdentity *rns.Identity)
	activeLink       *rns.Link
	audioPipeline    *AudioPipeline
	shouldRun        bool
	remoteIdentified bool

	// Test hooks
	testIdentifyFunc       func(link *rns.Link, identity *rns.Identity) error
	testSendSignallingFunc func(link *rns.Link, signal int)
}

// NewTelephoneEndpoint creates a new TelephoneEndpoint bound to the given identity and transport.
func NewTelephoneEndpoint(identity *rns.Identity, ts rns.Transport, logger *rns.Logger) (*TelephoneEndpoint, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity must not be nil")
	}
	if ts == nil {
		return nil, fmt.Errorf("transport must not be nil")
	}

	tep := &TelephoneEndpoint{
		logger:        logger,
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

// logf logs to the RNS logger at Notice level, which is always emitted
// regardless of the configured log level (default is LogNotice=3).
func (tep *TelephoneEndpoint) logf(format string, args ...any) {
	if tep.logger != nil {
		tep.logger.Notice(format, args...)
	}
}

func (tep *TelephoneEndpoint) logDebugf(format string, args ...any) {
	if tep.logger != nil {
		tep.logger.Debug(format, args...)
	}
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
			if err := tep.Announce(); err != nil {
				tep.logf("outgoingLinkEstablished: Announce failed: %v", err)
			}
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

// SetTelephone attaches a telephony.Telephone for call state and pipeline management.
func (tep *TelephoneEndpoint) SetTelephone(tel *telephony.Telephone) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.telephone = tel
}

// Telephone returns the attached telephony.Telephone, if any.
func (tep *TelephoneEndpoint) Telephone() *telephony.Telephone {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.telephone
}

func (tep *TelephoneEndpoint) testSetIdentifyFunc(fn func(*rns.Link, *rns.Identity) error) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.testIdentifyFunc = fn
}

func (tep *TelephoneEndpoint) testSetSendSignallingFunc(fn func(*rns.Link, int)) {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	tep.testSendSignallingFunc = fn
}

func (tep *TelephoneEndpoint) testFireOutgoingLinkEstablished(link *rns.Link) {
	tep.outgoingLinkEstablished(link)
}

func (tep *TelephoneEndpoint) testFireCallerIdentified(hashHex string) {
	tep.callerIdentified(hashHex, nil)
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
	tep.logf("Incoming link established callback fired")
	tep.mu.Lock()
	tep.remoteIdentified = false
	remoteIdentity := link.GetRemoteIdentity()
	tep.activeLink = link
	tel := tep.telephone
	tep.mu.Unlock()

	if tel != nil {
		tel.SetIncoming(true)
		tep.logf("incomingLinkEstablished: set incoming=true, state=%v, busy=%v", tel.State(), tel.Busy())
		signalFunc := func(signal int) error {
			tep.sendSignalling(link, signal)
			return nil
		}
		teardownFunc := func() {
			link.Teardown()
		}
		tel.IncomingLinkEstablished(signalFunc, teardownFunc)

		if tel.Busy() {
			tep.logf("incomingLinkEstablished: telephone is busy, sending BUSY and tearing down")
			tep.mu.Lock()
			onBusy := tep.onBusy
			tep.mu.Unlock()
			if onBusy != nil {
				onBusy(remoteIdentity)
			}
			return
		}
	}

	if remoteIdentity == nil {
		tep.logf("incomingLinkEstablished: no remote identity yet, waiting for identification")
		link.SetRemoteIdentifiedCallback(func(l *rns.Link, id *rns.Identity) {
			tep.logf("Remote identity identified: %v", id.HexHash)
			hashHex := id.HexHash
			fmt.Printf("Incoming call from %v\n", formatHash(hashHex))

			tep.callerIdentified(hashHex, l)
		})
	} else {
		hashHex := hex.EncodeToString(remoteIdentity.Hash)
		fmt.Printf("Incoming call from %v\n", formatHash(hashHex))
		tep.callerIdentified(hashHex, link)
	}

	link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
		tep.logDebugf("Responder received packet (len=%d)", len(data))
		tep.handleSignallingData(data, link, tep.identity)
	})
}

func (tep *TelephoneEndpoint) callerIdentified(hashHex string, link *rns.Link) {
	tep.mu.Lock()
	if tep.remoteIdentified {
		tep.mu.Unlock()
		tep.logf("callerIdentified: already identified, ignoring duplicate callback")
		return
	}
	tep.remoteIdentified = true
	onRinging := tep.onRinging
	onBusy := tep.onBusy
	tel := tep.telephone
	tep.mu.Unlock()

	tep.logf("callerIdentified: hash=%v, tel=%v", formatHash(hashHex), tel != nil)

	if tel != nil {
		signalFunc := func(signal int) error {
			tep.sendSignalling(link, signal)
			return nil
		}
		teardownFunc := func() {
			if link != nil {
				link.Teardown()
			}
		}
		accepted := tel.CallerIdentified(hashHex, signalFunc, teardownFunc)
		tep.logf("callerIdentified: CallerIdentified returned %v, state=%v", accepted, tel.State())
		if !accepted {
			if onBusy != nil {
				onBusy(nil)
			}
			return
		}
	}

	if onRinging != nil {
		idHash, _ := hex.DecodeString(hashHex)
		onRinging(&rns.Identity{Hash: idHash, HexHash: hashHex})
	}
}

func (tep *TelephoneEndpoint) outgoingLinkEstablished(link *rns.Link) {
	tep.logf("Outgoing link established callback fired")
	tep.mu.Lock()
	identity := tep.identity
	tel := tep.telephone
	tep.mu.Unlock()

	if link != nil {
		link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			tep.logDebugf("Caller received packet (len=%d)", len(data))
			tep.handleSignallingData(data, link, identity)
		})
	}

	if tel != nil {
		tel.SetIncoming(false)
		tep.logf("outgoingLinkEstablished: calling OutgoingLinkEstablished, state=%v", tel.State())
		tel.OutgoingLinkEstablished(func(signal int) error {
			tep.sendSignalling(link, signal)
			return nil
		})
	}
}

// handleSignallingData unpacks and processes signalling data received
// through the link's PacketCallback. Msgpack deserializes keys as uint8
// and values as int, so we handle both byte and uint8/int types.
func (tep *TelephoneEndpoint) handleSignallingData(data []byte, link *rns.Link, identity *rns.Identity) {
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

	var signals []int
	for _, s := range arr {
		switch v := s.(type) {
		case uint8:
			signals = append(signals, int(v))
		case int:
			signals = append(signals, v)
		case int64:
			signals = append(signals, int(v))
		case uint64:
			signals = append(signals, int(v))
		default:
			continue
		}
	}

	if len(signals) == 0 {
		return
	}

	tep.mu.Lock()
	tel := tep.telephone
	tep.mu.Unlock()

	signalFunc := func(signal int) error {
		tep.sendSignalling(link, signal)
		return nil
	}

	for _, signalVal := range signals {
		switch {
		case signalVal >= telephony.SignallingPreferredProfile:
			profile := byte(signalVal - telephony.SignallingPreferredProfile)
			tep.logf("Received preferred profile: 0x%02x", profile)
			if tel != nil {
				if tel.IsEstablished() {
					tel.SwitchProfile(profile, signalFunc)
				} else {
					tel.SetProfile(profile)
				}
			}

		case signalVal == telephony.SignallingAvailable:
			tep.logDebugf("Received SignallingAvailable, identifying to remote")
			if tep.testIdentifyFunc != nil {
				if err := tep.testIdentifyFunc(link, identity); err != nil {
					tep.logf("SignallingAvailable: testIdentifyFunc failed: %v", err)
				}
			} else if identity != nil && link != nil {
				if err := link.Identify(identity); err != nil {
					tep.logf("identify failed: %v", err)
				}
			}
			if tel != nil {
				tep.logDebugf("Processing SignallingAvailable: tel state=%v", tel.State())
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingAvailable: tel state=%v", tel.State())
			}

		case signalVal == telephony.SignallingRinging:
			tep.logDebugf("Received SignallingRinging")
			if tel != nil {
				tep.logDebugf("Processing SignallingRinging: tel state=%v", tel.State())
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingRinging: tel state=%v", tel.State())
			}

		case signalVal == telephony.SignallingConnecting:
			tep.logDebugf("Received SignallingConnecting: caller setting up packetizer and link source")
			if tel != nil {
				tep.mu.Lock()
				link := tep.activeLink
				tep.mu.Unlock()

				if link != nil {
					pktz := network.NewPacketizer(func(data []byte) error {
						p := rns.NewPacket(link, data)
						p.CreateReceipt = false
						if err := p.Pack(); err != nil {
							return err
						}
						return link.SendPacket(p)
					}, func() {
						tep.logf("Packetizer failure, terminating call")
						go tel.PacketizerFailure()
					})
					tel.SetPacketizer(pktz)
				}

				tep.logDebugf("Processing SignallingConnecting: tel state=%v, packetizer=%v", tel.State(), tel.Packetizer() != nil)
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingConnecting: tel state=%v", tel.State())
			}

		case signalVal == telephony.SignallingEstablished:
			tep.logf("Received SignallingEstablished: call established, setting up link source")
			if tel != nil {
				tep.logDebugf("Processing SignallingEstablished: tel state=%v", tel.State())
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingEstablished: tel state=%v", tel.State())

				tep.mu.Lock()
				link := tep.activeLink
				tep.mu.Unlock()

				if link != nil {
					rm := tel.ReceiveMixer()
					if rm != nil {
						ls := network.NewLinkSource(nil, rm)
						if ap := tep.audioPipeline; ap != nil {
							ls.SetCodec(ap.ReceiveCodec())
						}
						rm.SetSourceMaxFrames(ls, 2)

						link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
							tep.logDebugf("Caller received packet (len=%d)", len(data))
							ls.ReceivePacket(data)
							tep.handleSignallingData(data, link, tep.identity)
						})
					} else {
						tep.logf("SignallingEstablished: receive mixer is nil, cannot set up link source")
					}
				}
			}
			tep.mu.Lock()
			onEstablished := tep.onEstablished
			tep.mu.Unlock()
			if onEstablished != nil {
				var remote *rns.Identity
				if link != nil {
					remote = link.GetRemoteIdentity()
				}
				onEstablished(remote)
			}

		case signalVal == telephony.SignallingBusy:
			tep.logDebugf("Received SignallingBusy")
			if tel != nil {
				tep.logDebugf("Processing SignallingBusy: tel state=%v", tel.State())
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingBusy: tel state=%v", tel.State())
			}
			tep.mu.Lock()
			onBusy := tep.onBusy
			tep.mu.Unlock()
			if onBusy != nil {
				var remote *rns.Identity
				if link != nil {
					remote = link.GetRemoteIdentity()
				}
				onBusy(remote)
			}

		case signalVal == telephony.SignallingRejected:
			tep.logDebugf("Received SignallingRejected")
			if tel != nil {
				tep.logDebugf("Processing SignallingRejected: tel state=%v", tel.State())
				tel.SignallingReceived([]int{signalVal}, signalFunc)
				tep.logDebugf("After SignallingRejected: tel state=%v", tel.State())
			}
			tep.mu.Lock()
			onRejected := tep.onRejected
			tep.mu.Unlock()
			if onRejected != nil {
				var remote *rns.Identity
				if link != nil {
					remote = link.GetRemoteIdentity()
				}
				onRejected(remote)
			}

		case signalVal == telephony.SignallingCalling:
			tep.logDebugf("Received SignallingCalling")

		default:
			tep.logDebugf("Received unknown signalling: %d", signalVal)
		}
	}
}

// sendSignalling sends a signalling byte through the link as a raw
// data packet, matching Python's signal() method. The receiver's
// PacketCallback will receive the packet data.
func (tep *TelephoneEndpoint) sendSignalling(link *rns.Link, signal int) {
	tep.mu.Lock()
	testFn := tep.testSendSignallingFunc
	tep.mu.Unlock()

	if testFn != nil {
		testFn(link, signal)
		return
	}

	if link == nil {
		tep.logf("sendSignalling: link is nil, cannot send signal %d", signal)
		return
	}

	signallingData := map[byte]any{network.FieldSignalling: []any{signal}}
	packed, err := network.PackData(signallingData)
	if err != nil {
		tep.logf("sendSignalling: pack failed: %v", err)
		return
	}
	p := rns.NewPacket(link, packed)
	p.CreateReceipt = false
	tep.logDebugf("sendSignalling: sending signal %d (len=%d)", signal, len(packed))
	if err := p.Pack(); err != nil {
		tep.logf("sendSignalling: pack packet failed: %v", err)
		return
	}
	if err := link.SendPacket(p); err != nil {
		tep.logf("sendSignalling: send failed: %v", err)
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
	tep.remoteIdentified = false
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

	// Request a path if we don't have one, with spinner (matching Python rnphone dial()).
	// Show identity hash in user-facing messages, not destination hash, matching Python
	// rnphone which displays "Requesting path for call to <identity_hash>".
	if !ts.HasPath(destHash) {
		fmt.Printf("Requesting path for call to %v\n", formatHash(identityHash))
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
	fmt.Printf("Path found to %v (hops=%v)\n", formatHash(identityHash), ts.HopsTo(destHash))

	remoteID := ts.Recall(destHash)
	if remoteID == nil {
		return fmt.Errorf("identity not found on network (Recall returned nil)")
	}
	tep.logf("Identity recalled from network")

	callDest, err := rns.NewDestination(ts, remoteID, rns.DestinationOut, rns.DestinationSingle, appName, primitiveName)
	if err != nil {
		return fmt.Errorf("creating call destination: %w", err)
	}
	tep.logf("Call destination created")

	link, err := rns.NewLink(ts, callDest)
	if err != nil {
		return fmt.Errorf("creating link: %w", err)
	}
	tep.logf("Link object created")

	tep.mu.Lock()
	tep.activeLink = link
	tep.mu.Unlock()

	link.SetLinkEstablishedCallback(func(l *rns.Link) {
		tep.outgoingLinkEstablished(l)
	})

	link.SetLinkClosedCallback(func(l *rns.Link) {
		tep.logf("Link closed callback fired")
		tep.mu.Lock()
		tep.activeLink = nil
		ap := tep.audioPipeline
		tep.audioPipeline = nil
		onEnded := tep.onEnded
		tel := tep.telephone
		tep.mu.Unlock()

		if tel != nil {
			tel.Hangup()
		}

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

	// Clear spinner — the callback will print the "Call established" message
	fmt.Print("\r" + strings.Repeat(" ", 40) + "\r")

	return nil
}

// ActiveLink returns the current active link, if any.
func (tep *TelephoneEndpoint) ActiveLink() *rns.Link {
	tep.mu.Lock()
	defer tep.mu.Unlock()
	return tep.activeLink
}

// Hangup terminates the current active call and stops all audio pipelines.
func (tep *TelephoneEndpoint) Hangup() {
	tep.mu.Lock()
	link := tep.activeLink
	tel := tep.telephone
	ap := tep.audioPipeline
	tep.activeLink = nil
	tep.audioPipeline = nil
	tep.remoteIdentified = false
	tep.mu.Unlock()

	tep.logf("TelephoneEndpoint.Hangup: link=%v, tel=%v", link != nil, tel != nil)

	// Tear down the link first so that any pending sends in the mixer
	// goroutine fail immediately, allowing StopPipelines to complete.
	if link != nil {
		link.Teardown()
	}

	// Stop audio pipelines to prevent further send attempts.
	if ap != nil {
		ap.Stop()
	}

	if tel != nil {
		tep.logf("TelephoneEndpoint.Hangup: tel state=%v before hangup", tel.State())
		tel.Hangup()
		tep.logf("TelephoneEndpoint.Hangup: tel state=%v after hangup", tel.State())
	}
}

// Answer accepts an incoming call. It signals CONNECTING, creates the
// Packetizer and LinkSource, opens audio pipelines, signals ESTABLISHED,
// and starts the pipelines, matching the Python Telephony.answer() flow.
func (tep *TelephoneEndpoint) Answer() bool {
	tep.mu.Lock()
	link := tep.activeLink
	tel := tep.telephone
	identity := tep.identity
	tep.mu.Unlock()

	tep.logf("TelephoneEndpoint.Answer() called: link=%v, tel=%v", link != nil, tel != nil)

	if link == nil {
		tep.logf("TelephoneEndpoint.Answer(): no active link, returning false")
		return false
	}

	if tel == nil {
		tep.logf("TelephoneEndpoint.Answer(): no telephone, returning false")
		return false
	}

	if !tel.Answer() {
		tep.logf("TelephoneEndpoint.Answer(): tel.Answer() returned false (state=%v), returning false", tel.State())
		return false
	}

	tep.logf("TelephoneEndpoint.Answer(): tel.Answer() succeeded, incoming=%v", tel.Incoming())

	signalFunc := func(signal int) error {
		tep.sendSignalling(link, signal)
		return nil
	}

	if tel.Incoming() {
		tep.logf("TelephoneEndpoint.Answer(): incoming call, sending CONNECTING signal")
		if err := signalFunc(telephony.SignallingConnecting); err != nil {
			tep.logf("Answer: signalFunc Connecting failed: %v", err)
		}
	}

	pktz := network.NewPacketizer(func(data []byte) error {
		p := rns.NewPacket(link, data)
		p.CreateReceipt = false
		if err := p.Pack(); err != nil {
			return err
		}
		return link.SendPacket(p)
	}, func() {
		tep.logf("Packetizer failure, terminating call")
		go tel.PacketizerFailure()
	})

	tel.SetPacketizer(pktz)

	tel.PrepareDiallingPipelines()
	tel.OpenPipelines()

	var ls *network.LinkSource
	rm := tel.ReceiveMixer()
	if rm != nil {
		ls = network.NewLinkSource(nil, rm)
		if ap := tep.audioPipeline; ap != nil {
			ls.SetCodec(ap.ReceiveCodec())
		}
		rm.SetSourceMaxFrames(ls, 2)
	} else {
		tep.logf("TelephoneEndpoint.Answer(): receive mixer is nil, audio receive disabled")
	}

	tep.logf("TelephoneEndpoint.Answer(): sending ESTABLISHED signal")
	if err := signalFunc(telephony.SignallingEstablished); err != nil {
		tep.logf("Answer: signalFunc Established failed: %v", err)
	}

	tel.StartPipelines()

	if ls != nil {
		link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			tep.logDebugf("Responder received packet (len=%d)", len(data))

			ls.ReceivePacket(data)

			tep.handleSignallingData(data, link, identity)
		})
	} else {
		link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			tep.logDebugf("Responder received packet (len=%d)", len(data))
			tep.handleSignallingData(data, link, identity)
		})
	}

	tep.logf("TelephoneEndpoint.Answer(): call fully established, audio pipelines running")
	return true
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
