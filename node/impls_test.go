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

package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/committee"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/algorand/go-algorand/util/execpool"
)

// TestBlockValidatorImplNoMutation pins the BlockValidator contract that the
// agreement package relies on when it memoizes a proposal's encoding digest at
// validation time and when it matches verified payloads against
// proposal-values derived from votes: Validate must not mutate the input block
// -- not even in place through the payset's shared backing arrays -- and the
// returned ValidatedBlock must carry content byte-identical to that input.
func TestBlockValidatorImplNoMutation(t *testing.T) {
	partitiontest.PartitionTest(t)

	const numUsers = 10
	log := logging.TestingLog(t)
	secrets := make([]*crypto.SignatureSecrets, numUsers)
	addresses := make([]basics.Address, numUsers)

	genesis := make(map[basics.Address]basics.AccountData)
	for i := 0; i < numUsers; i++ {
		secret := keypair()
		addr := basics.Address(secret.SignatureVerifier)
		secrets[i] = secret
		addresses[i] = addr
		genesis[addr] = basics.AccountData{
			Status:            basics.Online,
			MicroAlgos:        basics.MicroAlgos{Raw: 10000000000000},
			IncentiveEligible: true,
		}
	}
	genesis[poolAddr] = basics.AccountData{
		Status:     basics.NotParticipating,
		MicroAlgos: basics.MicroAlgos{Raw: config.Consensus[protocol.ConsensusCurrentVersion].MinBalance},
	}
	// fund the fee sink so an eligible proposer earns a non-zero payout below
	genesis[sinkAddr] = basics.AccountData{
		Status:     basics.NotParticipating,
		MicroAlgos: basics.MicroAlgos{Raw: 1000000000000},
	}

	genBal := bookkeeping.MakeGenesisBalances(genesis, sinkAddr, poolAddr)
	const inMem = true
	cfg := config.GetDefaultLocal()
	cfg.Archival = true
	ledger, err := data.LoadLedger(log, t.Name(), inMem, protocol.ConsensusCurrentVersion, genBal, genesisID, genesisHash, cfg)
	require.NoError(t, err)
	defer ledger.Close()

	// fill the next block with a few payment transactions so the contract is
	// exercised on a non-empty payset
	prev, err := ledger.BlockHdr(ledger.LastRound())
	require.NoError(t, err)
	next := bookkeeping.MakeBlock(prev)
	blockEval, err := ledger.StartEvaluator(next.BlockHeader, 0, 0, nil)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		tx := transactions.Transaction{
			Type: protocol.PaymentTx,
			Header: transactions.Header{
				Sender:      addresses[i],
				Fee:         basics.MicroAlgos{Raw: proto.MinTxnFee * 2},
				FirstValid:  0,
				LastValid:   basics.Round(proto.MaxTxnLife),
				GenesisHash: genesisHash,
			},
			PaymentTxnFields: transactions.PaymentTxnFields{
				Receiver: addresses[i+1],
				Amount:   basics.MicroAlgos{Raw: mockBalancesMinBalance},
			},
		}
		stxn := tx.Sign(secrets[i])
		require.NoError(t, blockEval.TransactionGroup(transactions.SignedTxnWithAD{SignedTxn: stxn}))
	}

	// include an app call whose ApplyData carries an EvalDelta (logs) and a
	// created application ID -- ApplyData recomputation during validation is
	// the most plausible in-place mutation channel
	ops, err := logic.AssembleString("#pragma version 8\nbyte \"hello\"\nlog\nint 1")
	require.NoError(t, err)
	appcall := transactions.Transaction{
		Type: protocol.ApplicationCallTx,
		Header: transactions.Header{
			Sender:      addresses[6],
			Fee:         basics.MicroAlgos{Raw: proto.MinTxnFee * 2},
			FirstValid:  0,
			LastValid:   basics.Round(proto.MaxTxnLife),
			GenesisHash: genesisHash,
		},
		ApplicationCallTxnFields: transactions.ApplicationCallTxnFields{
			ApprovalProgram:   ops.Program,
			ClearStateProgram: ops.Program,
		},
	}
	require.NoError(t, blockEval.TransactionGroup(transactions.SignedTxnWithAD{SignedTxn: appcall.Sign(secrets[6])}))

	ub, err := blockEval.GenerateBlock(addresses)
	require.NoError(t, err)
	// finish as a payout-eligible proposer (when the current protocol pays
	// out) so the Proposer and ProposerPayout header fields are exercised too
	blk := ub.FinishBlock(committee.Seed{0x01}, addresses[0], proto.Payouts.Enabled)
	require.NotEmpty(t, blk.Payset)
	if proto.Payouts.Enabled {
		require.Equal(t, addresses[0], blk.Proposer())
		require.False(t, blk.ProposerPayout().IsZero())
	}

	backlogPool := execpool.MakeBacklog(nil, 0, execpool.LowPriority, t)
	defer backlogPool.Shutdown()
	bv := blockValidatorImpl{l: ledger, verificationPool: backlogPool}

	before := protocol.Encode(&blk)
	vb, err := bv.Validate(context.Background(), blk)
	require.NoError(t, err)

	// the input block's bytes are unchanged, including anything reachable
	// through the payset's backing arrays
	require.Equal(t, before, protocol.Encode(&blk))

	// the returned block is byte-identical to the input
	returned := vb.Block()
	require.Equal(t, before, protocol.Encode(&returned))
}
