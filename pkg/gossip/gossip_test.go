// Copyright 2014 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package gossip

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/retry"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// TestGossipInfoStore verifies operation of gossip instance infostore.
func TestGossipInfoStore(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)
	g := NewTest(1, stopper, metric.NewRegistry())
	slice := []byte("b")
	if err := g.AddInfo("s", slice, time.Hour); err != nil {
		t.Fatal(err)
	}
	if val, err := g.GetInfo("s"); !bytes.Equal(val, slice) || err != nil {
		t.Errorf("error fetching string: %v", err)
	}
	if _, err := g.GetInfo("s2"); err == nil {
		t.Errorf("expected error fetching nonexistent key \"s2\"")
	}

	g.mu.Lock()
	if info := g.mu.is.getInfo("s"); info == nil || info.TTLStamp == math.MaxInt64 {
		t.Errorf("expected info to be present and have finite TTL: %+v", info)
	}
	g.mu.Unlock()

	if err := g.AddInfoIfNotRedundant("s", slice); err != nil {
		t.Error(err)
	}
	if val, err := g.GetInfo("s"); !bytes.Equal(val, slice) || err != nil {
		t.Errorf("error fetching string: %v", err)
	}

	g.mu.Lock()
	if info := g.mu.is.getInfo("s"); info == nil || info.TTLStamp != math.MaxInt64 {
		t.Errorf("expected info be updated with an infinite TTL: %+v", info)
	}
	g.mu.Unlock()

	slice2 := []byte("b2")
	err := g.BulkAddInfoIfNotRedundant([]InfoToAdd{
		{Key: "s", Val: slice},
		{Key: "s2", Val: slice2},
	})
	if err != nil {
		t.Error(err)
	}
	if val, err := g.GetInfo("s"); !bytes.Equal(val, slice) || err != nil {
		t.Errorf("error fetching string: %v", err)
	}
	if val, err := g.GetInfo("s2"); !bytes.Equal(val, slice2) || err != nil {
		t.Errorf("error fetching string: %v", err)
	}

	g.mu.Lock()
	if info := g.mu.is.getInfo("s"); info == nil || info.TTLStamp != math.MaxInt64 {
		t.Errorf("expected info be updated with an infinite TTL: %+v", info)
	}
	if info := g.mu.is.getInfo("s2"); info == nil || info.TTLStamp != math.MaxInt64 {
		t.Errorf("expected info be written with an infinite TTL: %+v", info)
	}
	g.mu.Unlock()
}

// TestGossipMoveNode verifies that if a node is moved to a new address, it
// gets properly updated in gossip.
func TestGossipMoveNode(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)
	g := NewTest(1, stopper, metric.NewRegistry())
	var nodes []*roachpb.NodeDescriptor
	for i := 1; i <= 3; i++ {
		node := &roachpb.NodeDescriptor{
			NodeID:  roachpb.NodeID(i),
			Address: util.MakeUnresolvedAddr("tcp", fmt.Sprintf("1.1.1.1:%d", i)),
		}
		if err := g.SetNodeDescriptor(node); err != nil {
			t.Fatalf("failed setting node descriptor %+v: %s", node, err)
		}
		nodes = append(nodes, node)
	}
	for _, node := range nodes {
		if val, err := g.GetNodeDescriptor(node.NodeID); err != nil {
			t.Fatal(err)
		} else if !node.Equal(val) {
			t.Fatalf("expected node %+v, got %+v", node, val)
		}
	}

	// Move node 2 to the address of node 3.
	movedNode := nodes[1]
	replacedNode := nodes[2]
	movedNode.Address = replacedNode.Address
	if err := g.SetNodeDescriptor(movedNode); err != nil {
		t.Fatal(err)
	}

	testutils.SucceedsSoon(t, func() error {
		if val, err := g.GetNodeDescriptor(movedNode.NodeID); err != nil {
			return err
		} else if !movedNode.Equal(val) {
			return fmt.Errorf("expected node %+v, got %+v", movedNode, val)
		}
		return nil
	})
}

func TestGossipGetNextBootstrapAddress(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	addresses := []util.UnresolvedAddr{
		util.MakeUnresolvedAddr("tcp", "127.0.0.1:9000"),
		util.MakeUnresolvedAddr("tcp", "127.0.0.1:9001"),
		util.MakeUnresolvedAddr("tcp", "localhost:9004"),
	}

	g := NewTest(0, stopper, metric.NewRegistry())
	g.setAddresses(addresses)

	// Using specified addresses, fetch bootstrap addresses 3 times
	// and verify the results match expected addresses.
	expAddresses := []string{
		"127.0.0.1:9000",
		"127.0.0.1:9001",
		"localhost:9004",
	}
	for i := 0; i < len(expAddresses); i++ {
		g.mu.Lock()
		addr := g.getNextBootstrapAddressLocked()
		if addrStr := addr.String(); addrStr != expAddresses[i] {
			t.Errorf("%d: expected addr %s; got %s", i, expAddresses[i], addrStr)
		}
		g.mu.Unlock()
	}
}

