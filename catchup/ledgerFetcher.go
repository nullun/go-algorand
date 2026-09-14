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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/ledger"
	"github.com/algorand/go-algorand/ledger/encoded"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/network"
	"github.com/algorand/go-algorand/rpcs"
	"github.com/algorand/go-algorand/util"
)

var errNoLedgerForRound = errors.New("no ledger available for given round")

const (
	// legacyResourcesPerCatchpointFileChunk is the resource count that writers
	// predating the byte-based bound used to batch by. It survives only to size
	// maxCatchpointFileChunkDownloadSize.
	legacyResourcesPerCatchpointFileChunk = 100_000

	// maxCatchpointSectionAllocHint is the non-resource portion of a catchpoint
	// chunk: up to BalancesPerCatchpointFileChunk accounts plus the same number of
	// KV pairs, both hard-capped in count (a maximal KV chunk is exactly
	// BalancesPerCatchpointFileChunk*MaxEncodedKVDataSize). It bounds how much
	// memory readCatchpointFileChunk reserves up front for a section whose size a
	// peer has declared but not yet sent. The much larger remaining term of
	// maxCatchpointFileChunkDownloadSize comes from per-account resources, which
	// only approach their bound for pathological all-max-app accounts that do not
	// occur in practice, so realistic sections are received in a single allocation.
	maxCatchpointSectionAllocHint = ledger.BalancesPerCatchpointFileChunk * (ledger.MaxEncodedBaseAccountDataSize + encoded.MaxEncodedKVDataSize)

	// maxCatchpointFileChunkDownloadSize accepts the largest chunk that an older
	// writer could produce, which is roughly 2.8 GiB. The writer now bounds its
	// chunks at ledger.MaxCatchpointFileChunkSize, so once catchpoint files
	// predating that bound age out of support this can be lowered to match.
	// Until then it is only an acceptance ceiling: what a download actually
	// allocates is governed by maxCatchpointSectionAllocHint and by the bytes
	// the peer really sends.
	maxCatchpointFileChunkDownloadSize = maxCatchpointSectionAllocHint +
		legacyResourcesPerCatchpointFileChunk*ledger.MaxEncodedBaseResourceDataSize
	// defaultMinCatchpointFileDownloadBytesPerSecond defines the worst-case scenario download speed we expect to get while downloading a catchpoint file
	defaultMinCatchpointFileDownloadBytesPerSecond = 20 * 1024
	// catchpointFileStreamReadSize defines the number of bytes we would attempt to read at each iteration from the incoming http data stream
	catchpointFileStreamReadSize = 4096
)

var errNonHTTPPeer = fmt.Errorf("downloadLedger : non-HTTPPeer encountered")

func validateCatchpointFileChunkSize(size int64) error {
	if size > maxCatchpointFileChunkDownloadSize || size < 1 {
		return fmt.Errorf("getPeerLedger received a tar header with data size of %d", size)
	}
	return nil
}

// readCatchpointFileChunk reads one catchpoint tar section into buf and returns
// its contents, which remain valid only until the next call.
//
// size is declared by the remote peer and checked against
// maxCatchpointFileChunkDownloadSize before this is called, but that ceiling is
// far larger than any real chunk. Reserving it outright would let a peer force a
// multi-gigabyte allocation by declaring a section it never sends, so only
// maxCatchpointSectionAllocHint is reserved up front and buf grows to hold
// whatever actually arrives. buf is reused across sections, so a download of
// same-sized chunks settles into a single allocation.
func readCatchpointFileChunk(reader io.Reader, size int64, buf *bytes.Buffer) ([]byte, error) {
	buf.Reset()
	buf.Grow(int(min(size, maxCatchpointSectionAllocHint)))
	if _, err := io.Copy(buf, io.LimitReader(reader, size)); err != nil {
		return nil, err
	}
	// io.Copy stops at the end of the section rather than at size, so this
	// restores the "exactly size bytes or fail" guarantee io.ReadFull gave.
	if int64(buf.Len()) != size {
		return nil, fmt.Errorf("catchpoint chunk declared %d bytes but %d were received", size, buf.Len())
	}
	return buf.Bytes(), nil
}

type ledgerFetcherReporter interface {
	updateLedgerFetcherProgress(*ledger.CatchpointCatchupAccessorProgress)
}

type ledgerFetcher struct {
	net      network.GossipNode
	accessor ledger.CatchpointCatchupAccessor
	log      logging.Logger

	reporter ledgerFetcherReporter
	config   config.Local
}

func makeLedgerFetcher(net network.GossipNode, accessor ledger.CatchpointCatchupAccessor, log logging.Logger, reporter ledgerFetcherReporter, cfg config.Local) *ledgerFetcher {
	return &ledgerFetcher{
		net:      net,
		accessor: accessor,
		log:      log,
		reporter: reporter,
		config:   cfg,
	}
}

func (lf *ledgerFetcher) requestLedger(ctx context.Context, peer network.HTTPPeer, round basics.Round, method string) (*http.Response, error) {
	ledgerURL := network.SubstituteGenesisID(lf.net, "/v1/{genesisID}/ledger/"+strconv.FormatUint(uint64(round), 36))
	lf.log.Debugf("ledger %s %#v peer %#v %T", method, ledgerURL, peer, peer)
	request, err := http.NewRequestWithContext(ctx, method, ledgerURL, nil)
	if err != nil {
		return nil, err
	}

	network.SetUserAgentHeader(request.Header)
	httpClient := peer.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("requestLedger: HTTPPeer %s has no http client", peer.GetAddress())
	}
	return httpClient.Do(request)
}

