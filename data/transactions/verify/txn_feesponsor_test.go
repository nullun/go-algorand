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
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// createFeeSponsoredBlockHeader returns a block header with consensus that supports fee sponsorship
func createFeeSponsoredBlockHeader() *bookkeeping.BlockHeader {
	return &bookkeeping.BlockHeader{
		RewardsState: bookkeeping.RewardsState{
			FeeSink:     feeSink,
			RewardsPool: poolAddr,
		},
		UpgradeState: bookkeeping.UpgradeState{
			CurrentProtocol: protocol.ConsensusFuture,
		},
	}
}

// createFeeSponsoredPayment creates a fee-sponsored payment transaction
func createFeeSponsoredPayment(sender, receiver basics.Address, amount uint64, fee uint64) transactions.Transaction {
	return transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:       sender,
			Fee:          basics.MicroAlgos{Raw: fee},
			FirstValid:   1,
			LastValid:    100,
			FeeSponsored: true,
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver: receiver,
			Amount:   basics.MicroAlgos{Raw: amount},
		},
	}
}

func TestFeeSponsoredSingleSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender and sponsor accounts
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sponsorSecrets, sponsorAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Add sponsor signature
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Sig = sponsorSecrets[0].Sign(tx)

	// Verify with fee-sponsored consensus
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	require.NoError(t, verifyTxn(0, groupCtx))
}

func TestFeeSponsoredMissingSponsorSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender account
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	_, sponsorAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender only (missing sponsor sig)
	stxn := tx.Sign(senderSecrets[0])

	// Verify should fail - FeeSponsored=true but no Ssig
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	err = verifyTxn(0, groupCtx)
	require.Error(t, err)
	require.ErrorIs(t, err, errTxnSigHasIncompleteOrMissingSponsorSig)
}

func TestFeeSponsoredNotEnabled(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender and sponsor accounts
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sponsorSecrets, sponsorAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Add sponsor signature
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Sig = sponsorSecrets[0].Sign(tx)

	// Verify with non-future consensus (fee sponsorship not enabled)
	// Use a consensus version that doesn't support fee sponsorship
	blkHdr := &bookkeeping.BlockHeader{
		RewardsState: bookkeeping.RewardsState{
			FeeSink:     feeSink,
			RewardsPool: poolAddr,
		},
		UpgradeState: bookkeeping.UpgradeState{
			CurrentProtocol: protocol.ConsensusCurrentVersion,
		},
	}

	// Check if current version supports fee sponsorship
	proto := config.Consensus[protocol.ConsensusCurrentVersion]
	if proto.SupportFeeSponsored {
		t.Skip("Current consensus version supports fee sponsorship, skipping test")
	}

	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	err = verifyTxn(0, groupCtx)
	require.Error(t, err)
	require.ErrorIs(t, err, errFeeSponsoredNotSupported)
}

func TestFeeSponsoredInvalidSponsorSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender and sponsor accounts
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	_, sponsorAddrs, _ := generateAccounts(1)
	wrongSecrets, _, _ := generateAccounts(1) // Wrong key

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Add invalid sponsor signature (signed with wrong key)
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Sig = wrongSecrets[0].Sign(tx)

	// Verify should fail - wrong sponsor signature
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	err = verifyTxn(0, groupCtx)
	require.Error(t, err) // Signature verification should fail
}

func TestFeeSponsoredWithRekeying(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender, sponsor, and sponsor's auth accounts
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	_, sponsorAddrs, _ := generateAccounts(1)
	authSecrets, authAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]
	authAddr := authAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Add sponsor signature with rekeyed auth address
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.AuthAddr = authAddr
	stxn.Ssig.Sig = authSecrets[0].Sign(tx)

	// Verify with fee-sponsored consensus
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	require.NoError(t, verifyTxn(0, groupCtx))
}

func TestFeeSponsoredMultiSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender account
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sender := senderAddrs[0]

	// Create multisig sponsor account (2 of 3)
	sponsorSecrets, _, sponsorPKs := generateAccounts(3)
	sponsorMsigAddr, err := crypto.MultisigAddrGen(1, 2, sponsorPKs)
	require.NoError(t, err)
	sponsor := basics.Address(sponsorMsigAddr)

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Create multisig sponsor signature (2 of 3 signers)
	msig1, err := crypto.MultisigSign(tx, crypto.Digest(sponsor), 1, 2, sponsorPKs, *sponsorSecrets[0])
	require.NoError(t, err)
	msig2, err := crypto.MultisigSign(tx, crypto.Digest(sponsor), 1, 2, sponsorPKs, *sponsorSecrets[1])
	require.NoError(t, err)
	combinedMsig, err := crypto.MultisigAssemble([]crypto.MultisigSig{msig1, msig2})
	require.NoError(t, err)

	// Add sponsor multisig
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Msig = combinedMsig

	// Verify with fee-sponsored consensus
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	require.NoError(t, verifyTxn(0, groupCtx))
}

