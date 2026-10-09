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

package agreement

import (
	"context"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/account"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/algorand/go-algorand/util"
)

// fixedBlockFactory returns the same prebuilt block for every round, so the
// measurements cover the per-account proposal work rather than pool assembly.
type fixedBlockFactory struct{ blk bookkeeping.Block }

func (f fixedBlockFactory) AssembleBlock(r basics.Round, _ []basics.Address) (UnfinishedBlock, error) {
	blk := f.blk
	blk.BlockHeader.Round = r
	return testValidatedBlock{Inside: blk}, nil
}

func seededHash(parts ...uint64) [64]byte {
	buf := make([]byte, 8*len(parts))
	for i, p := range parts {
		binary.LittleEndian.PutUint64(buf[8*i:], p)
	}
	return sha512.Sum512(buf)
}

// benchProposalBlock builds a block of numTxns distinct signed payments;
// numTxns < 0 fills it to MaxTxnBytesPerBlock.
func benchProposalBlock(numTxns int) bookkeeping.Block {
	maxBytes := config.Consensus[protocol.ConsensusCurrentVersion].MaxTxnBytesPerBlock
	var payset transactions.Payset
	size := 0
	for i := 0; numTxns < 0 || i < numTxns; i++ {
		h, sh := seededHash(uint64(i)), seededHash(uint64(i), 1)
		var snd, rcv basics.Address
		copy(snd[:], h[:32])
		copy(rcv[:], h[32:])
		var sig crypto.Signature
		copy(sig[:], sh[:])
		stib := transactions.SignedTxnInBlock{
			SignedTxnWithAD: transactions.SignedTxnWithAD{SignedTxn: transactions.SignedTxn{Sig: sig, Txn: transactions.Transaction{
				Type:             protocol.PaymentTx,
				Header:           transactions.Header{Sender: snd, Fee: basics.MicroAlgos{Raw: 1000}, FirstValid: 1, LastValid: 1000},
				PaymentTxnFields: transactions.PaymentTxnFields{Receiver: rcv, Amount: basics.MicroAlgos{Raw: uint64(i) + 1}},
			}}},
			HasGenesisID: true, HasGenesisHash: true,
		}
		n := stib.GetEncodedLength()
		if numTxns < 0 && size+n > maxBytes {
			break
		}
		size += n
		payset = append(payset, stib)
	}
	return bookkeeping.Block{
		BlockHeader: bookkeeping.BlockHeader{
			UpgradeState: bookkeeping.UpgradeState{CurrentProtocol: protocol.ConsensusCurrentVersion},
			TimeStamp:    time.Now().Unix(),
		},
		Payset: payset,
	}
}

// benchParticipants makes one online participation account per stake entry,
// plus an online account with otherStake that this node has no keys for.
func benchParticipants(stakes []uint64, otherStake uint64) ([]account.Participation, map[basics.Address]basics.AccountData) {
	keyDilution := config.Consensus[protocol.ConsensusCurrentVersion].DefaultKeyDilution
	firstID, lastID := basics.OneTimeIDForRound(0, keyDilution), basics.OneTimeIDForRound(1000, keyDilution)
	accounts := make([]account.Participation, len(stakes))
	balances := make(map[basics.Address]basics.AccountData, len(stakes)+1)
	for i, stake := range stakes {
		h := seededHash(uint64(i), 2)
		var addr basics.Address
		copy(addr[:], h[:32])
		accounts[i] = account.Participation{
			Parent:     addr,
			VRF:        generatePseudoRandomVRF(i),
			Voting:     crypto.GenerateOneTimeSignatureSecrets(firstID.Batch, lastID.Batch-firstID.Batch+1),
			FirstValid: 0,
			LastValid:  1000,
		}
		balances[addr] = basics.AccountData{
			Status: basics.Online, MicroAlgos: basics.MicroAlgos{Raw: stake},
			VoteID: accounts[i].VotingSecrets().OneTimeSignatureVerifier, SelectionID: accounts[i].VRFSecrets().PK,
		}
	}
	if otherStake > 0 {
		h := seededHash(3)
		var other basics.Address
		copy(other[:], h[:32])
		balances[other] = basics.AccountData{Status: basics.Online, MicroAlgos: basics.MicroAlgos{Raw: otherStake}}
	}
	return accounts, balances
}