func TestGossipLocalityResolver(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	gossipLocalityAdvertiseList := roachpb.Locality{}
	tier := roachpb.Tier{}
	tier.Key = "zone"
	tier.Value = "1"

	tier2 := roachpb.Tier{}
	tier2.Key = "zone"
	tier2.Value = "2"

	gossipLocalityAdvertiseList.Tiers = append(gossipLocalityAdvertiseList.Tiers, tier)

	node1PrivateAddress := util.MakeUnresolvedAddr("tcp", "1.0.0.1")
	node2PrivateAddress := util.MakeUnresolvedAddr("tcp", "2.0.0.1")

	node1PublicAddressRPC := util.MakeUnresolvedAddr("tcp", "1.1.1.1:1")
	node2PublicAddressRPC := util.MakeUnresolvedAddr("tcp", "2.2.2.2:3")
	node2PublicAddressSQL := util.MakeUnresolvedAddr("tcp", "2.2.2.2:4")

	var node1LocalityList []roachpb.LocalityAddress
	nodeLocalityAddress := roachpb.LocalityAddress{}
	nodeLocalityAddress.Address = node1PrivateAddress
	nodeLocalityAddress.LocalityTier = tier

	nodeLocalityAddress2 := roachpb.LocalityAddress{}
	nodeLocalityAddress2.Address = node2PrivateAddress
	nodeLocalityAddress2.LocalityTier = tier2

	node1LocalityList = append(node1LocalityList, nodeLocalityAddress)
	node1LocalityList = append(node1LocalityList, nodeLocalityAddress2)

	var node2LocalityList []roachpb.LocalityAddress
	node2LocalityList = append(node2LocalityList, nodeLocalityAddress2)

	g := NewTestWithLocality(1, stopper, metric.NewRegistry(), gossipLocalityAdvertiseList)
	node1 := &roachpb.NodeDescriptor{
		NodeID:          1,
		Address:         node1PublicAddressRPC,
		LocalityAddress: node1LocalityList,
	}
	node2 := &roachpb.NodeDescriptor{
		NodeID:          2,
		Address:         node2PublicAddressRPC,
		SQLAddress:      node2PublicAddressSQL,
		LocalityAddress: node2LocalityList,
	}

	if err := g.SetNodeDescriptor(node1); err != nil {
		t.Fatal(err)
	}
	if err := g.SetNodeDescriptor(node2); err != nil {
		t.Fatal(err)
	}

	nodeAddress, _, err := g.GetNodeIDAddress(node1.NodeID)
	if err != nil {
		t.Error(err)
	}
	if *nodeAddress != node1PrivateAddress {
		t.Fatalf("expected: %s but got: %s address", node1PrivateAddress, *nodeAddress)
	}

	nodeAddress, _, err = g.GetNodeIDAddress(node2.NodeID)
	if err != nil {
		t.Error(err)
	}

	if *nodeAddress != node2PublicAddressRPC {
		t.Fatalf("expected: %s but got: %s address", node2PublicAddressRPC, *nodeAddress)
	}

	nodeAddressSQL, _, err := g.GetNodeIDSQLAddress(node2.NodeID)
	if err != nil {
		t.Error(err)
	}

	if *nodeAddressSQL != node2PublicAddressSQL {
		t.Fatalf("expected: %s but got: %s address", node2PublicAddressSQL, *nodeAddressSQL)
	}
}

func TestGossipRaceLogStatus(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())
	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()
	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())

	local.mu.Lock()
	peer, _ := startGossip(clusterID, 2, stopper, t, metric.NewRegistry())
	local.startClientLocked(peer.mu.is.NodeAddr, roachpb.Locality{}, localCtx)
	local.mu.Unlock()

	// Race gossiping against LogStatus.
	gun := make(chan struct{})
	for i := uint8(0); i < 10; i++ {
		go func() {
			<-gun
			local.LogStatus()
			gun <- struct{}{}
		}()
		gun <- struct{}{}
		if err := local.AddInfo(
			strconv.FormatUint(uint64(i), 10),
			[]byte{i},
			time.Hour,
		); err != nil {
			t.Fatal(err)
		}
		<-gun
	}
	close(gun)
}

