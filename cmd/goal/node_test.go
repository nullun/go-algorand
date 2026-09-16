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
package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/go-algorand/ledger/ledgercore"
	"github.com/algorand/go-algorand/test/partitiontest"
)

var isNum = regexp.MustCompile(`^[0-9]+$`)
var isAlnum = regexp.MustCompile(`^[a-zA-Z0-9_]*$`)

func TestGetMissingCatchpointLabel(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	tests := []struct {
		name        string
		URL         string
		expectedErr string
		statusCode  int
	}{
		{
			"bad request",
			"",
			"400 Bad Request",
			http.StatusBadRequest,
		},
		{
			"forbidden request",
			"",
			"403 Forbidden",
			http.StatusForbidden,
		},
		{
			"page not found",
			"",
			"404 Not Found",
			http.StatusNotFound,
		},
		{
			"bad gateway",
			"",
			"502 Bad Gateway",
			http.StatusBadGateway,
		},
		{
			"mainnet catchpoint",
			"https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/mainnet/latest.catchpoint",
			"",
			http.StatusAccepted,
		},
		{
			"betanet catchpoint",
			"https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/betanet/latest.catchpoint",
			"",
			http.StatusAccepted,
		},
		{
			"testnet catchpoint",
			"https://algorand-catchpoints.s3.us-east-2.amazonaws.com/channel/testnet/latest.catchpoint",
			"",
			http.StatusAccepted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, test.expectedErr, test.statusCode)
			}))
			defer ts.Close()

			if test.expectedErr != "" {
				test.URL = ts.URL
			}

			label, err := getMissingCatchpointLabel(test.URL)

			if test.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedErr)
			} else {
				_, _, err = ledgercore.ParseCatchpointLabel(label)
				assert.Equal(t, err, nil)
				splittedLabel := strings.Split(label, "#")
				assert.Equal(t, len(splittedLabel), 2)
				assert.True(t, isNum.MatchString(splittedLabel[0]))
				assert.True(t, isAlnum.MatchString(splittedLabel[1]))
			}
		})
	}
}

func TestFriendlyStreams(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// a typical DHT/pubsub-only connection: deduplicated, ordered, short names
	require.Equal(t, "pubsub,dht",
		friendlyStreams([]string{
			"/algorand/kad/mainnet/kad/1.0.0",
			"/meshsub/1.3.0",
			"/meshsub/1.3.0",
		}))

	// pubsub is ordered before gossip, since nearly every p2p connection
	// carries pubsub and only some carry gossip
	require.Equal(t, "pubsub,gossip,dht",
		friendlyStreams([]string{
			"/algorand/kad/mainnet/kad/1.0.0",
			"/algorand/kad/mainnet/kad/1.0.0",
			"/meshsub/1.3.0",
			"/algorand-ws/2.2.0",
		}))

	// all gossip protocol variants collapse to the same label
	require.Equal(t, "gossip", friendlyStreams([]string{"ws-gossip/2.2"}))
	require.Equal(t, "gossip", friendlyStreams([]string{"/algorand-ws/1.0.0"}))

	// libp2p machinery and unknown protocols
	require.Equal(t, "http,identify,ping,/unknown/1.0.0",
		friendlyStreams([]string{
			"/unknown/1.0.0",
			"/ipfs/ping/1.0.0",
			"/ipfs/id/1.0.0",
			"/http/1.1",
		}))

	// a stream still negotiating its protocol has an empty ID: named, and
	// placed after the known names but before unknown protocols
	require.Equal(t, "negotiating", friendlyStreams([]string{""}))
	require.Equal(t, "gossip,identify,negotiating,/unknown/1.0.0",
		friendlyStreams([]string{"/unknown/1.0.0", "", "/ipfs/id/1.0.0", "/algorand-ws/2.2.0", ""}))

	// an unrecognized protocol is shown verbatim, so it must be escaped
	require.Equal(t, `gossip,/x\x0d\x1b[2K`,
		friendlyStreams([]string{"/algorand-ws/2.2.0", "/x\r\x1b[2K"}))
}