// TestMakeProposalsMatchesVerifier checks, over many periods, that the
// proposals that survive verification are exactly those of the accounts whose
// proposal vote verifies on its own.
func TestMakeProposalsMatchesVerifier(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	stakes := make([]uint64, 20)
	for i := range stakes {
		stakes[i] = uint64(i+1) * 1_000_000_000
	}
	accounts, balances := benchParticipants(stakes, 1_500_000_000_000) // keys hold ~12% of online stake
	ledger := makeTestLedger(balances)
	pn := asyncPseudonode{factory: fixedBlockFactory{blk: benchProposalBlock(10)}, keys: makeRecordingKeyManager(accounts), ledger: ledger, log: serviceLogger{logging.TestingLog(t)}}
	round := ledger.NextRound()
	partKeys := pn.loadRoundParticipationKeys(round)
	require.Len(t, partKeys, len(stakes))

	built, selected := 0, 0
	for p := period(0); p < 200; p++ {
		expected := map[basics.Address]bool{}
		for _, acc := range partKeys {
			rv := rawVote{Sender: acc.Account, Round: round, Period: p, Step: propose, Proposal: proposalValue{OriginalPeriod: p, OriginalProposer: acc.Account, BlockDigest: crypto.Digest{1}}}
			uv, err := makeVote(rv, acc.VotingSigner(), acc.VRF, ledger)
			require.NoError(t, err)
			if _, err := uv.verify(ledger); err == nil {
				expected[acc.Account] = true
			}
		}
		payloads, votes := pn.makeProposals(round, p, partKeys)
		require.Len(t, payloads, len(votes))
		got := map[basics.Address]bool{}
		for _, uv := range votes {
			if _, err := uv.verify(ledger); err == nil {
				got[uv.R.Sender] = true
			}
		}
		require.Equal(t, expected, got, "period %d", p)
		built += len(votes)
		selected += len(expected)
	}
	require.NotZero(t, selected)
	t.Logf("200 periods x %d keys: %d selected, %d proposals built", len(stakes), selected, built)
}

// BenchmarkPseudonodeProposals runs the real proposal path: MakeProposals,
// the pseudonode verifier goroutine and the async vote verifier. ns/op is the
// time until the task's output channel closes, cpu-ns/op is the process CPU
// time per op, and first-ns is the time until the first proposal event leaves
// the pseudonode.
//
// keys=1: one key with a negligible share of stake (never selected).
// keys=N>1: key 0 holds nearly all stake (selected every period) and the rest
// are never selected. This is a node hosting many accounts, one of which wins.
func BenchmarkPseudonodeProposals(b *testing.B) {
	for _, bc := range []struct {
		name string
		n    int
	}{{"empty", 0}, {"1000txns", 1000}, {"full", -1}} {
		blk := benchProposalBlock(bc.n)
		for _, numKeys := range []int{1, 10, 50} {
			b.Run(fmt.Sprintf("keys=%d/block=%s", numKeys, bc.name), func(b *testing.B) {
				stakes := make([]uint64, numKeys)
				for i := range stakes {
					stakes[i] = 1_000_000
				}
				other := uint64(1e16)
				if numKeys > 1 {
					stakes[0], other = 1e16, 0
				}
				accounts, balances := benchParticipants(stakes, other)
				ledger := makeTestLedger(balances)
				log := logging.TestingLog(b)
				log.SetLevel(logging.Error)
				verifier := MakeAsyncVoteVerifier(nil)
				defer verifier.Quit()
				pn := makePseudonode(pseudonodeParams{
					factory: fixedBlockFactory{blk: blk}, validator: testBlockValidator{},
					keys: makeRecordingKeyManager(accounts), ledger: ledger,
					voteVerifier: verifier, log: serviceLogger{log},
				})
				defer pn.Quit()
				round := ledger.NextRound()

				cpuTime := func() int64 {
					utime, stime, _ := util.GetCurrentProcessTimes()
					return utime + stime
				}
				var firstTotal time.Duration
				events := 0
				b.ReportAllocs()
				cpu0 := cpuTime()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					start := time.Now()
					ch, err := pn.MakeProposals(context.Background(), round, period(i%16))
					if err != nil {
						b.Fatal(err)
					}
					first := true
					for range ch {
						if first {
							firstTotal += time.Since(start)
							first = false
						}
						events++
					}
				}
				b.StopTimer()
				b.ReportMetric(float64(cpuTime()-cpu0)/float64(b.N), "cpu-ns/op")
				if events > 0 {
					b.ReportMetric(float64(firstTotal.Nanoseconds())/float64(b.N), "first-ns")
				}
				b.ReportMetric(float64(events)/float64(b.N), "events/op")
			})
		}
	}
}
