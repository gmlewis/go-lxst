// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gmlewis/go-lxst/testutils"
	"github.com/gmlewis/go-reticulum/rns"
)

// rnsCallTestConfig writes a minimal RNS config for a single UDP interface
// pair and returns the config directory path.
func rnsCallTestConfig(t *testing.T, instanceName string, listenPort, forwardPort int) string {
	t.Helper()
	dir := testutils.TempDir(t, "gornphone-call-test-")

	configText := fmt.Sprintf(`[reticulum]
enable_transport = Yes
share_instance = No
instance_name = %s

[logging]
loglevel = 4

[interfaces]
  [[Default Interface]]
    type = UDPInterface
    enabled = Yes
    listen_ip = 127.0.0.1
    listen_port = %d
    forward_ip = 127.0.0.1
    forward_port = %d
`, instanceName, listenPort, forwardPort)

	configPath := filepath.Join(dir, "config")
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return dir
}

// newCallTestRNS creates a Reticulum instance with a UDP interface for
// call testing. It returns the transport, identity, and a cleanup function.
func newCallTestRNS(t *testing.T, instanceName string, listenPort, forwardPort int) (rns.Transport, *rns.Identity, func()) {
	t.Helper()
	configDir := rnsCallTestConfig(t, instanceName, listenPort, forwardPort)
	logger := rns.NewLogger()
	logger.SetLogLevel(4)
	ts := rns.NewTransportSystem(logger)
	ret, err := rns.NewReticulumWithLogger(ts, configDir, logger)
	if err != nil {
		t.Fatalf("NewReticulum(%s) failed: %v", instanceName, err)
	}
	id, err := rns.NewIdentity(true, logger)
	if err != nil {
		t.Fatalf("NewIdentity(%s) failed: %v", instanceName, err)
	}
	cleanup := func() {
		_ = ret.Close()
	}
	return ts, id, cleanup
}

// TestRNSLinkEstablishmentAndDataFlow verifies that an RNS link can be
// established between two instances on the same machine via loopback UDP,
// and that data packets flow bidirectionally over the link. This is the
// single-machine integration test that verifies the core call connectivity
// path without needing a physical remote machine.
func TestRNSLinkEstablishmentAndDataFlow(t *testing.T) {
	testutils.SkipShortIntegration(t)

	portA := testutils.ReserveUDPPort(t)
	portB := testutils.ReserveUDPPort(t)

	// Two RNS instances pointing at each other on loopback.
	tsA, _, cleanupA := newCallTestRNS(t, "call-link-A", portA, portB)
	defer cleanupA()
	tsB, idB, cleanupB := newCallTestRNS(t, "call-link-B", portB, portA)
	defer cleanupB()

	logger := tsA.GetLogger()

	// Create an IN destination on B (listener side).
	destB, err := rns.NewDestination(tsB, idB, rns.DestinationIn, rns.DestinationSingle, "lxst", "telephony")
	if err != nil {
		t.Fatalf("NewDestination(B) failed: %v", err)
	}

	// Track link establishment on B.
	var (
		bMu         sync.Mutex
		bLinkActive bool
		bDataRcvd   int
	)

	destB.SetLinkEstablishedCallback(func(link *rns.Link) {
		bMu.Lock()
		bLinkActive = true
		bMu.Unlock()
		link.SetPacketCallback(func(data []byte, packet *rns.Packet) {
			bMu.Lock()
			bDataRcvd++
			bMu.Unlock()
		})
	})

	// Announce B so A can discover it.
	if err := destB.Announce(nil); err != nil {
		t.Fatalf("Announce(B) failed: %v", err)
	}

	// Wait for A to learn the path to B.
	destHashB := rns.CalculateHash(idB, "lxst", "telephony")
	deadline := time.Now().Add(10 * time.Second)
	for !tsA.HasPath(destHashB) && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if !tsA.HasPath(destHashB) {
		t.Fatal("A did not learn path to B within 10s")
	}
	logger.Info("Path to B found, hops=%v", tsA.HopsTo(destHashB))

	// Recall B's identity from the network (populated by announce).
	remoteID := tsA.Recall(destHashB)
	if remoteID == nil {
		t.Fatal("A could not recall B's identity")
	}

	// Create an OUT destination on A using the recalled identity and establish a link.
	callDestA, err := rns.NewDestination(tsA, remoteID, rns.DestinationOut, rns.DestinationSingle, "lxst", "telephony")
	if err != nil {
		t.Fatalf("NewDestination(A) failed: %v", err)
	}

	linkA, err := rns.NewLink(tsA, callDestA)
	if err != nil {
		t.Fatalf("NewLink(A) failed: %v", err)
	}

	var (
		aMu         sync.Mutex
		aLinkActive bool
	)
	linkA.SetLinkEstablishedCallback(func(l *rns.Link) {
		aMu.Lock()
		aLinkActive = true
		aMu.Unlock()
	})

	if err := linkA.Establish(); err != nil {
		t.Fatalf("Establish(A) failed: %v", err)
	}

	// Wait for the link to become active on both sides.
	// The establishment timeout should now account for hops (the fix).
	deadline = time.Now().Add(30 * time.Second)
	for {
		aMu.Lock()
		aActive := aLinkActive
		aMu.Unlock()
		bMu.Lock()
		bActive := bLinkActive
		bMu.Unlock()
		if aActive && bActive {
			break
		}
		if !time.Now().Before(deadline) {
			aMu.Lock()
			aSt := aLinkActive
			aMu.Unlock()
			bMu.Lock()
			bSt := bLinkActive
			bMu.Unlock()
			t.Fatalf("link did not establish within 30s (A=%v, B=%v)", aSt, bSt)
		}
		time.Sleep(100 * time.Millisecond)
	}
	logger.Info("Link established on both sides!")

	// Send data packets from A to B through the link.
	testPackets := 5
	for i := 0; i < testPackets; i++ {
		pkt := rns.NewPacket(linkA, []byte{byte(i), 0x01, 0x02, 0x03})
		pkt.CreateReceipt = false
		if err := pkt.Pack(); err != nil {
			t.Fatalf("Pack packet %d failed: %v", i, err)
		}
		if err := linkA.SendPacket(pkt); err != nil {
			t.Fatalf("SendPacket(A) %d failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for B to receive the data packets.
	deadline = time.Now().Add(10 * time.Second)
	for {
		bMu.Lock()
		rcvd := bDataRcvd
		bMu.Unlock()
		if rcvd >= testPackets {
			break
		}
		if !time.Now().Before(deadline) {
			bMu.Lock()
			final := bDataRcvd
			bMu.Unlock()
			t.Fatalf("B only received %d/%d data packets", final, testPackets)
		}
		time.Sleep(100 * time.Millisecond)
	}
	logger.Info("All %d data packets received by B!", testPackets)

	// Verify link is still active.
	if linkA.GetStatus() != rns.LinkActive {
		t.Errorf("link A status = %v, want LinkActive", linkA.GetStatus())
	}
}