// TestGossipOutgoingLimitEnforced verifies that a gossip node won't open more
// outgoing connections than it should. If the gossip implementation is racy
// with respect to opening outgoing connections, this may not fail every time
// it's run, but should fail very quickly if run under stress.
func TestGossipOutgoingLimitEnforced(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// This test has an implicit dependency on the maxPeers logic deciding that
	// maxPeers is 3 for a 5-node cluster, so let's go ahead and make that
	// explicit.
	maxPeers := maxPeers(5)
	if maxPeers > 3 {
		t.Fatalf("maxPeers(5)=%d, which is higher than this test's assumption", maxPeers)
	}

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	local.mu.Lock()
	localAddr := local.mu.is.NodeAddr
	local.mu.Unlock()
	var peers []*Gossip
	for i := 0; i < 4; i++ {
		// After creating a new node, join it to the first node to ensure that the
		// network is connected (and thus all nodes know each other's addresses)
		// before we start the actual test.
		newPeer, peerCtx := startGossip(clusterID, roachpb.NodeID(i+2), stopper, t, metric.NewRegistry())
		newPeer.mu.Lock()
		newPeer.startClientLocked(localAddr, roachpb.Locality{}, peerCtx)
		newPeer.mu.Unlock()
		peers = append(peers, newPeer)
	}

	// Wait until the network is at least mostly connected.
	testutils.SucceedsSoon(t, func() error {
		local.mu.Lock()
		defer local.mu.Unlock()
		if local.mu.incoming.len() == maxPeers {
			return nil
		}
		return fmt.Errorf("local.mu.incoming.len() = %d, want %d", local.mu.incoming.len(), maxPeers)
	})

	// Verify that we can't open more than maxPeers connections. We have to muck
	// with the infostore's data so that the other nodes will appear far enough
	// away to be worth opening a connection to.
	local.mu.Lock()
	err := local.mu.is.visitInfos(func(key string, i *Info) error {
		copy := *i
		copy.Hops = maxHops + 1
		copy.Value.Timestamp.WallTime++
		return local.mu.is.addInfo(key, &copy)
	}, true /* deleteExpired */)
	local.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for range peers {
		local.tightenNetwork(context.Background(), localCtx)
	}

	if outgoing := local.outgoing.gauge.Value(); outgoing > int64(maxPeers) {
		t.Errorf("outgoing nodeSet has %d connections; the max should be %d", outgoing, maxPeers)
	}
	local.clientsMu.Lock()
	if numClients := len(local.clientsMu.clients); numClients > maxPeers {
		t.Errorf("local gossip has %d clients; the max should be %d", numClients, maxPeers)
	}
	local.clientsMu.Unlock()
}

func TestGossipMostDistant(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	connect := func(from, to *Gossip, fromCtx *rpc.Context) {
		to.mu.Lock()
		addr := to.mu.is.NodeAddr
		to.mu.Unlock()
		from.mu.Lock()
		from.startClientLocked(addr, roachpb.Locality{}, fromCtx)
		from.mu.Unlock()
	}

	mostDistant := func(g *Gossip) (roachpb.NodeID, uint32) {
		g.mu.Lock()
		distantNodeID, distantHops := g.mu.is.mostDistant(func(roachpb.NodeID) bool {
			return false
		})
		g.mu.Unlock()
		return distantNodeID, distantHops
	}

	const n = 10
	testCases := []struct {
		from, to int
	}{
		{0, n - 1}, // n1 connects to n10
		{n - 1, 0}, // n10 connects to n1
	}

	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			// Shared cluster ID by all gossipers (this ensures that the gossipers
			// don't talk to servers from unrelated tests by accident).
			clusterID := uuid.MakeV4()

			// Set up a gossip network of 10 nodes connected in a single line:
			//
			//   1 <- 2 <- 3 <- 4 <- 5 <- 6 <- 7 <- 8 <- 9 <- 10
			nodes := make([]*Gossip, n)
			nodesCtx := make([]*rpc.Context, n)
			for i := range nodes {
				nodes[i], nodesCtx[i] = startGossip(clusterID, roachpb.NodeID(i+1), stopper, t, metric.NewRegistry())
				if i == 0 {
					continue
				}
				connect(nodes[i], nodes[i-1], nodesCtx[i])
			}

			// Wait for n1 to determine that n10 is the most distant node.
			testutils.SucceedsSoon(t, func() error {
				g := nodes[0]
				distantNodeID, distantHops := mostDistant(g)
				if distantNodeID == 10 && distantHops == 9 {
					return nil
				}
				return fmt.Errorf("n%d: distantHops: %d from n%d", g.NodeID.Get(), distantHops, distantNodeID)
			})
			// Wait for the infos to be fully propagated.
			testutils.SucceedsSoon(t, func() error {
				infosCount := func(g *Gossip) int {
					g.mu.Lock()
					defer g.mu.Unlock()
					return len(g.mu.is.Infos)
				}
				count := infosCount(nodes[0])
				for _, g := range nodes[1:] {
					if tmp := infosCount(g); tmp != count {
						return fmt.Errorf("unexpected info count: %d != %d", tmp, count)
					}
				}
				return nil
			})

			// Connect the network in a loop. This will cut the distance to the most
			// distant node in half.
			log.Infof(context.Background(), "connecting from n%d to n%d", c.from, c.to)
			connect(nodes[c.from], nodes[c.to], nodesCtx[c.from])

			// Wait for n1 to determine that n6 is now the most distant hops from 9
			// to 5 and change the most distant node to n6.
			testutils.SucceedsSoon(t, func() error {
				g := nodes[0]
				g.mu.Lock()
				var buf bytes.Buffer
				_ = g.mu.is.visitInfos(func(key string, i *Info) error {
					if i.NodeID != 1 && IsNodeDescKey(key) {
						fmt.Fprintf(&buf, "n%d: hops=%d\n", i.NodeID, i.Hops)
					}
					return nil
				}, true /* deleteExpired */)
				g.mu.Unlock()

				distantNodeID, distantHops := mostDistant(g)
				if distantNodeID == 6 && distantHops == 5 {
					return nil
				}
				return fmt.Errorf("n%d: distantHops: %d from n%d\n%s",
					g.NodeID.Get(), distantHops, distantNodeID, buf.String())
			})
		})
	}
}

