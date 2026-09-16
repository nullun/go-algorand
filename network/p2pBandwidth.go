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
	"sync"
	"sync/atomic"
	"time"

	lpmetrics "github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

// peerBandwidthCounter accumulates byte totals per remote peer for all libp2p
// traffic.
//
// It replaces lpmetrics.BandwidthCounter, which never forgets a peer: its
// registries gain an entry for every distinct peer ID the host exchanges bytes
// with and keep it for the lifetime of the process. Its TrimIdle cannot be used
// to bound that, because go-flow-metrics v0.3.0 collects the entries to drop by
// appending the cutoff timestamp instead of the registry key, so it deletes
// nothing. A peer cycling through identities would therefore grow the host's
// memory without ever exceeding the concurrent connection limits.
//
// This counter instead keeps an entry only while the host holds at least one
// connection to the peer: the entry is created when the first connection to a
// peer opens and dropped when the last one closes, so its retention is bounded
// by the peers currently connected. Bytes logged for a peer with no entry are
// discarded, which is what keeps a stream read
// that completes after its connection is gone - libp2p reports even a failed,
// zero-byte read - from resurrecting a dropped peer. The per-peer totals are
// approximate indicators for the current connections, not byte-perfect
// accounting. Initial traffic can precede the connection notification and be
// missed; a rapid reconnect can retain the previous totals if its notification
// arrives before the old connection's disconnection notification.
//
// Only per-peer totals are tracked. The host-wide and per-protocol accessors
// required by lpmetrics.Reporter report nothing: nothing in the node consumes
// them, a host-wide total would be a counter every stream write on the host
// contends for, and a per-protocol registry has the same unbounded retention
// problem for stream protocol IDs chosen by the remote.
type peerBandwidthCounter struct {
	mu    sync.Mutex // serializes connection bookkeeping; not taken to read totals
	peers sync.Map   // peer.ID -> *peerBandwidth; reads do not take mu
}

// peerBandwidth holds the byte totals exchanged with a single remote peer, and
// how many connections to it the host holds. conns is guarded by the counter's
// mutex; the totals are atomic so that logging bytes does not take that lock.
type peerBandwidth struct {
	conns int

	in  atomic.Uint64
	out atomic.Uint64
}

// interfaces implemented by peerBandwidthCounter: it is both the host's
// bandwidth reporter and the notifiee that prunes it.
var _ lpmetrics.Reporter = (*peerBandwidthCounter)(nil)
var _ network.Notifiee = (*peerBandwidthCounter)(nil)

func makePeerBandwidthCounter() *peerBandwidthCounter {
	return &peerBandwidthCounter{}
}

// forPeer returns the counters for p, or nil if the host holds no connection
// to p. Counters are created by addConn, never here: bytes belonging to a
// connection that is already gone must not bring a peer back. The lookup
// takes no lock, so callers already holding mu may use it.
func (c *peerBandwidthCounter) forPeer(p peer.ID) *peerBandwidth {
	if pb, ok := c.peers.Load(p); ok {
		return pb.(*peerBandwidth)
	}
	return nil
}

// addConn records one more open connection to p, creating its counters if this
// is the first.
func (c *peerBandwidthCounter) addConn(p peer.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pb := c.forPeer(p)
	if pb == nil {
		pb = &peerBandwidth{}
		c.peers.Store(p, pb)
	}
	pb.conns++
}

// removeConn records one fewer open connection to p, dropping its counters
// with the last one.
func (c *peerBandwidthCounter) removeConn(p peer.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pb := c.forPeer(p)
	if pb == nil {
		return
	}
	pb.conns--
	if pb.conns <= 0 {
		c.peers.Delete(p)
	}
}

// watch starts counting the connections net already holds, and must be called
// when the counter is installed as a notifiee so that connections opened before
// it was registered are matched by their close notifications.
func (c *peerBandwidthCounter) watch(net network.Network) {
	for _, conn := range net.Conns() {
		c.addConn(conn.RemotePeer())
	}
}

// bytesForPeer returns the bytes received from and sent to p.
func (c *peerBandwidthCounter) bytesForPeer(p peer.ID) (in, out uint64) {
	if pb := c.forPeer(p); pb != nil {
		return pb.in.Load(), pb.out.Load()
	}
	return 0, 0
}

