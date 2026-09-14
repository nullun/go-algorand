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

package catchup

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/components/mocks"
	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/ledger"
	"github.com/algorand/go-algorand/logging"
	p2ptesting "github.com/algorand/go-algorand/network/p2p/testing"
	"github.com/algorand/go-algorand/rpcs"
	"github.com/algorand/go-algorand/test/partitiontest"
)

type dummyLedgerFetcherReporter struct {
}

func (lf *dummyLedgerFetcherReporter) updateLedgerFetcherProgress(*ledger.CatchpointCatchupAccessorProgress) {
}

func TestValidateCatchpointFileChunkSize(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	require.Error(t, validateCatchpointFileChunkSize(0))
	require.NoError(t, validateCatchpointFileChunkSize(ledger.MaxCatchpointFileChunkSize))
	require.NoError(t, validateCatchpointFileChunkSize(maxCatchpointFileChunkDownloadSize))
	require.Error(t, validateCatchpointFileChunkSize(maxCatchpointFileChunkDownloadSize+1))
	// the download bound must accept everything the writer can produce
	require.GreaterOrEqual(t, int64(maxCatchpointFileChunkDownloadSize), int64(ledger.MaxCatchpointFileChunkSize))
}

func TestReadCatchpointFileChunk(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	var buf bytes.Buffer
	data, err := readCatchpointFileChunk(bytes.NewReader([]byte("first")), 5, &buf)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), data)
	firstCapacity := buf.Cap()

	// the buffer is reused, so a second section of the same shape does not grow it
	data, err = readCatchpointFileChunk(bytes.NewReader([]byte("next")), 4, &buf)
	require.NoError(t, err)
	require.Equal(t, []byte("next"), data)
	require.Equal(t, firstCapacity, buf.Cap())
}

func TestReadCatchpointFileChunkRejectsShortSection(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	var buf bytes.Buffer
	_, err := readCatchpointFileChunk(bytes.NewReader([]byte("short")), 64, &buf)
	require.ErrorContains(t, err, "declared 64 bytes but 5 were received")
}

// TestReadCatchpointFileChunkDoesNotPreallocate asserts that a section size a
// peer declares but does not send is not reserved up front. A peer may legally
// declare up to maxCatchpointFileChunkDownloadSize (~2.8 GiB); only
// maxCatchpointSectionAllocHint may be reserved before data arrives.
func TestReadCatchpointFileChunkDoesNotPreallocate(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	var buf bytes.Buffer
	_, err := readCatchpointFileChunk(bytes.NewReader(nil), maxCatchpointFileChunkDownloadSize, &buf)
	require.ErrorContains(t, err, "0 were received")
	require.LessOrEqual(t, buf.Cap(), 2*maxCatchpointSectionAllocHint,
		"reserved %d bytes for a declared but unsent section", buf.Cap())
}

// TestLedgerFetcherDoesNotPreallocateTarSize is the end-to-end form of the
// check above: a peer declaring a large tar section and sending almost nothing
// must not drive the fetch's allocation.
func TestLedgerFetcherDoesNotPreallocateTarSize(t *testing.T) {
	partitiontest.PartitionTest(t)
	// Intentionally not parallel: this measures process-wide allocation, which
	// concurrently running tests would perturb.

	// Large enough that a pre-allocation dwarfs the threshold asserted below,
	// small enough that a regressed build fails rather than exhausting memory.
	const declaredSize = int64(512 << 20) // 512 MiB

	mux := http.NewServeMux()
	s := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	go s.Serve(listener)
	defer s.Close()
	defer listener.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", rpcs.LedgerResponseContentType)
		w.WriteHeader(http.StatusOK)
		wtar := tar.NewWriter(w)
		// Declare a large section but send only a few bytes, then end the
		// response so the client observes a truncated section.
		if werr := wtar.WriteHeader(&tar.Header{Name: "balances.1.msgpack", Size: declaredSize}); werr != nil {
			return
		}
		_, _ = wtar.Write([]byte("truncated section"))
	})

	peer := testHTTPPeer(listener.Addr().String())
	lf := makeLedgerFetcher(&mocks.MockNetwork{}, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	err = lf.getPeerLedger(context.Background(), &peer, basics.Round(0))
	runtime.ReadMemStats(&after)

	// The truncated section must surface as an error rather than being processed.
	require.Error(t, err)

	// TotalAlloc is cumulative, so the delta is what the fetch allocated. Before
	// the fix this included make([]byte, header.Size).
	allocated := after.TotalAlloc - before.TotalAlloc
	require.Less(t, allocated, uint64(64<<20),
		"getPeerLedger allocated %d bytes for a peer-declared section size of %d; it must not pre-allocate based on the tar header",
		allocated, declaredSize)
}

func TestNoPeersAvailable(t *testing.T) {
	partitiontest.PartitionTest(t)

	lf := makeLedgerFetcher(&mocks.MockNetwork{}, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())
	peer := &lf // The peer is an opaque interface.. we can add anything as a Peer.
	err := lf.downloadLedger(context.Background(), peer, basics.Round(0))
	require.Equal(t, errNonHTTPPeer, err)
}

func TestNonParsableAddress(t *testing.T) {
	partitiontest.PartitionTest(t)

	lf := makeLedgerFetcher(&mocks.MockNetwork{}, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())
	peer := testHTTPPeer(":def")
	err := lf.getPeerLedger(context.Background(), &peer, basics.Round(0))
	require.Error(t, err)
}