func TestSanitizeTerminal(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// ordinary protocol names pass through untouched
	require.Equal(t, "/algorand-ws/2.2.0", sanitizeTerminal("/algorand-ws/2.2.0"))
	require.Equal(t, "", sanitizeTerminal(""))

	// escape sequences, newlines and other control characters are escaped
	require.Equal(t, `\x1b[31mred`, sanitizeTerminal("\x1b[31mred"))
	require.Equal(t, `a\x0d\x0aCONN TYPE`, sanitizeTerminal("a\r\nCONN TYPE"))
	require.Equal(t, `\x1b]0;pwned\x07`, sanitizeTerminal("\x1b]0;pwned\a"))
	require.Equal(t, `\x00\x7f`, sanitizeTerminal("\x00\x7f"))

	// C1 controls, invisible line separators and astral non-printables too
	require.Equal(t, `\x9b`, sanitizeTerminal("\u009b"))
	require.Equal(t, `\u2028`, sanitizeTerminal("\u2028"))
	require.Equal(t, `\U0001d173`, sanitizeTerminal("\U0001d173"))

	// bytes that are not valid UTF-8 are escaped one at a time
	require.Equal(t, `\xff\xfe`, sanitizeTerminal("\xff\xfe"))

	// printable non-ASCII is left alone
	require.Equal(t, "héllo", sanitizeTerminal("héllo"))
}

func TestPeerIDTail(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// two peer IDs that differ only past the shared "12D3KooW" prefix must
	// still be told apart
	first := "12D3KooWBmVtgUuHqR8Km4vJmkLrfmVPnUyGkFznVXnSeVhcpFcG"
	second := "12D3KooWBmVtgUuHqR8Km4vJmkLrfmVPnUyGkFznVXnSeVhcpAbC"
	require.Equal(t, "..eVhcpFcG", peerIDTail(first))
	require.NotEqual(t, peerIDTail(first), peerIDTail(second))

	// an ID no longer than the display length is shown whole, unmarked
	require.Equal(t, "12D3KooW", peerIDTail("12D3KooW"))
	require.Equal(t, "", peerIDTail(""))

	// the ID is escaped before it reaches the terminal, and cut on runes so
	// that escaping cannot be truncated into a partial sequence
	require.Equal(t, `..Kdefghij`, peerIDTail("abc\r\x1b[2Kdefghij"))
	require.Equal(t, "..llohéllo", peerIDTail("héllohéllo"))
}

func TestRepeatedPeerIDs(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	id := func(s string) *string { return &s }
	peers := []model.PeerStatus{
		{NetworkAddress: "a", PeerId: id("one")},
		{NetworkAddress: "b", PeerId: id("two")},
		{NetworkAddress: "c", PeerId: id("one")},
		{NetworkAddress: "d"},
		{NetworkAddress: "e", PeerId: id("")},
		{NetworkAddress: "f", PeerId: id("")},
		{NetworkAddress: "g", PeerId: id("one")},
	}
	// only a real ID seen more than once counts: missing and empty IDs never do
	require.Equal(t, map[string]bool{"one": true}, repeatedPeerIDs(peers))
	require.Empty(t, repeatedPeerIDs(peers[:2]))
	require.Empty(t, repeatedPeerIDs(nil))
}