// TestGossipNoForwardSelf verifies that when a Gossip instance is full, it
// redirects clients elsewhere (in particular not to itself).
//
// NB: Stress testing this test really stresses the OS networking stack
// more than anything else. For example, on Linux it may quickly deplete
// the ephemeral port range (due to the TIME_WAIT state).
// On a box which only runs tests, this can be circumvented by running
//
//	sudo bash -c "echo 1 > /proc/sys/net/ipv4/tcp_tw_recycle"
//
// See https://vincent.bernat.im/en/blog/2014-tcp-time-wait-state-linux.html
// for details.
//
// On OSX, things similarly fall apart. See #7524 and #5218 for some discussion
// of this.
func TestGossipNoForwardSelf(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start one loopback client plus enough additional clients to fill the
	// incoming clients.
	peers := []*Gossip{local}
	peerCtx := []*rpc.Context{localCtx}

	local.server.mu.Lock()
	maxSize := local.server.mu.incoming.maxSize
	local.server.mu.Unlock()
	for i := 0; i < maxSize; i++ {
		g, gCtx := startGossip(clusterID, roachpb.NodeID(i+2), stopper, t, metric.NewRegistry())
		peers = append(peers, g)
		peerCtx = append(peerCtx, gCtx)
	}

	for i, peer := range peers {
		c := newClient(log.MakeTestingAmbientCtxWithNewTracer(), local.GetNodeAddr(), roachpb.Locality{}, makeMetrics())

		testutils.SucceedsSoon(t, func() error {
			conn, err := peerCtx[i].GRPCUnvalidatedDial(c.addr.String(), c.locality).Connect(ctx)
			if err != nil {
				return err
			}

			stream, err := NewGRPCGossipClientAdapter(conn).Gossip(ctx)
			if err != nil {
				return err
			}

			if err := c.requestGossip(peer, stream); err != nil {
				return err
			}

			// Wait until the server responds, so we know we're connected.
			_, err = stream.Recv()
			return err
		})
	}

	numClients := len(peers) * 2
	disconnectedCh := make(chan *client)

	// Start a few overflow peers and assert that they don't get forwarded to us
	// again.
	for i := 0; i < numClients; i++ {
		local.server.mu.Lock()
		maxSize := local.server.mu.incoming.maxSize
		local.server.mu.Unlock()
		peer, peerCtx := startGossip(clusterID, roachpb.NodeID(i+maxSize+2), stopper, t, metric.NewRegistry())

		for {
			localAddr := local.GetNodeAddr()
			c := newClient(log.MakeTestingAmbientCtxWithNewTracer(), localAddr, roachpb.Locality{}, makeMetrics())
			peer.mu.Lock()
			c.startLocked(peer, disconnectedCh, peerCtx, stopper)
			peer.mu.Unlock()

			disconnectedClient := <-disconnectedCh
			if disconnectedClient != c {
				t.Fatalf("expected %p to be disconnected, got %p", c, disconnectedClient)
			} else if c.forwardAddr == nil {
				// Under high load, clients sometimes fail to connect for reasons
				// unrelated to the test, so we need to permit some.
				t.Logf("node #%d: got nil forwarding address", peer.NodeID.Get())
				continue
			} else if *c.forwardAddr == *localAddr {
				t.Errorf("node #%d: got local's forwarding address", peer.NodeID.Get())
			}
			break
		}
	}
}

// TestGossipCullNetwork verifies that a client will be culled from
// the network periodically (at cullInterval duration intervals).
func TestGossipCullNetwork(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	local.SetCullInterval(5 * time.Millisecond)

	local.mu.Lock()
	for i := 0; i < minPeers; i++ {
		peer, peerCtx := startGossip(clusterID, roachpb.NodeID(i+2), stopper, t, metric.NewRegistry())
		local.startClientLocked(*peer.GetNodeAddr(), roachpb.Locality{}, peerCtx)
	}
	local.mu.Unlock()

	const slowGossipDuration = time.Minute

	if err := retry.ForDuration(slowGossipDuration, func() error {
		if peers := len(local.Outgoing()); peers != minPeers {
			return errors.Errorf("%d of %d peers connected", peers, minPeers)
		}
		return nil
	}); err != nil {
		t.Fatalf("condition failed to evaluate within %s: %s", slowGossipDuration, err)
	}

	local.manage(localCtx)

	if err := retry.ForDuration(slowGossipDuration, func() error {
		// Verify that a client is closed within the cull interval.
		if peers := len(local.Outgoing()); peers != minPeers-1 {
			return errors.Errorf("%d of %d peers connected", peers, minPeers-1)
		}
		return nil
	}); err != nil {
		t.Fatalf("condition failed to evaluate within %s: %s", slowGossipDuration, err)
	}
}

