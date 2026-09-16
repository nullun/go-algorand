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

package v2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/go-algorand/network"
	"github.com/algorand/go-algorand/test/partitiontest"
)

type testPeerConnInfo struct {
	addr        string
	networkType network.PeerNetworkType
}

func (p testPeerConnInfo) GetAddress() string                      { return p.addr }
func (p testPeerConnInfo) GetNetworkType() network.PeerNetworkType { return p.networkType }

type testPeerUsageInfo struct {
	testPeerConnInfo
	usage network.PeerUsage
}

func (p testPeerUsageInfo) GetUsage() network.PeerUsage { return p.usage }

func TestConvertPeers(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	peers := []network.Peer{
		testPeerConnInfo{addr: "/ip4/10.0.0.2/tcp/4160", networkType: network.PeerNetworkTypeLibP2P},
		testPeerConnInfo{addr: "192.168.1.5:4160", networkType: network.PeerNetworkTypeWebsocket},
		// peers that cannot report connection info are skipped
		struct{}{},
	}

	statuses := convertPeers(peers, model.PeerStatusConnectionTypeInbound)
	require.Equal(t, []model.PeerStatus{
		{
			ConnectionType: model.PeerStatusConnectionTypeInbound,
			NetworkAddress: "/ip4/10.0.0.2/tcp/4160",
			NetworkType:    model.PeerStatusNetworkTypeP2p,
		},
		{
			ConnectionType: model.PeerStatusConnectionTypeInbound,
			NetworkAddress: "192.168.1.5:4160",
			NetworkType:    model.PeerStatusNetworkTypeWs,
		},
	}, statuses)

	// every status must use a value declared in the API enum
	for _, status := range statuses {
		require.Contains(t, []model.PeerStatusNetworkType{
			model.PeerStatusNetworkTypeWs,
			model.PeerStatusNetworkTypeP2p,
		}, status.NetworkType)
	}

	require.Empty(t, convertPeers(nil, model.PeerStatusConnectionTypeOutbound))
}

func TestConvertPeersUsage(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	supported := []string{"/algorand-ws/2.2.0", "/algorand/kad/testnet/1.0.0"}
	active := []string{"/algorand-ws/2.2.0"}
	peers := []network.Peer{
		testPeerUsageInfo{
			testPeerConnInfo: testPeerConnInfo{addr: "/ip4/10.0.0.2/tcp/4160", networkType: network.PeerNetworkTypeLibP2P},
			usage: network.PeerUsage{
				PeerID:             "12D3KooWFirst",
				SupportedProtocols: supported,
				ActiveStreams:      active,
				TotalBytesReceived: 1234,
				TotalBytesSent:     567,
			},
		},
		// a peer without usage info leaves the optional fields unset
		testPeerConnInfo{addr: "192.168.1.5:4160", networkType: network.PeerNetworkTypeWebsocket},
		// a usage-reporting peer with no streams and no traffic reports zero
		// totals and an empty stream list: goal's gossip-only filter reads an
		// omitted list as "this algod does not report streams" and keeps the
		// peer, so a peer that really has no streams must say so
		testPeerUsageInfo{
			testPeerConnInfo: testPeerConnInfo{addr: "/ip4/10.0.0.3/tcp/4160", networkType: network.PeerNetworkTypeLibP2P},
		},
	}

	statuses := convertPeers(peers, model.PeerStatusConnectionTypeOutbound)
	require.Len(t, statuses, 3)

	require.Equal(t, "/ip4/10.0.0.2/tcp/4160", statuses[0].NetworkAddress)
	require.NotNil(t, statuses[0].PeerId)
	require.Equal(t, "12D3KooWFirst", *statuses[0].PeerId)
	require.NotNil(t, statuses[0].SupportedProtocols)
	require.Equal(t, supported, *statuses[0].SupportedProtocols)
	require.NotNil(t, statuses[0].ActiveStreams)
	require.Equal(t, active, *statuses[0].ActiveStreams)
	require.NotNil(t, statuses[0].TotalBytesReceived)
	require.EqualValues(t, 1234, *statuses[0].TotalBytesReceived)
	require.NotNil(t, statuses[0].TotalBytesSent)
	require.EqualValues(t, 567, *statuses[0].TotalBytesSent)

	require.Equal(t, "/ip4/10.0.0.3/tcp/4160", statuses[1].NetworkAddress)
	require.Nil(t, statuses[1].PeerId)
	require.Nil(t, statuses[1].SupportedProtocols)
	require.NotNil(t, statuses[1].ActiveStreams)
	require.Empty(t, *statuses[1].ActiveStreams)
	require.NotNil(t, statuses[1].TotalBytesReceived)
	require.Zero(t, *statuses[1].TotalBytesReceived)

	require.Equal(t, "192.168.1.5:4160", statuses[2].NetworkAddress)
	require.Nil(t, statuses[2].PeerId)
	require.Nil(t, statuses[2].SupportedProtocols)
	require.Nil(t, statuses[2].ActiveStreams)
	require.Nil(t, statuses[2].TotalBytesReceived)
	require.Nil(t, statuses[2].TotalBytesSent)
}
