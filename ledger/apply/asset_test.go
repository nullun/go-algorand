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

package apply

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	ledgertesting "github.com/algorand/go-algorand/ledger/testing"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestAssetTransfer(t *testing.T) {
	partitiontest.PartitionTest(t)

	src := ledgertesting.RandomAddress()
	dst := ledgertesting.RandomAddress()
	cls := ledgertesting.RandomAddress()

	var total, toSend, dstAmount uint64
	total = 1000000
	dstAmount = 500
	toSend = 200

	// prepare data
	var addrs = map[basics.Address]basics.AccountData{
		src: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total - dstAmount},
			},
		},
		dst: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: dstAmount},
			},
		},
		cls: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 0},
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusCurrentVersion, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     dst,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:     1,
			AssetAmount:   toSend,
			AssetReceiver: src,
			AssetCloseTo:  cls,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	if config.Consensus[protocol.ConsensusCurrentVersion].EnableAssetCloseAmount {
		require.Equal(t, uint64(0), addrs[dst].Assets[1].Amount)
		require.Equal(t, dstAmount-toSend, ad.AssetClosingAmount)
		require.Equal(t, total-dstAmount+toSend, addrs[src].Assets[1].Amount)
		require.Equal(t, dstAmount-toSend, addrs[cls].Assets[1].Amount)
	}
}

// TestAssetDelegationApprove tests the ApproveAssetDelegation flow where a
// delegator opts in another account to an asset and covers their MBR.
func TestAssetDelegationApprove(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - delegator is also the creator (so GetCreator returns zero address
	// which matches an account with asset params)
	var addrs = map[basics.Address]basics.AccountData{
		// Use zero address as creator since mockBalances.GetCreator returns zero address
		{}: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total},
			},
		},
		delegator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			// recipient does NOT have an asset holding yet
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	// Create a transaction that delegates the asset to recipient
	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.ApproveAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the recipient now has an asset holding with the delegator set
	recipientAcct, err := mockBal.Get(recipient, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), recipientAcct.TotalAssets)
	require.Equal(t, uint64(1), recipientAcct.TotalAssetsDelegated)

	recipientHolding, ok, err := mockBal.GetAssetHolding(recipient, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, delegator, recipientHolding.Delegator)
	require.Equal(t, uint64(0), recipientHolding.Amount)

	// Verify the delegator's TotalAssetsDelegating was incremented
	delegatorAcct, err := mockBal.Get(delegator, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), delegatorAcct.TotalAssetsDelegating)
}

// TestAssetDelegationApproveExistingHoldingFails tests that approving delegation
// for an account that already has an asset holding fails.
func TestAssetDelegationApproveExistingHoldingFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient already has an asset holding
	// Use zero address as creator since mockBalances.GetCreator returns zero address
	var addrs = map[basics.Address]basics.AccountData{
		{}: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total - 100},
			},
		},
		delegator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 100}, // Already has an asset holding
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.ApproveAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "cannot approve delegation for an existing asset holding")
}

// TestAssetDelegationRevoke tests the RevokeAssetDelegation flow where a
// delegator revokes their delegation from an account with zero balance.
func TestAssetDelegationRevoke(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has a delegated asset holding with zero balance
	// Use zero address as creator since mockBalances.GetCreator returns zero address
	var addrs = map[basics.Address]basics.AccountData{
		{}: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total},
			},
		},
		delegator: {
			MicroAlgos:            basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegating: 1,
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 0, Delegator: delegator}, // Zero balance, delegated
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.RevokeAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the recipient no longer has the asset holding
	recipientAcct, err := mockBal.Get(recipient, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), recipientAcct.TotalAssets)
	require.Equal(t, uint64(0), recipientAcct.TotalAssetsDelegated)

	_, ok, err := mockBal.GetAssetHolding(recipient, 1)
	require.NoError(t, err)
	require.False(t, ok)

	// Verify the delegator's TotalAssetsDelegating was decremented
	delegatorAcct, err := mockBal.Get(delegator, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), delegatorAcct.TotalAssetsDelegating)
}

// TestAssetDelegationRevokeNonZeroBalanceFails tests that revoking delegation
// for an account with non-zero balance fails.
func TestAssetDelegationRevokeNonZeroBalanceFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has a delegated asset holding with non-zero balance
	// Use zero address as creator since mockBalances.GetCreator returns zero address
	var addrs = map[basics.Address]basics.AccountData{
		{}: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total - 100},
			},
		},
		delegator: {
			MicroAlgos:            basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegating: 1,
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 100, Delegator: delegator}, // Non-zero balance
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.RevokeAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "cannot revoke delegation from an asset holding with non-zero balance")
}