func TestGossipOrphanedStallDetection(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	local.SetStallInterval(5 * time.Millisecond)

	// Make sure we have the sentinel to ensure that its absence is not the
	// cause of stall detection.
	if err := local.AddInfo(KeySentinel, nil, time.Hour); err != nil {
		t.Fatal(err)
	}

	peerStopper := stop.NewStopper()
	peer, _ := startGossip(clusterID, 2, peerStopper, t, metric.NewRegistry())

	peerNodeID := peer.NodeID.Get()
	peerAddr := peer.GetNodeAddr()
	peerAddrStr := peerAddr.String()

	local.mu.Lock()
	local.startClientLocked(*peerAddr, roachpb.Locality{}, localCtx)
	local.mu.Unlock()

	testutils.SucceedsSoon(t, func() error {
		for _, peerID := range local.Outgoing() {
			if peerID == peerNodeID {
				return nil
			}
		}
		return errors.Errorf("n%d not yet connected", peerNodeID)
	})

	testutils.SucceedsSoon(t, func() error {
		for _, addr := range local.GetAddresses() {
			if addr.String() == peerAddrStr {
				return nil
			}
		}
		return errors.Errorf("n%d descriptor not yet available", peerNodeID)
	})

	local.bootstrap(localCtx)
	local.manage(localCtx)

	peerStopper.Stop(context.Background())

	testutils.SucceedsSoon(t, func() error {
		for _, peerID := range local.Outgoing() {
			if peerID == peerNodeID {
				return errors.Errorf("n%d still connected", peerNodeID)
			}
		}
		return nil
	})

	peerStopper = stop.NewStopper()
	defer peerStopper.Stop(context.Background())
	startGossipAtAddr(clusterID, peerNodeID, peerAddr, peerStopper, t, metric.NewRegistry())

	testutils.SucceedsSoon(t, func() error {
		for _, peerID := range local.Outgoing() {
			if peerID == peerNodeID {
				return nil
			}
		}
		return errors.Errorf("n%d not yet connected", peerNodeID)
	})
}

// TestGossipCantJoinTwoClusters verifies that a node can't
// participate in two separate clusters if two nodes from different
// clusters are specified as bootstrap hosts. Previously, this would
// be allowed, because a node verifies the cluster ID only at startup.
// If after joining the first cluster via that cluster's init node,
// the init node shuts down, the joining node will reconnect via its
// second bootstrap host and begin to participate [illegally] in
// another cluster.
func TestGossipJoinTwoClusters(t *testing.T) {
	defer leaktest.AfterTest(t)()

	const interval = 10 * time.Millisecond
	var stoppers []*stop.Stopper
	var g []*Gossip
	var clusterIDs []uuid.UUID
	var addrs []net.Addr

	ctx := context.Background()

	// Create three gossip nodes, init the first two with no bootstrap
	// hosts, but unique cluster IDs. The third host has the first two
	// hosts as bootstrap hosts, but has the same cluster ID as the
	// first of its bootstrap hosts.
	for i := 0; i < 3; i++ {
		stopper := stop.NewStopper()
		stoppers = append(stoppers, stopper)
		defer func() {
			select {
			case <-stopper.ShouldQuiesce():
			default:
				stopper.Stop(ctx)
			}
		}()

		var clusterID uuid.UUID
		switch i {
		case 0, 1:
			clusterID = uuid.MakeV4()
		case 2:
			clusterID = clusterIDs[0]
		}
		clusterIDs = append(clusterIDs, clusterID)
		clock := hlc.NewClockForTesting(nil)
		rpcContext := rpc.NewInsecureTestingContextWithClusterID(ctx, clock, stopper, clusterID)

		server, err := rpc.NewServer(ctx, rpcContext)
		require.NoError(t, err)

		// node ID must be non-zero
		gnode := NewTest(roachpb.NodeID(i+1), stopper, metric.NewRegistry())
		RegisterGossipServer(server, gnode)
		g = append(g, gnode)
		gnode.SetStallInterval(interval)
		gnode.SetBootstrapInterval(interval)
		gnode.clusterID.Set(context.Background(), clusterIDs[i])

		ln, err := netutil.ListenAndServeGRPC(stopper, server, util.IsolatedTestAddr)
		require.NoError(t, err)
		addrs = append(addrs, ln.Addr())

		// Only the third node has addresses.
		var addresses []util.UnresolvedAddr
		switch i {
		case 2:
			for j := 0; j < 2; j++ {
				addresses = append(addresses, util.MakeUnresolvedAddr("tcp", addrs[j].String()))
			}
		}
		gnode.Start(ln.Addr(), addresses, rpcContext)
	}

	// Wait for connections.
	testutils.SucceedsSoon(t, func() error {
		// The first gossip node should have one gossip client address
		// in nodeMap if the 2nd gossip node connected. The second gossip
		// node should have none.
		g[0].mu.Lock()
		defer g[0].mu.Unlock()
		if a, e := len(g[0].mu.nodeMap), 1; a != e {
			return errors.Errorf("expected %v to contain %d nodes, got %d", g[0].mu.nodeMap, e, a)
		}
		g[1].mu.Lock()
		defer g[1].mu.Unlock()
		if a, e := len(g[1].mu.nodeMap), 0; a != e {
			return errors.Errorf("expected %v to contain %d nodes, got %d", g[1].mu.nodeMap, e, a)
		}
		return nil
	})

	// Kill node 0 to force node 2 to bootstrap with node 1.
	stoppers[0].Stop(ctx)
	// Wait for twice the bootstrap interval, and verify that
	// node 2 still has not connected to node 1.
	time.Sleep(2 * interval)

	g[1].mu.Lock()
	if a, e := len(g[1].mu.nodeMap), 0; a != e {
		t.Errorf("expected %v to contain %d nodes, got %d", g[1].mu.nodeMap, e, a)
	}
	g[1].mu.Unlock()
}

