// Copyright (C) 2019-2026 Algorand, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// verifyTxn prepares and verifies the signatures of a single transaction in a group.
func verifyTxn(gi int, groupCtx *GroupContext) error {
	batchVerifier := crypto.MakeBatchVerifier()
	if err := txnBatchPrep(gi, groupCtx, batchVerifier); err != nil {
		return err
	}
	return batchVerifier.Verify()
}

// sponsorLogicSigVersions returns the lowest LogicSig version a sponsor may use
// under ConsensusFuture, and the highest version below it, which must be rejected.
func sponsorLogicSigVersions(t *testing.T) (minVersion, belowMinVersion uint64) {
	t.Helper()
	proto := config.Consensus[protocol.ConsensusFuture]
	require.NotZero(t, proto.MinSponsorLogicSigVersion, "no minimum sponsor LogicSig version configured")
	return proto.MinSponsorLogicSigVersion, proto.MinSponsorLogicSigVersion - 1
}

func TestFeeSponsoredLogicSigVersion(t *testing.T) {
	partitiontest.PartitionTest(t)

	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sender := senderAddrs[0]

	minVersion, belowMinVersion := sponsorLogicSigVersions(t)

	// Program that always returns 1
	programBelowMin, err := logic.AssembleStringWithVersion("int 1", belowMinVersion)
	require.NoError(t, err)

	programAtMin, err := logic.AssembleStringWithVersion("int 1", minVersion)
	require.NoError(t, err)

	pBelow := logic.Program(programBelowMin.Program)
	pAtMin := logic.Program(programAtMin.Program)
	sponsorBelowMin := basics.Address(crypto.HashObj(&pBelow))
	sponsorAtMin := basics.Address(crypto.HashObj(&pAtMin))

	blkHdr := createFeeSponsoredBlockHeader()
	dummyLedger := DummyLedgerForSignature{}

	t.Run("BelowMinVersion_ShouldFail", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsorBelowMin, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsorBelowMin
		stxn.Ssig.Lsig = transactions.LogicSig{Logic: programBelowMin.Program}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.Error(t, err)
		require.ErrorIs(t, err, errSponsorLogicSigVersionTooLow)
	})

	t.Run("MinVersion_ShouldPass", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsorAtMin, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsorAtMin
		stxn.Ssig.Lsig = transactions.LogicSig{Logic: programAtMin.Program}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.NoError(t, err)
	})
}

func TestFeeSponsoredDelegatedLogicSigVersion(t *testing.T) {
	partitiontest.PartitionTest(t)

	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sender := senderAddrs[0]

	sponsorSecrets, sponsorAddrs, _ := generateAccounts(1)
	sponsor := sponsorAddrs[0]

	minVersion, belowMinVersion := sponsorLogicSigVersions(t)

	// Program that always returns 1
	programBelowMin, err := logic.AssembleStringWithVersion("int 1", belowMinVersion)
	require.NoError(t, err)

	programAtMin, err := logic.AssembleStringWithVersion("int 1", minVersion)
	require.NoError(t, err)

	blkHdr := createFeeSponsoredBlockHeader()
	dummyLedger := DummyLedgerForSignature{}

	t.Run("DelegatedBelowMinVersion_ShouldFail", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor

		st := transactions.SponsoredTransaction{Txn: tx, Sponsor: sponsor}
		sig := sponsorSecrets[0].Sign(st)

		stxn.Ssig.Lsig = transactions.LogicSig{Logic: programBelowMin.Program, Sig: sig}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.Error(t, err)
		require.ErrorIs(t, err, errSponsorLogicSigVersionTooLow)
	})

	t.Run("DelegatedMinVersion_ShouldPass", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor

		st := transactions.SponsoredTransaction{Txn: tx, Sponsor: sponsor}
		sig := sponsorSecrets[0].Sign(st)

		stxn.Ssig.Lsig = transactions.LogicSig{Logic: programAtMin.Program, Sig: sig}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.NoError(t, err)
	})
}