func (lf *ledgerFetcher) headLedger(ctx context.Context, peer network.Peer, round basics.Round) error {
	httpPeer, ok := peer.(network.HTTPPeer)
	if !ok {
		return errNonHTTPPeer
	}
	timeoutContext, timeoutContextCancel := context.WithTimeout(ctx, lf.config.MaxCatchpointDownloadDuration)
	defer timeoutContextCancel()
	response, err := lf.requestLedger(timeoutContext, httpPeer, round, http.MethodHead)
	if err != nil {
		lf.log.Debugf("getPeerLedger HEAD : %s", err)
		return err
	}
	defer func() { _ = response.Body.Close() }()

	// check to see that we had no errors.
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound: // server could not find a block with that round number.
		return errNoLedgerForRound
	default:
		return fmt.Errorf("headLedger error response status code %d", response.StatusCode)
	}
}

func (lf *ledgerFetcher) downloadLedger(ctx context.Context, peer network.Peer, round basics.Round) error {
	httpPeer, ok := peer.(network.HTTPPeer)
	if !ok {
		return errNonHTTPPeer
	}
	return lf.getPeerLedger(ctx, httpPeer, round)
}

func (lf *ledgerFetcher) getPeerLedger(ctx context.Context, peer network.HTTPPeer, round basics.Round) error {
	timeoutContext, timeoutContextCancel := context.WithTimeout(ctx, lf.config.MaxCatchpointDownloadDuration)
	defer timeoutContextCancel()
	response, err := lf.requestLedger(timeoutContext, peer, round, http.MethodGet)
	if err != nil {
		lf.log.Debugf("getPeerLedger GET : %s", err)
		return err
	}
	defer response.Body.Close()

	// check to see that we had no errors.
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound: // server could not find a block with that round numbers.
		return errNoLedgerForRound
	default:
		return fmt.Errorf("getPeerLedger error response status code %d", response.StatusCode)
	}

	// at this point, we've already received the response headers. ensure that the
	// response content type is what we'd like it to be.
	contentTypes := response.Header["Content-Type"]
	if len(contentTypes) != 1 {
		err = fmt.Errorf("getPeerLedger : http ledger fetcher invalid content type count %d", len(contentTypes))
		return err
	}

	if contentTypes[0] != rpcs.LedgerResponseContentType {
		err = fmt.Errorf("getPeerLedger : http ledger fetcher response has an invalid content type : %s", contentTypes[0])
		return err
	}

	// maxCatchpointFileChunkDownloadDuration is the maximum amount of time we would wait to download a single chunk off a catchpoint file
	maxCatchpointFileChunkDownloadDuration := 2 * time.Minute
	if lf.config.MinCatchpointFileDownloadBytesPerSecond > 0 {
		maxCatchpointFileChunkDownloadDuration += maxCatchpointFileChunkDownloadSize * time.Second / time.Duration(lf.config.MinCatchpointFileDownloadBytesPerSecond)
	} else {
		maxCatchpointFileChunkDownloadDuration += maxCatchpointFileChunkDownloadSize * time.Second / defaultMinCatchpointFileDownloadBytesPerSecond
	}

	watchdogReader := util.MakeWatchdogStreamReader(response.Body, catchpointFileStreamReadSize, 2*maxCatchpointFileChunkDownloadSize, maxCatchpointFileChunkDownloadDuration)
	defer watchdogReader.Close()
	tarReader := tar.NewReader(watchdogReader)
	var downloadProgress ledger.CatchpointCatchupAccessorProgress
	var writeDuration time.Duration

	printLogsFunc := func() {
		lf.log.Infof(
			"writing balances to disk took %d seconds, "+
				"writing creatables to disk took %d seconds, "+
				"writing hashes to disk took %d seconds, "+
				"writing kv pairs to disk took %d seconds, "+
				"total duration is %d seconds",
			downloadProgress.BalancesWriteDuration/time.Second,
			downloadProgress.CreatablesWriteDuration/time.Second,
			downloadProgress.HashesWriteDuration/time.Second,
			downloadProgress.KVWriteDuration/time.Second,
			writeDuration/time.Second)
	}

	var chunkBuffer bytes.Buffer
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				printLogsFunc()
				return nil
			}
			return err
		}
		if err = validateCatchpointFileChunkSize(header.Size); err != nil {
			return err
		}
		balancesBlockBytes, err := readCatchpointFileChunk(tarReader, header.Size, &chunkBuffer)
		if err != nil {
			return err
		}
		start := time.Now()
		err = lf.processBalancesBlock(ctx, header.Name, balancesBlockBytes, &downloadProgress)
		if err != nil {
			return err
		}
		writeDuration += time.Since(start)
		if lf.reporter != nil {
			lf.reporter.updateLedgerFetcherProgress(&downloadProgress)
		}
		if err = watchdogReader.Reset(); err != nil {
			if err == io.EOF {
				printLogsFunc()
				return nil
			}
			err = fmt.Errorf("getPeerLedger received the following error while reading the catchpoint file : %v", err)
			return err
		}
	}
}

func (lf *ledgerFetcher) processBalancesBlock(ctx context.Context, sectionName string, bytes []byte, downloadProgress *ledger.CatchpointCatchupAccessorProgress) error {
	return lf.accessor.ProcessStagingBalances(ctx, sectionName, bytes, downloadProgress)
}