// Test propagation of gossip infos in both directions across an existing
// gossip connection.
func TestGossipPropagation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	remote, remoteCtx := startGossip(clusterID, 2, stopper, t, metric.NewRegistry())
	remote.mu.Lock()
	rAddr := remote.mu.is.NodeAddr
	remote.mu.Unlock()
	local.manage(localCtx)
	remote.manage(remoteCtx)

	mustAdd := func(g *Gossip, key string, val []byte, ttl time.Duration) {
		if err := g.AddInfo(key, val, ttl); err != nil {
			t.Fatal(err)
		}
	}

	// Gossip a key on local and wait for it to show up on remote. This
	// guarantees we have an active local to remote client connection.
	mustAdd(local, "bootstrap", nil, 0)
	testutils.SucceedsSoon(t, func() error {
		c := local.findClient(func(c *client) bool { return c.addr.String() == rAddr.String() })
		if c == nil {
			// Restart the client connection in the loop. It might have failed due to
			// a heartbeat timeout.
			local.mu.Lock()
			local.startClientLocked(rAddr, roachpb.Locality{}, localCtx)
			local.mu.Unlock()
			return fmt.Errorf("unable to find local to remote client")
		}
		_, err := remote.GetInfo("bootstrap")
		return err
	})

	// Add entries on both the local and remote nodes and verify they get propagated.
	mustAdd(local, "local", nil, time.Minute)
	mustAdd(remote, "remote", nil, time.Minute)

	getInfo := func(g *Gossip, key string) *Info {
		g.mu.RLock()
		defer g.mu.RUnlock()
		return g.mu.is.getInfo(key)
	}

	var localInfo *Info
	var remoteInfo *Info
	testutils.SucceedsSoon(t, func() error {
		localInfo = getInfo(remote, "local")
		if localInfo == nil {
			return fmt.Errorf("local info not propagated")
		}
		remoteInfo = getInfo(local, "remote")
		if remoteInfo == nil {
			return fmt.Errorf("remote info not propagated")
		}
		return nil
	})

	// Replace the existing entries on both the local and remote nodes and verify
	// these new entries get propagated with updated timestamps.
	mustAdd(local, "local", nil, 2*time.Minute)
	mustAdd(remote, "remote", nil, 2*time.Minute)

	testutils.SucceedsSoon(t, func() error {
		if i := getInfo(remote, "local"); i == nil || reflect.DeepEqual(i, localInfo) {
			return fmt.Errorf("new local info not propagated:\n%v\n%v", i, localInfo)
		}
		if i := getInfo(local, "remote"); reflect.DeepEqual(i, remoteInfo) {
			return fmt.Errorf("new remote info not propagated:\n%v\n%v", i, remoteInfo)
		}
		return nil
	})

	mustClear := func(g *Gossip, key string) {
		if _, err := g.tryClearInfoWithTTL(key, 3*time.Second); err != nil {
			t.Fatal(err)
		}
	}

	// Clear both entries. Verify that both are removed from the gossip network.
	mustClear(local, "local")
	mustClear(remote, "remote")
	testutils.SucceedsSoon(t, func() error {
		for gName, g := range map[string]*Gossip{"local": local, "remote": remote} {
			for _, key := range []string{"local", "remote"} {
				if getInfo(g, key) != nil {
					return fmt.Errorf("%s info %q not cleared", gName, key)
				}
			}
		}
		return nil
	})
}

