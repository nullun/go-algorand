// Copyright (C) 2019-2026 Algorand Foundation Ltd.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package network

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	lpmetrics "github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestP2PBandwidthCounterConcurrentConnections(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	c := makePeerBandwidthCounter()
	p := peer.ID("shared")
	// Keep one connection alive while other connections to the same peer
	// open, exchange traffic, and close concurrently. None should reset or
	// prematurely remove the shared totals.
	c.addConn(p)
	const workers, iterations = 16, 1000
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range iterations {
				c.addConn(p)
				c.LogRecvMessageStream(1, "/test", p)
				c.LogSentMessageStream(2, "/test", p)
				c.removeConn(p)
			}
		})
	}
	wg.Go(func() {
		for range iterations {
			c.bytesForPeer(p)
			c.GetBandwidthByPeer()
		}
	})
	wg.Wait()

	in, out := c.bytesForPeer(p)
	require.EqualValues(t, workers*iterations, in)
	require.EqualValues(t, 2*workers*iterations, out)
	c.removeConn(p)
	require.Empty(t, c.GetBandwidthByPeer())
}

// BenchmarkP2PBandwidthCounter measures the reporter calls made by each
// libp2p stream write, with both one shared peer and independent peers. Use
// -cpu=1,8 to distinguish uncontended cost from cross-peer contention.
func BenchmarkP2PBandwidthCounter(b *testing.B) {
	for _, numPeers := range []int{1, 128} {
		b.Run(fmt.Sprintf("peers=%d", numPeers), func(b *testing.B) {
			c := makePeerBandwidthCounter()
			peers := make([]peer.ID, numPeers)
			for i := range peers {
				// Match the length of an Ed25519 peer ID's binary representation.
				peers[i] = peer.ID(fmt.Sprintf("\x00\x24\x08\x01\x12\x20%032d", i))
				c.addConn(peers[i])
			}
			var reporter lpmetrics.Reporter = c
			var next atomic.Uint64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				p := peers[(next.Add(1)-1)%uint64(len(peers))]
				for pb.Next() {
					reporter.LogSentMessage(1024)
					reporter.LogSentMessageStream(1024, "/test", p)
				}
			})
		})
	}
}