func TestLedgerFetcherErrorResponseHandling(t *testing.T) {
	partitiontest.PartitionTest(t)
	testcases := []struct {
		name               string
		httpServerResponse int
		contentTypes       []string
		err                error
	}{
		{
			name:               "getPeerLedger 400 Response",
			httpServerResponse: http.StatusNotFound,
			contentTypes:       make([]string, 0),
			err:                errNoLedgerForRound,
		},
		{
			name:               "getPeerLedger 500 Response",
			httpServerResponse: http.StatusInternalServerError,
			contentTypes:       make([]string, 0),
			err:                fmt.Errorf("getPeerLedger error response status code %d", http.StatusInternalServerError),
		},
		{
			name:               "getPeerLedger No Content Type",
			httpServerResponse: http.StatusOK,
			contentTypes:       make([]string, 0),
			err:                fmt.Errorf("getPeerLedger : http ledger fetcher invalid content type count %d", 0),
		},
		{
			name:               "getPeerLedger Too Many Content Types",
			httpServerResponse: http.StatusOK,
			contentTypes:       []string{"applications/one", "applications/two"},
			err:                fmt.Errorf("getPeerLedger : http ledger fetcher invalid content type count %d", 2),
		},
		{
			name:               "getPeerLedger Invalid Content Type",
			httpServerResponse: http.StatusOK,
			contentTypes:       []string{"applications/one"},
			err:                fmt.Errorf("getPeerLedger : http ledger fetcher response has an invalid content type : %s", "applications/one"),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// create a dummy server.
			mux := http.NewServeMux()
			s := &http.Server{
				Handler: mux,
			}
			listener, err := net.Listen("tcp", "localhost:")

			require.NoError(t, err)
			go s.Serve(listener)
			defer s.Close()
			defer listener.Close()
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				for _, contentType := range tc.contentTypes {
					w.Header().Add("Content-Type", contentType)
				}
				w.WriteHeader(tc.httpServerResponse)
			})
			lf := makeLedgerFetcher(&mocks.MockNetwork{}, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())
			peer := testHTTPPeer(listener.Addr().String())
			err = lf.getPeerLedger(context.Background(), &peer, basics.Round(0))
			require.Equal(t, tc.err, err)
		})
	}
}

func TestLedgerFetcher(t *testing.T) {
	partitiontest.PartitionTest(t)

	// create a dummy server.
	mux := http.NewServeMux()
	s := &http.Server{
		Handler: mux,
	}
	listener, err := net.Listen("tcp", "localhost:")

	var httpServerResponse = 0
	require.NoError(t, err)
	go s.Serve(listener)
	defer s.Close()
	defer listener.Close()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodHead {
			w.WriteHeader(httpServerResponse)
		} else {
			w.Header().Add("Content-Type", rpcs.LedgerResponseContentType)
			w.WriteHeader(httpServerResponse)
			wtar := tar.NewWriter(w)
			wtar.Close()
		}
	})
	successPeer := testHTTPPeer(listener.Addr().String())
	lf := makeLedgerFetcher(&mocks.MockNetwork{}, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())

	// headLedger non-http peer
	err = lf.headLedger(context.Background(), nil, basics.Round(0))
	require.Equal(t, errNonHTTPPeer, err)

	// headLedger parseURL failure
	parseFailurePeer := testHTTPPeer("foobar")
	err = lf.headLedger(context.Background(), &parseFailurePeer, basics.Round(0))
	require.ErrorContains(t, err, "could not parse a host from url")

	// headLedger 404 response
	httpServerResponse = http.StatusNotFound
	err = lf.headLedger(context.Background(), &successPeer, basics.Round(0))
	require.Equal(t, errNoLedgerForRound, err)

	// headLedger 200 response
	httpServerResponse = http.StatusOK
	err = lf.headLedger(context.Background(), &successPeer, basics.Round(0))
	require.NoError(t, err)

	httpServerResponse = http.StatusOK
	err = lf.downloadLedger(context.Background(), &successPeer, basics.Round(0))
	require.NoError(t, err)

	// headLedger 500 response
	httpServerResponse = http.StatusInternalServerError
	err = lf.headLedger(context.Background(), &successPeer, basics.Round(0))
	require.Equal(t, fmt.Errorf("headLedger error response status code %d", http.StatusInternalServerError), err)
}

func TestLedgerFetcherP2P(t *testing.T) {
	partitiontest.PartitionTest(t)

	mux := http.NewServeMux()
	nodeA := p2ptesting.MakeHTTPNode(t)
	nodeA.RegisterHTTPHandler("/v1/ledger/0", mux)
	var httpServerResponse = 0
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodHead {
			w.WriteHeader(httpServerResponse)
		} else {
			w.Header().Add("Content-Type", rpcs.LedgerResponseContentType)
			w.WriteHeader(httpServerResponse)
			wtar := tar.NewWriter(w)
			wtar.Close()
		}
	})

	nodeA.Start()
	defer nodeA.Stop()

	successPeer := nodeA.GetHTTPPeer()
	lf := makeLedgerFetcher(nodeA, &mocks.MockCatchpointCatchupAccessor{}, logging.TestingLog(t), &dummyLedgerFetcherReporter{}, config.GetDefaultLocal())

	// headLedger 200 response
	httpServerResponse = http.StatusOK
	err := lf.headLedger(context.Background(), successPeer, basics.Round(0))
	require.NoError(t, err)

	httpServerResponse = http.StatusOK
	err = lf.downloadLedger(context.Background(), successPeer, basics.Round(0))
	require.NoError(t, err)
}