func TestPrintPeers(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	id := func(s string) *string { return &s }
	streams := func(protos ...string) *[]string { return &protos }
	bytes := func(n uint64) *uint64 { return &n }
	nodeA := "12D3KooWBmVtgUuHqR8Km4vJmkLrfmVPnUyGkFznVXnSeVhcpFcG"
	nodeB := "12D3KooWBmVtgUuHqR8Km4vJmkLrfmVPnUyGkFznVXnSeVhcpAbC"

	// no peer holds more than one connection: seven columns, no PEER
	// column, no trailing padding after STREAMS, and no footer
	single := []model.PeerStatus{
		{ConnectionType: "inbound", NetworkType: "p2p", NetworkAddress: "/ip4/10.0.0.5/tcp/4160", PeerId: id(nodeA),
			TotalBytesReceived: bytes(1234567), TotalBytesSent: bytes(834 * 1024), ActiveStreams: streams("/algorand-ws/2.2.0", "/meshsub/1.3.0")},
		{ConnectionType: "outbound", NetworkType: "ws", NetworkAddress: "relay.example.com:4160",
			TotalBytesReceived: bytes(5 * 1024 * 1024), TotalBytesSent: bytes(0), ActiveStreams: streams("ws-gossip/2.2")},
		// an old algod reporting no usage at all
		{ConnectionType: "outbound", NetworkType: "ws", NetworkAddress: "old.example.com:4160"},
	}
	var out strings.Builder
	printPeers(&out, single, false)
	require.Equal(t, ""+
		"CONN TYPE  NETWORK  ADDRESS                  RECEIVED       SENT  STREAMS\n"+
		"inbound    p2p      /ip4/10.0.0.5/tcp/4160      1.2MB    834.0KB  pubsub,gossip\n"+
		"outbound   ws       relay.example.com:4160      5.0MB         0B  gossip\n"+
		"outbound   ws       old.example.com:4160            -          -  -\n",
		out.String())

	// one node with two connections: the PEER column appears, marked only on
	// that node's rows, and its totals are printed once, in algod's order,
	// then zeroed so that the columns still sum correctly
	repeated := []model.PeerStatus{
		single[1],
		{ConnectionType: "inbound", NetworkType: "p2p", NetworkAddress: "/ip4/10.0.0.7/tcp/51022", PeerId: id(nodeB),
			TotalBytesReceived: bytes(3 * 1024 * 1024), TotalBytesSent: bytes(2 * 1024 * 1024), ActiveStreams: streams("/meshsub/1.3.0")},
		single[0],
		{ConnectionType: "outbound", NetworkType: "p2p", NetworkAddress: "/ip4/10.0.0.7/tcp/4160", PeerId: id(nodeB),
			TotalBytesReceived: bytes(3 * 1024 * 1024), TotalBytesSent: bytes(2 * 1024 * 1024), ActiveStreams: streams("/algorand-ws/2.2.0", "/meshsub/1.3.0")},
	}
	out.Reset()
	printPeers(&out, repeated, false)
	require.Equal(t, ""+
		"CONN TYPE  NETWORK  ADDRESS                   RECEIVED       SENT  STREAMS        PEER\n"+
		"outbound   ws       relay.example.com:4160       5.0MB         0B  gossip         \n"+
		"inbound    p2p      /ip4/10.0.0.7/tcp/51022      3.0MB      2.0MB  pubsub         ..eVhcpAbC\n"+
		"inbound    p2p      /ip4/10.0.0.5/tcp/4160       1.2MB    834.0KB  pubsub,gossip  \n"+
		"outbound   p2p      /ip4/10.0.0.7/tcp/4160          0B         0B  pubsub,gossip  ..eVhcpAbC\n"+
		"1 peer ID(s) with more than one connection: traffic totals cover all of a peer's connections and are shown on its first row only\n",
		out.String())

	// verbose adds the full peer ID and the supported protocols, one per
	// line and each escaped, under the row
	verbose := []model.PeerStatus{
		{ConnectionType: "inbound", NetworkType: "p2p", NetworkAddress: "/ip4/10.0.0.5/tcp/4160", PeerId: id(nodeA),
			TotalBytesReceived: bytes(10), TotalBytesSent: bytes(20), ActiveStreams: streams("/algorand-ws/2.2.0"),
			SupportedProtocols: streams("/algorand-ws/2.2.0", "/ipfs/id/1.0.0", "/x\x1b[2K")},
		{ConnectionType: "outbound", NetworkType: "ws", NetworkAddress: "relay.example.com:4160",
			TotalBytesReceived: bytes(30), TotalBytesSent: bytes(40), ActiveStreams: streams("ws-gossip/2.2")},
	}
	out.Reset()
	printPeers(&out, verbose, true)
	require.Equal(t, ""+
		"CONN TYPE  NETWORK  ADDRESS                  RECEIVED       SENT  STREAMS\n"+
		"inbound    p2p      /ip4/10.0.0.5/tcp/4160        10B        20B  gossip\n"+
		"                    peer: "+nodeA+"\n"+
		"                    supported protocols:\n"+
		"                      /algorand-ws/2.2.0\n"+
		"                      /ipfs/id/1.0.0\n"+
		`                      /x\x1b[2K`+"\n"+
		"outbound   ws       relay.example.com:4160        30B        40B  gossip\n",
		out.String())
}