// TestAssetDelegationRevokeWrongDelegatorFails tests that only the original
// delegator can revoke their delegation.
func TestAssetDelegationRevokeWrongDelegatorFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	wrongDelegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()
	creator := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has a delegated asset holding
	var addrs = map[basics.Address]basics.AccountData{
		creator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total},
			},
		},
		delegator: {
			MicroAlgos:            basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegating: 1,
		},
		wrongDelegator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 0, Delegator: delegator}, // Delegated by 'delegator', not 'wrongDelegator'
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     wrongDelegator, // Wrong sender trying to revoke
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.RevokeAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "only the delegator can revoke their delegation")
}

// TestAssetDelegationRevokeNonExistentFails tests that revoking delegation
// for a non-existent asset holding fails.
func TestAssetDelegationRevokeNonExistentFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()
	creator := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has no asset holding at all
	var addrs = map[basics.Address]basics.AccountData{
		creator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total},
			},
		},
		delegator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			// No asset holdings
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     0,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.RevokeAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "cannot revoke delegation for a non-existent asset holding")
}

// TestAssetOptInDischargesDelegator tests that when an account performs an
// "OptIn" (self-transfer of zero amount) for an asset that was delegated,
// the delegator is discharged from their MBR responsibility.
func TestAssetOptInDischargesDelegator(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()
	creator := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has a delegated asset holding
	var addrs = map[basics.Address]basics.AccountData{
		creator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total - 50},
			},
		},
		delegator: {
			MicroAlgos:            basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegating: 1,
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 50, Delegator: delegator},
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	// Recipient performs an opt-in (self-transfer of 0)
	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     recipient,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:     1,
			AssetAmount:   0,
			AssetReceiver: recipient, // self-transfer
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the delegator has been discharged
	recipientAcct, err := mockBal.Get(recipient, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), recipientAcct.TotalAssets)
	require.Equal(t, uint64(0), recipientAcct.TotalAssetsDelegated)

	recipientHolding, ok, err := mockBal.GetAssetHolding(recipient, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, recipientHolding.Delegator.IsZero())  // Delegator cleared
	require.Equal(t, uint64(50), recipientHolding.Amount) // Amount unchanged

	// Verify the delegator's TotalAssetsDelegating was decremented
	delegatorAcct, err := mockBal.Get(delegator, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), delegatorAcct.TotalAssetsDelegating)
}

// TestAssetCloseOutDelegated tests that closing out a delegated asset holding
// properly updates the delegator's TotalAssetsDelegating counter.
func TestAssetCloseOutDelegated(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()
	creator := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - recipient has a delegated asset holding with some balance
	var addrs = map[basics.Address]basics.AccountData{
		creator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total - 50},
			},
		},
		delegator: {
			MicroAlgos:            basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegating: 1,
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 50, Delegator: delegator},
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	// Recipient closes out the asset to the creator
	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     recipient,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:     1,
			AssetAmount:   0,
			AssetReceiver: creator,
			AssetCloseTo:  creator, // Close to creator
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the recipient no longer has the asset holding
	recipientAcct, err := mockBal.Get(recipient, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), recipientAcct.TotalAssets)
	require.Equal(t, uint64(0), recipientAcct.TotalAssetsDelegated)

	_, ok, err := mockBal.GetAssetHolding(recipient, 1)
	require.NoError(t, err)
	require.False(t, ok)

	// Verify the delegator's TotalAssetsDelegating was decremented
	delegatorAcct, err := mockBal.Get(delegator, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), delegatorAcct.TotalAssetsDelegating)

	// Verify close amount
	require.Equal(t, uint64(50), ad.AssetClosingAmount)
}

// TestAssetDelegationRevokeWithNonZeroAmountFails tests that revoking delegation
// while also attempting to transfer a non-zero amount of the asset fails.
// This is because the recipient's holding is deleted during the revocation
// phase, before the asset transfer (putIn) can complete.
func TestAssetDelegationRevokeWithNonZeroAmountFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	delegator := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	var total uint64 = 1000000

	// prepare data - delegator is the creator and has the asset balance.
	// recipient has a delegated asset holding with zero balance.
	var addrs = map[basics.Address]basics.AccountData{
		delegator: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
			AssetParams: map[basics.AssetIndex]basics.AssetParams{
				1: {Total: total},
			},
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: total},
			},
			TotalAssetsDelegating: 1,
		},
		recipient: {
			MicroAlgos:           basics.MicroAlgos{Raw: 10000000},
			TotalAssetsDelegated: 1,
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 0, Delegator: delegator}, // Zero balance, delegated
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	// Attempt to revoke delegation while also sending 50 units of the asset.
	tx := transactions.Transaction{
		Type: protocol.AssetTransferTx,
		Header: transactions.Header{
			Sender:     delegator,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		AssetTransferTxnFields: transactions.AssetTransferTxnFields{
			XferAsset:       1,
			AssetAmount:     50,
			AssetReceiver:   recipient,
			AssetDelegation: transactions.RevokeAssetDelegation,
		},
	}

	var ad transactions.ApplyData
	err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)

	// Verify the failure: revocation deletes the holding, so the subsequent
	// putIn operation fails because the recipient is no longer opted in.
	require.ErrorContains(t, err, "receiver error: must optin")
}