// Test whether propagation of an info that was generated by a prior
// incarnation of a server can correctly be sent back to that originating
// server. Consider the scenario:
//
// n1: decommissioned
// n2: gossip node-liveness:1
// n3: node-liveness range lease acquired (does not gossip node-liveness:1
//
//	record because it is unchanged)
//
// n2: restarted
//   - connects as gossip client to n3
//   - sends a batch of gossip records to n3
//   - n3 responds without sending node-liveness:1 because it's
//     OrigStamp is less than the highwater stamp from n2
func TestGossipLoopbackInfoPropagation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	skip.WithIssue(t, 34494)
	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	remote, remoteCtx := startGossip(clusterID, 2, stopper, t, metric.NewRegistry())
	remote.mu.Lock()
	rAddr := remote.mu.is.NodeAddr
	remote.mu.Unlock()
	local.manage(localCtx)
	remote.manage(remoteCtx)

	// Add a gossip info for "foo" on remote, that was generated by local. This
	// simulates what happens if local was to gossip an info, and later restart
	// and never gossip that info again.
	func() {
		local.mu.Lock()
		defer local.mu.Unlock()
		remote.mu.Lock()
		defer remote.mu.Unlock()
		// NB: replacing local.mu.is.newInfo with remote.mu.is.newInfo allows "foo"
		// to be propagated.
		if err := remote.mu.is.addInfo("foo", local.mu.is.newInfo(nil, 0)); err != nil {
			t.Fatal(err)
		}
	}()

	// Add an info to local so that it has a highwater timestamp that is newer
	// than the info we added to remote. NB: commenting out this line allows
	// "foo" to be propagated.
	if err := local.AddInfo("bar", nil, 0); err != nil {
		t.Fatal(err)
	}

	// Start a client connection to the remote node.
	local.mu.Lock()
	local.startClientLocked(rAddr, roachpb.Locality{}, localCtx)
	local.mu.Unlock()

	getInfo := func(g *Gossip, key string) *Info {
		g.mu.RLock()
		defer g.mu.RUnlock()
		return g.mu.is.Infos[key]
	}

	testutils.SucceedsSoon(t, func() error {
		if getInfo(remote, "bar") == nil {
			return fmt.Errorf("bar not propagated")
		}
		if getInfo(local, "foo") == nil {
			return fmt.Errorf("foo not propagated")
		}
		return nil
	})
}

