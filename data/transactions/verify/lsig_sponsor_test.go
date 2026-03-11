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

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestFeeSponsoredLogicSigVersion(t *testing.T) {
	partitiontest.PartitionTest(t)

	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sender := senderAddrs[0]

	// Program that always returns 1
	program12, err := logic.AssembleStringWithVersion("int 1", 12)
	require.NoError(t, err)

	program13, err := logic.AssembleStringWithVersion("int 1", 13)
	require.NoError(t, err)

	p12 := logic.Program(program12.Program)
	p13 := logic.Program(program13.Program)
	sponsor12 := basics.Address(crypto.HashObj(&p12))
	sponsor13 := basics.Address(crypto.HashObj(&p13))

	blkHdr := createFeeSponsoredBlockHeader()
	dummyLedger := DummyLedgerForSignature{}

	t.Run("Version12_ShouldFail", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor12, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor12
		stxn.Ssig.Lsig = transactions.LogicSig{Logic: program12.Program}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.Error(t, err)
		require.ErrorIs(t, err, errSponsorLogicSigVersionTooLow)
	})

	t.Run("Version13_ShouldPass", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor13, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor13
		stxn.Ssig.Lsig = transactions.LogicSig{Logic: program13.Program}

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

	// Program that always returns 1
	program12, err := logic.AssembleStringWithVersion("int 1", 12)
	require.NoError(t, err)

	program13, err := logic.AssembleStringWithVersion("int 1", 13)
	require.NoError(t, err)

	blkHdr := createFeeSponsoredBlockHeader()
	dummyLedger := DummyLedgerForSignature{}

	t.Run("DelegatedVersion12_ShouldFail", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor

		st := transactions.SponsoredTransaction{Txn: tx, Sponsor: sponsor}
		sig := sponsorSecrets[0].Sign(st)

		stxn.Ssig.Lsig = transactions.LogicSig{Logic: program12.Program, Sig: sig}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.Error(t, err)
		require.ErrorIs(t, err, errSponsorLogicSigVersionTooLow)
	})

	t.Run("DelegatedVersion13_ShouldPass", func(t *testing.T) {
		tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)
		stxn := tx.Sign(senderSecrets[0])
		stxn.Ssig.Sponsor = sponsor

		st := transactions.SponsoredTransaction{Txn: tx, Sponsor: sponsor}
		sig := sponsorSecrets[0].Sign(st)

		stxn.Ssig.Lsig = transactions.LogicSig{Logic: program13.Program, Sig: sig}

		groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, &dummyLedger, nil)
		require.NoError(t, err)
		err = verifyTxn(0, groupCtx)
		require.NoError(t, err)
	})
}