func TestFeeSponsoredSponsorSameAsSender(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create single account that is both sender and sponsor
	secrets, addrs, _ := generateAccounts(1)
	addr := addrs[0]

	// Create fee-sponsored transaction where sender == sponsor
	tx := createFeeSponsoredPayment(addr, addr, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(secrets[0])

	// Add sponsor signature (same as sender)
	stxn.Ssig.Sponsor = addr
	stxn.Ssig.Sig = secrets[0].Sign(tx)

	// Verify - this should be allowed at the verification level
	// (the economic impact is the same, just adds complexity)
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	// Note: The verification layer allows this - validation happens at apply time
	err = verifyTxn(0, groupCtx)
	require.NoError(t, err)
}

func TestFeeSponsoredBatchSigCount(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender and sponsor accounts
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sponsorSecrets, sponsorAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	sponsor := sponsorAddrs[0]

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Add sponsor signature
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Sig = sponsorSecrets[0].Sign(tx)

	// Check batch sig count - should be 2 (sender + sponsor)
	count, err := getNumberOfBatchableSigsInTxn(&stxn, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(2), count, "Expected 2 batchable signatures (sender + sponsor)")
}

func TestFeeSponsoredBatchSigCountMultiSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender account
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	sender := senderAddrs[0]

	// Create multisig sponsor account (2 of 3)
	sponsorSecrets, _, sponsorPKs := generateAccounts(3)
	sponsorMsigAddr, err := crypto.MultisigAddrGen(1, 2, sponsorPKs)
	require.NoError(t, err)
	sponsor := basics.Address(sponsorMsigAddr)

	// Create fee-sponsored transaction
	tx := createFeeSponsoredPayment(sender, sponsor, 1000, 1000)

	// Sign transaction as sender
	stxn := tx.Sign(senderSecrets[0])

	// Create multisig sponsor signature (2 of 3 signers)
	msig1, err := crypto.MultisigSign(tx, crypto.Digest(sponsor), 1, 2, sponsorPKs, *sponsorSecrets[0])
	require.NoError(t, err)
	msig2, err := crypto.MultisigSign(tx, crypto.Digest(sponsor), 1, 2, sponsorPKs, *sponsorSecrets[1])
	require.NoError(t, err)
	combinedMsig, err := crypto.MultisigAssemble([]crypto.MultisigSig{msig1, msig2})
	require.NoError(t, err)

	// Add sponsor multisig
	stxn.Ssig.Sponsor = sponsor
	stxn.Ssig.Msig = combinedMsig

	// Check batch sig count - should be 3 (1 sender + 2 sponsor multisig)
	count, err := getNumberOfBatchableSigsInTxn(&stxn, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(3), count, "Expected 3 batchable signatures (1 sender + 2 sponsor multisig)")
}

func TestFeeSponsoredFlagWithoutSponsorSig(t *testing.T) {
	partitiontest.PartitionTest(t)

	// Create sender account
	senderSecrets, senderAddrs, _ := generateAccounts(1)
	_, sponsorAddrs, _ := generateAccounts(1)

	sender := senderAddrs[0]
	receiver := sponsorAddrs[0]

	// Create fee-sponsored transaction but WITHOUT sponsor signature
	tx := createFeeSponsoredPayment(sender, receiver, 1000, 1000)

	// Sign transaction as sender only
	stxn := tx.Sign(senderSecrets[0])
	// Note: Ssig is empty/blank

	// Verify should fail because FeeSponsored=true but Ssig is blank
	blkHdr := createFeeSponsoredBlockHeader()
	groupCtx, err := PrepareGroupContext([]transactions.SignedTxn{stxn}, blkHdr, nil, nil)
	require.NoError(t, err)
	err = verifyTxn(0, groupCtx)
	require.Error(t, err)
	require.ErrorIs(t, err, errTxnSigHasIncompleteOrMissingSponsorSig)
}