// TestServerSendsHighStampsDiff tests that the server sends high water stamps
// diffs to the client rather than sending the whole map.
func TestServerSendsHighStampsDiff(t *testing.T) {
	defer leaktest.AfterTest(t)()

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers (this ensures that the gossipers
	// don't talk to servers from unrelated tests by accident).
	clusterID := uuid.MakeV4()

	// Start local and remote gossip servers.
	local, localCxt := startGossip(clusterID, 1 /* nodeID */, stopper, t, metric.NewRegistry())
	remote, remoteCxt := startGossip(clusterID, 2 /* nodeID */, stopper, t, metric.NewRegistry())
	local.manage(localCxt)
	remote.manage(remoteCxt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a client to the remote node.
	c := newClient(log.MakeTestingAmbientCtxWithNewTracer(), remote.GetNodeAddr(), roachpb.Locality{}, makeMetrics())

	conn, err := localCxt.GRPCUnvalidatedDial(c.addr.String(), roachpb.Locality{}).Connect(ctx)
	require.NoError(t, err)

	stream, err := NewGRPCGossipClientAdapter(conn).Gossip(ctx)
	require.NoError(t, err)

	requestGossip := func(g *Gossip, stream RPCGossip_GossipClient) Response {
		err := c.requestGossip(g, stream)
		require.NoError(t, err)
		resp := &Response{}
		resp, err = stream.Recv()
		require.NoError(t, err)
		return *resp
	}

	// Expect that the server will return its high water stamps in the response.
	testutils.SucceedsSoon(t, func() error {
		resp := requestGossip(local, stream)
		local.mu.Lock()
		currentHighStamps := remote.mu.is.getHighWaterStamps()
		local.mu.Unlock()
		if !reflect.DeepEqual(resp.HighWaterStamps, currentHighStamps) {
			return errors.Errorf(
				"Expected to receive the server's high water stamps: %+v but received %+v instead.",
				remote.mu.is.getHighWaterStamps(), resp.HighWaterStamps)
		}
		return nil
	})

	// Since the server high water stamps haven't changed, expect the server to
	// return an empty map.
	resp := requestGossip(local, stream)
	require.Empty(t, resp.HighWaterStamps)

	// Add some info to the server. This causes an increase in the high water
	// time stamp.
	err = remote.AddInfo("remote", nil, time.Hour)
	require.NoError(t, err)

	testutils.SucceedsSoon(t, func() error {
		resp := requestGossip(local, stream)
		local.mu.Lock()
		currentHighStamps := remote.mu.is.getHighWaterStamps()
		local.mu.Unlock()

		if !reflect.DeepEqual(resp.HighWaterStamps, currentHighStamps) {
			return errors.Errorf(
				"Expected to receive the server's high water stamps: %+v but received %+v instead.",
				remote.mu.is.getHighWaterStamps(), resp.HighWaterStamps)
		}
		return nil
	})
}

// TestGossipBatching verifies that both server and client gossip updates are
// batched.
func TestGossipBatching(t *testing.T) {
	defer leaktest.AfterTest(t)()
	skip.UnderDeadlock(t, "might be flaky since it relies on some upper-bound timing")
	skip.UnderRace(t, "might be flaky since it relies on some upper-bound timing")

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	// Shared cluster ID by all gossipers
	clusterID := uuid.MakeV4()

	local, localCtx := startGossip(clusterID, 1, stopper, t, metric.NewRegistry())
	remote, remoteCtx := startGossip(clusterID, 2, stopper, t, metric.NewRegistry())
	remote.mu.Lock()
	rAddr := remote.mu.is.NodeAddr
	remote.mu.Unlock()
	local.manage(localCtx)
	remote.manage(remoteCtx)

	// Start a client connection to the remote node
	local.mu.Lock()
	local.startClientLocked(rAddr, roachpb.Locality{}, localCtx)
	local.mu.Unlock()

	// Wait for connection to be established
	var c *client
	testutils.SucceedsSoon(t, func() error {
		c = local.findClient(func(c *client) bool { return c.addr.String() == rAddr.String() })
		if c == nil {
			return fmt.Errorf("client not found")
		}
		return nil
	})

	// Prepare 10,000 keys to gossip. This is a large enough number to allow
	// batching to kick in.
	numKeys := 10_000
	localKeys := make([]string, numKeys)
	remoteKeys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		localKeys[i] = fmt.Sprintf("local-key-%d", i)
		remoteKeys[i] = fmt.Sprintf("remote-key-%d", i)
	}

	// Gossip the keys to both local and remote nodes.
	for i := range numKeys {
		require.NoError(t, local.AddInfo(localKeys[i], []byte("value"), time.Hour))
		require.NoError(t, remote.AddInfo(remoteKeys[i], []byte("value"), time.Hour))
	}

	// Wait for updates to propagate
	testutils.SucceedsSoon(t, func() error {
		for i := range numKeys {
			if _, err := local.GetInfo(remoteKeys[i]); err != nil {
				return err
			}
			if _, err := remote.GetInfo(localKeys[i]); err != nil {
				return err
			}
		}
		return nil
	})

	// Record the number of messages both the client and the server sent, and
	// assert that it's within the expected bounds.
	serverMessagesSentCount := remote.serverMetrics.MessagesSent.Count()
	clientMessagesSentCount := local.serverMetrics.MessagesSent.Count()

	fmt.Printf("client msgs sent: %+v\n", clientMessagesSentCount)
	fmt.Printf("server msgs sent: %+v\n", serverMessagesSentCount)

	// upperBoundMessages is the maximum number of sent messages we expect to see.
	// Note that in reality with batching, we see 3-10 messages sent in this test,
	// However, in order to avoid flakiness, we set a very high number here. The
	// test would fail even with this high number if we don't have batching.
	upperBoundMessages := int64(500)
	require.LessOrEqual(t, serverMessagesSentCount, upperBoundMessages)
	require.LessOrEqual(t, clientMessagesSentCount, upperBoundMessages)
}

// TestCallbacksPendingMetricGoesToZeroOnStop verifies that the CallbacksPending
// metric is correctly decremented when a callback is unregistered with pending work
// or when the stopper is stopped.
func TestCallbacksPendingMetricGoesToZeroOnStop(t *testing.T) {
	defer leaktest.AfterTest(t)()

	testCases := []struct {
		name    string
		cleanup func(g *Gossip, unregister func(), stopper *stop.Stopper, ctx context.Context)
	}{
		{
			name: "unregister callback",
			cleanup: func(g *Gossip, unregister func(), stopper *stop.Stopper, ctx context.Context) {
				unregister()
			},
		},
		{
			name: "stopper shutdown",
			cleanup: func(g *Gossip, unregister func(), stopper *stop.Stopper, ctx context.Context) {
				stopper.Stop(ctx)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			stopper := stop.NewStopper()
			defer stopper.Stop(ctx)
			g := NewTest(1, stopper, metric.NewRegistry())

			unregister := g.RegisterCallback("test.*", func(key string, val roachpb.Value) {
				// Do nothing.
			})

			// Add 100 infos to the gossip that will be processed by the callback.
			for i := 0; i < 100; i++ {
				slice := []byte("b1")
				require.NoError(t, g.AddInfo(fmt.Sprintf("test.key%d", i), slice, time.Hour))
			}

			// Execute the cleanup action (either unregister or stopper.Stop)
			// We do this in a goroutine to help cause interesting potential race conditions.
			go func() {
				tc.cleanup(g, unregister, stopper, ctx)
			}()

			// Add another 100 infos to the gossip that will be processed by the callback.
			// We do this in a goroutine to help cause interesting potential race conditions.
			go func() {
				for i := 0; i < 100; i++ {
					slice := []byte("b2")
					require.NoError(t, g.AddInfo(fmt.Sprintf("test.key%d", i), slice, time.Hour))
				}
			}()

			// Wait for the pending callbacks metric to go to 0.
			testutils.SucceedsSoon(t, func() error {
				if g.mu.is.metrics.CallbacksPending.Value() != 0 {
					return fmt.Errorf("CallbacksPending should be 0, got %d", g.mu.is.metrics.CallbacksPending.Value())
				}
				return nil
			})
		})
	}
}