// LogSentMessage does nothing: host-wide totals are not tracked. libp2p calls
// this for every stream write, alongside LogSentMessageStream, so a total here
// would be a single counter that every stream on the host contends for.
func (c *peerBandwidthCounter) LogSentMessage(int64) {}

// LogRecvMessage does nothing: host-wide totals are not tracked, see
// LogSentMessage.
func (c *peerBandwidthCounter) LogRecvMessage(int64) {}

// LogSentMessageStream records bytes sent to a peer over a stream.
func (c *peerBandwidthCounter) LogSentMessageStream(size int64, _ protocol.ID, p peer.ID) {
	if pb := c.forPeer(p); pb != nil {
		pb.out.Add(uint64(size))
	}
}

// LogRecvMessageStream records bytes received from a peer over a stream.
func (c *peerBandwidthCounter) LogRecvMessageStream(size int64, _ protocol.ID, p peer.ID) {
	if pb := c.forPeer(p); pb != nil {
		pb.in.Add(uint64(size))
	}
}

// GetBandwidthForPeer returns the byte totals exchanged with p. Rates are not
// tracked and are reported as zero.
func (c *peerBandwidthCounter) GetBandwidthForPeer(p peer.ID) lpmetrics.Stats {
	in, out := c.bytesForPeer(p)
	return lpmetrics.Stats{TotalIn: int64(in), TotalOut: int64(out)}
}

// GetBandwidthForProtocol is not tracked and always reports zero.
func (c *peerBandwidthCounter) GetBandwidthForProtocol(protocol.ID) lpmetrics.Stats {
	return lpmetrics.Stats{}
}

// GetBandwidthTotals is not tracked and always reports zero.
func (c *peerBandwidthCounter) GetBandwidthTotals() lpmetrics.Stats {
	return lpmetrics.Stats{}
}

// GetBandwidthByPeer returns the byte totals of every peer currently counted.
// The snapshot is not taken at a single instant: peers may connect or leave,
// and totals may advance, while it is assembled.
func (c *peerBandwidthCounter) GetBandwidthByPeer() map[peer.ID]lpmetrics.Stats {
	stats := make(map[peer.ID]lpmetrics.Stats)
	c.peers.Range(func(key, value any) bool {
		pb := value.(*peerBandwidth)
		stats[key.(peer.ID)] = lpmetrics.Stats{TotalIn: int64(pb.in.Load()), TotalOut: int64(pb.out.Load())}
		return true
	})
	return stats
}

// GetBandwidthByProtocol is not tracked and always reports an empty map.
func (c *peerBandwidthCounter) GetBandwidthByProtocol() map[protocol.ID]lpmetrics.Stats {
	return map[protocol.ID]lpmetrics.Stats{}
}

// Reset zeroes every counter. The peers themselves are kept, so that the
// connection bookkeeping stays in step with the host.
func (c *peerBandwidthCounter) Reset() {
	c.peers.Range(func(_, value any) bool {
		pb := value.(*peerBandwidth)
		pb.in.Store(0)
		pb.out.Store(0)
		return true
	})
}

// TrimIdle does nothing: retention is bounded by the host's open connections
// rather than by idle time.
func (c *peerBandwidthCounter) TrimIdle(time.Time) {}

// Listen implements network.Notifiee.
func (c *peerBandwidthCounter) Listen(network.Network, multiaddr.Multiaddr) {}

// ListenClose implements network.Notifiee.
func (c *peerBandwidthCounter) ListenClose(network.Network, multiaddr.Multiaddr) {}

// Connected starts counting a connection's traffic. Other libp2p notifiees can
// start streams before this callback runs, so initial bytes may be missed.
func (c *peerBandwidthCounter) Connected(_ network.Network, conn network.Conn) {
	c.addConn(conn.RemotePeer())
}

// Disconnected stops counting a connection's traffic, dropping the peer's
// counters once its last connection is gone.
func (c *peerBandwidthCounter) Disconnected(_ network.Network, conn network.Conn) {
	c.removeConn(conn.RemotePeer())
}
