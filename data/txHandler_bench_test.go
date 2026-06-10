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

package data

import (
	"testing"

	"github.com/algorand/go-deadlock"

	"github.com/algorand/go-algorand/network"
)

// BenchmarkProcessIncomingTxnDroppedDup measures the cost of a message
// dropped by the duplicate check -- the highest-volume early-return path in
// processIncomingTxn (relays receive the same txn group from many peers).
func BenchmarkProcessIncomingTxnDroppedDup(b *testing.B) {
	deadlockDisable := deadlock.Opts.Disable
	deadlock.Opts.Disable = true
	defer func() {
		deadlock.Opts.Disable = deadlockDisable
	}()

	handler := makeTestTxHandlerOrphaned(txBacklogSize)
	_, blob := makeRandomTransactions(16)

	// first delivery enqueues; drain it so the backlog stays empty
	action := handler.processIncomingTxn(network.IncomingMessage{Data: blob})
	if action.Action != network.Ignore {
		b.Fatalf("unexpected action %v", action.Action)
	}
	<-handler.backlogQueue

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// every delivery after the first is dropped as a duplicate
		handler.processIncomingTxn(network.IncomingMessage{Data: blob})
	}
}

// BenchmarkErlGetClientRegister measures the steady-state per-message ERL
// admission path: getClient resolves the sender's routing address to its
// erlIPClient and register confirms the (already-registered) membership.
func BenchmarkErlGetClientRegister(b *testing.B) {
	deadlockDisable := deadlock.Opts.Disable
	deadlock.Opts.Disable = true
	defer func() {
		deadlock.Opts.Disable = deadlockDisable
	}()

	mapper := &erlClientMapper{
		mapping:    make(map[string]*erlIPClient),
		maxClients: 4,
	}
	peer := newErlMockPeer("192.168.1.1")
	mapper.getClient(peer) // register once; the loop measures the steady state

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.getClient(peer)
	}
}

// BenchmarkErlGetClientRegisterParallel is the contended variant of
// BenchmarkErlGetClientRegister: many goroutines admitting messages from
// clients behind the same routing address, as GossipSub validator workers do.
func BenchmarkErlGetClientRegisterParallel(b *testing.B) {
	deadlockDisable := deadlock.Opts.Disable
	deadlock.Opts.Disable = true
	defer func() {
		deadlock.Opts.Disable = deadlockDisable
	}()

	mapper := &erlClientMapper{
		mapping:    make(map[string]*erlIPClient),
		maxClients: 4,
	}
	peer := newErlMockPeer("192.168.1.1")
	mapper.getClient(peer)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mapper.getClient(peer)
		}
	})
}
