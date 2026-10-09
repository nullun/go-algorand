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

package verify

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	basics_testing "github.com/algorand/go-algorand/data/basics/testing"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/util/execpool"
)

// makeSchemePayments returns n distinct singleton payment groups authorized by
// one account of the given scheme. The zero scheme means a classic Ed25519
// Sig.
func makeSchemePayments(b *testing.B, scheme protocol.PQScheme, n int) [][]transactions.SignedTxn {
	fee := config.Consensus[protocol.ConsensusFuture].MinTxnFee
	groups := make([][]transactions.SignedTxn, n)

	if scheme == (protocol.PQScheme{}) {
		secrets, addrs, _ := generateAccounts(1)
		for i := range groups {
			txn := createPayTransaction(fee, 40, 60, i+1, addrs[0], basics.Address{1})
			txn.Note = binary.AppendUvarint(nil, uint64(i))
			groups[i] = []transactions.SignedTxn{txn.Sign(secrets[0])}
		}
		return groups
	}

	acct := basics_testing.MakePQTestAccount(b, 1, scheme)
	for i := range groups {
		txn := createPayTransaction(fee, 40, 60, i+1, acct.Address, basics.Address{1})
		txn.Note = binary.AppendUvarint(nil, uint64(i))
		sig, err := acct.Signer.Sign(txn)
		require.NoError(b, err)
		groups[i] = []transactions.SignedTxn{{
			Txn: txn,
			PQsig: transactions.PQSig{
				Scheme:    acct.Scheme,
				Salt:      acct.Salt,
				PublicKey: acct.PublicKey,
				Signature: sig,
			},
		}}
	}
	return groups
}

// BenchmarkPaysetGroupsByScheme measures the cost of verifying signed
// payments through PaysetGroups, the path that block validation takes, for
// each signature scheme. Every iteration uses a fresh cache, so every
// signature is verified. The ns/txn metric is wall time per transaction with
// every core in use; BenchmarkTxnGroupByScheme gives the CPU time.
func BenchmarkPaysetGroupsByScheme(b *testing.B) {
	type scheme struct {
		name   string
		scheme protocol.PQScheme
		n      int
	}
	schemes := []scheme{{"sig-ed25519", protocol.PQScheme{}, 4096}}
	for _, s := range basics_testing.PQTestSchemes {
		n := 4096
		if s.Scheme == protocol.PQSchemeSQIsign1 {
			// Signing takes about 60 ms. 768 transactions fill 24 worksets of
			// txnPerWorksetThreshold, enough to occupy every core.
			n = 768
		}
		schemes = append(schemes, scheme{"pqsig-" + s.Name, s.Scheme, n})
	}

	blkHdr := createDummyBlockHeader(protocol.ConsensusFuture)
	execPool := execpool.MakePool(b)
	defer execPool.Shutdown()
	verificationPool := execpool.MakeBacklog(execPool, 64, execpool.LowPriority, b)
	defer verificationPool.Shutdown()

	for _, s := range schemes {
		groups := makeSchemePayments(b, s.scheme, s.n)
		var bytes int
		for _, g := range groups {
			bytes += len(protocol.Encode(&g[0]))
		}

		b.Run(s.name, func(b *testing.B) {
			var elapsed time.Duration
			for b.Loop() {
				cache := MakeVerifiedTransactionCache(2 * s.n)
				start := time.Now()
				err := PaysetGroups(context.Background(), groups, blkHdr, verificationPool, cache, nil)
				elapsed += time.Since(start)
				require.NoError(b, err)
			}
			b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N*s.n), "ns/txn")
			b.ReportMetric(float64(bytes)/float64(s.n), "B/txn")
		})
	}
}

// BenchmarkSignatureVerifyByScheme measures one signature verification per
// scheme, outside the transaction pipeline. Ed25519 is measured both alone and
// amortized over a batch of 64, as the node verifies it in batches.
func BenchmarkSignatureVerifyByScheme(b *testing.B) {
	b.Run("ed25519-batch64", func(b *testing.B) {
		const batch = 64
		secrets, _, _ := generateAccounts(batch)
		txns := make([]transactions.Transaction, batch)
		sigs := make([]crypto.Signature, batch)
		for i := range txns {
			txns[i] = createPayTransaction(1000, 40, 60, i+1, basics.Address{2}, basics.Address{1})
			sigs[i] = secrets[i].Sign(txns[i])
		}
		for b.Loop() {
			bv := crypto.MakeBatchVerifierWithHint(batch)
			for i := range txns {
				bv.EnqueueSignature(secrets[i].SignatureVerifier, txns[i], sigs[i])
			}
			require.NoError(b, bv.Verify())
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*batch), "ns/sig")
	})

	for _, s := range basics_testing.PQTestSchemes {
		b.Run(s.Name, func(b *testing.B) {
			acct := basics_testing.MakePQTestAccount(b, 1, s.Scheme)
			txn := createPayTransaction(1000, 40, 60, 1, acct.Address, basics.Address{1})
			sig, err := acct.Signer.Sign(txn)
			require.NoError(b, err)
			verifier, ok := crypto.LookupPQScheme(s.Scheme)
			require.True(b, ok)
			for b.Loop() {
				require.NoError(b, verifier.Verify(txn, acct.PublicKey, sig))
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "ns/sig")
		})
	}
}

// BenchmarkTxnGroupByScheme verifies the same payments as
// BenchmarkPaysetGroupsByScheme one at a time on a single goroutine, so its
// ns/txn metric is the CPU time to verify one transaction.
func BenchmarkTxnGroupByScheme(b *testing.B) {
	type scheme struct {
		name   string
		scheme protocol.PQScheme
		n      int
	}
	schemes := []scheme{{"sig-ed25519", protocol.PQScheme{}, 1024}}
	for _, s := range basics_testing.PQTestSchemes {
		n := 1024
		if s.Scheme == protocol.PQSchemeSQIsign1 {
			n = 32
		}
		schemes = append(schemes, scheme{"pqsig-" + s.Name, s.Scheme, n})
	}

	blkHdr := createDummyBlockHeader(protocol.ConsensusFuture)
	for _, s := range schemes {
		groups := makeSchemePayments(b, s.scheme, s.n)
		b.Run(s.name, func(b *testing.B) {
			for b.Loop() {
				for _, g := range groups {
					_, err := TxnGroup(g, &blkHdr, nil, nil)
					require.NoError(b, err)
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*s.n), "ns/txn")
		})
	}
}
