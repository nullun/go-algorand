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
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	ledgertesting "github.com/algorand/go-algorand/ledger/testing"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

var spec = transactions.SpecialAddresses{
	FeeSink:     ledgertesting.RandomAddress(),
	RewardsPool: ledgertesting.RandomAddress(),
}

func TestAlgosEncoding(t *testing.T) {
	partitiontest.PartitionTest(t)

	var a basics.MicroAlgos
	var b basics.MicroAlgos
	var i uint64

	a.Raw = 222233333
	err := protocol.Decode(protocol.Encode(&a), &b)
	if err != nil {
		panic(err)
	}
	require.Equal(t, a, b)

	a.Raw = 12345678
	err = protocol.DecodeReflect(protocol.Encode(a), &i)
	if err != nil {
		panic(err)
	}
	require.Equal(t, a.Raw, i)

	i = 87654321
	err = protocol.Decode(protocol.EncodeReflect(i), &a)
	if err != nil {
		panic(err)
	}
	require.Equal(t, a.Raw, i)

	x := true
	err = protocol.Decode(protocol.EncodeReflect(x), &a)
	if err == nil {
		panic("decode of bool into MicroAlgos succeeded")
	}
}

func TestPaymentApply(t *testing.T) {
	partitiontest.PartitionTest(t)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     ledgertesting.RandomAddress(),
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver: ledgertesting.RandomAddress(),
			Amount:   basics.MicroAlgos{Raw: uint64(50)},
		},
	}

	mockBalV0 := makeMockBalances(protocol.ConsensusCurrentVersion)
	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBalV0, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)
}

func TestPaymentValidation(t *testing.T) {
	partitiontest.PartitionTest(t)

	current := config.Consensus[protocol.ConsensusCurrentVersion]
	for _, txn := range generateTestPays(100) {
		// Check malformed transactions
		largeWindow := txn
		largeWindow.LastValid += basics.Round(current.MaxTxnLife)
		if largeWindow.WellFormed(spec, current) == nil {
			t.Errorf("transaction with large window %#v verified incorrectly", largeWindow)
		}

		badWindow := txn
		badWindow.LastValid = badWindow.FirstValid - 1
		if badWindow.WellFormed(spec, current) == nil {
			t.Errorf("transaction with bad window %#v verified incorrectly", badWindow)
		}

		badFee := txn
		badFee.Fee = basics.MicroAlgos{}
		if badFee.WellFormed(spec, config.Consensus[protocol.ConsensusV27]) == nil {
			t.Errorf("transaction with no fee %#v verified incorrectly", badFee)
		}
		assert.NoError(t, badFee.WellFormed(spec, current))

		badFee.Fee.Raw = 1
		if badFee.WellFormed(spec, config.Consensus[protocol.ConsensusV27]) == nil {
			t.Errorf("transaction with low fee %#v verified incorrectly", badFee)
		}
		assert.NoError(t, badFee.WellFormed(spec, current))
	}
}

func TestPaymentSelfClose(t *testing.T) {
	partitiontest.PartitionTest(t)

	self := ledgertesting.RandomAddress()

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     self,
			Fee:        basics.MicroAlgos{Raw: config.Consensus[protocol.ConsensusCurrentVersion].MinTxnFee},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         ledgertesting.RandomAddress(),
			Amount:           basics.MicroAlgos{Raw: uint64(50)},
			CloseRemainderTo: self,
		},
	}
	require.Error(t, tx.WellFormed(spec, config.Consensus[protocol.ConsensusCurrentVersion]))
}

func generateTestPays(numTxs int) []transactions.Transaction {
	txs := make([]transactions.Transaction, numTxs)
	for i := range numTxs {
		a := rand.IntN(1000)
		f := config.Consensus[protocol.ConsensusCurrentVersion].MinTxnFee + uint64(rand.IntN(10))
		iss := 50 + rand.IntN(30)
		exp := iss + 10

		txs[i] = transactions.Transaction{
			Type: protocol.PaymentTx,
			Header: transactions.Header{
				Sender:      ledgertesting.RandomAddress(),
				Fee:         basics.MicroAlgos{Raw: f},
				FirstValid:  basics.Round(iss),
				LastValid:   basics.Round(exp),
				GenesisHash: crypto.Digest{0x02},
			},
			PaymentTxnFields: transactions.PaymentTxnFields{
				Receiver: ledgertesting.RandomAddress(),
				Amount:   basics.MicroAlgos{Raw: uint64(a)},
			},
		}
	}
	return txs
}

// TestAccountBootstrapApply tests the BootstrapAccount flow where a
// bootstrapper creates a new account and covers their initial MBR.
func TestAccountBootstrapApply(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	newAccount := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			MicroAlgos: basics.MicroAlgos{Raw: uint64(50)},
		},
		// newAccount intentionally not in map - it doesn't exist yet
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         newAccount,
			Amount:           basics.MicroAlgos{Raw: 0},
			AccountBootstrap: transactions.BootstrapAccount,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the new account was bootstrapped
	newAcctData, err := mockBal.Get(newAccount, false)
	require.NoError(t, err)
	require.Equal(t, bootstrapper, newAcctData.Bootstrapper)

	// Verify the bootstrapper's TotalAccountsBootstrapping was incremented
	bootstrapperData, err := mockBal.Get(bootstrapper, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), bootstrapperData.TotalAccountsBootstrapping)
}

// TestAccountBootstrapExistingAccountFails tests that bootstrapping an
// existing account fails.
func TestAccountBootstrapExistingAccountFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	existingAccount := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
		existingAccount: {
			MicroAlgos: basics.MicroAlgos{Raw: 100}, // Account already exists with balance
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         existingAccount,
			Amount:           basics.MicroAlgos{Raw: 0},
			AccountBootstrap: transactions.BootstrapAccount,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "cannot bootstrap account: account already exists")
}

// TestAccountRescindApply tests the RescindAccount flow where a
// bootstrapper rescinds their bootstrap from an empty account.
func TestAccountRescindApply(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	bootstrappedAccount := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			MicroAlgos:                 basics.MicroAlgos{Raw: 10000000},
			TotalAccountsBootstrapping: 1,
		},
		bootstrappedAccount: {
			MicroAlgos:   basics.MicroAlgos{Raw: 0},
			Bootstrapper: bootstrapper,
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         bootstrappedAccount,
			Amount:           basics.MicroAlgos{Raw: 0},
			AccountBootstrap: transactions.RescindAccount,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.NoError(t, err)

	// Verify the bootstrapped account was cleared
	// (CloseAccount is called, so the record should be zeroed)
	rescindedAcctData, err := mockBal.Get(bootstrappedAccount, false)
	require.NoError(t, err)
	require.True(t, rescindedAcctData.IsZero())

	// Verify the bootstrapper's TotalAccountsBootstrapping was decremented
	bootstrapperData, err := mockBal.Get(bootstrapper, false)
	require.NoError(t, err)
	require.Equal(t, uint64(0), bootstrapperData.TotalAccountsBootstrapping)
}

// TestAccountRescindWithDelegatedAssetsFails tests that rescinding a
// bootstrapped account with delegated assets fails.
func TestAccountRescindWithDelegatedAssetsFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	bootstrappedAccount := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			MicroAlgos:                 basics.MicroAlgos{Raw: 10000000},
			TotalAccountsBootstrapping: 1,
		},
		bootstrappedAccount: {
			MicroAlgos:           basics.MicroAlgos{Raw: 0},
			Bootstrapper:         bootstrapper,
			TotalAssetsDelegated: 1, // Has delegated assets
			// Must include corresponding asset entry to avoid MinBalance underflow
			// (MinBalance calculation: adjustedTotalAssets = len(Assets) - TotalAssetsDelegated)
			Assets: map[basics.AssetIndex]basics.AssetHolding{
				1: {Amount: 0, Frozen: false, Delegator: bootstrapper},
			},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         bootstrappedAccount,
			Amount:           basics.MicroAlgos{Raw: 0},
			AccountBootstrap: transactions.RescindAccount,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "outstanding delegated assets")
}

// TestCloseAccountWithBootstrapping tests that closing an account
// that is bootstrapping another account requires sufficient balance.
// Note: mockBalances.Move() doesn't actually move funds, so we can only test
// the insufficient balance check here. Full integration tests would be needed
// to test the complete close-with-bootstrap flow.
func TestCloseAccountWithBootstrappingInsufficientBalance(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	bootstrappedAccount := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			// Balance is 0 - the close will fail because there's insufficient
			// balance to remove the bootstrap (needs >= MinBalance).
			MicroAlgos:                 basics.MicroAlgos{Raw: 0},
			TotalAccountsBootstrapping: 1,
		},
		bootstrappedAccount: {
			MicroAlgos:   basics.MicroAlgos{Raw: 0},
			Bootstrapper: bootstrapper,
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         bootstrappedAccount,
			Amount:           basics.MicroAlgos{Raw: 0},
			CloseRemainderTo: bootstrappedAccount,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "insufficient balance")
}

// TestCloseAccountMultipleBootstrappedFails tests that closing an account
// that is bootstrapping multiple accounts fails.
func TestCloseAccountMultipleBootstrappedFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	bootstrapper := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		bootstrapper: {
			// Balance is 0 because mockBalances.Move() doesn't actually move funds.
			MicroAlgos:                 basics.MicroAlgos{Raw: 0},
			TotalAccountsBootstrapping: 2, // Bootstrapping multiple accounts
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     bootstrapper,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         recipient,
			Amount:           basics.MicroAlgos{Raw: 0},
			CloseRemainderTo: recipient,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "outstanding bootstrapped accounts")
}

// TestCloseAccountWithDelegatingFails tests that closing an account
// that is delegating assets fails.
func TestCloseAccountWithDelegatingFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	sender := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		sender: {
			// Balance is 0 because mockBalances.Move() doesn't actually move funds.
			// In real scenarios, the balance would be moved by Move() before CloseAccount checks.
			MicroAlgos:            basics.MicroAlgos{Raw: 0},
			TotalAssetsDelegating: 1, // Is delegating an asset
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     sender,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         recipient,
			Amount:           basics.MicroAlgos{Raw: 0},
			CloseRemainderTo: recipient,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "outstanding delegating assets")
}

// TestCloseAccountWithDelegatedAssetsFails tests that closing an account
// that has delegated assets fails.
func TestCloseAccountWithDelegatedAssetsFails(t *testing.T) {
	partitiontest.PartitionTest(t)

	sender := ledgertesting.RandomAddress()
	recipient := ledgertesting.RandomAddress()

	addrs := map[basics.Address]basics.AccountData{
		sender: {
			// Balance is 0 because mockBalances.Move() doesn't actually move funds.
			// In real scenarios, the balance would be moved by Move() before CloseAccount checks.
			MicroAlgos:           basics.MicroAlgos{Raw: 0},
			TotalAssetsDelegated: 1, // Has delegated assets
		},
		recipient: {
			MicroAlgos: basics.MicroAlgos{Raw: 10000000},
		},
	}

	mockBal := makeMockBalancesWithAccounts(protocol.ConsensusFuture, addrs)

	tx := transactions.Transaction{
		Type: protocol.PaymentTx,
		Header: transactions.Header{
			Sender:     sender,
			Fee:        basics.MicroAlgos{Raw: 1},
			FirstValid: basics.Round(100),
			LastValid:  basics.Round(1000),
		},
		PaymentTxnFields: transactions.PaymentTxnFields{
			Receiver:         recipient,
			Amount:           basics.MicroAlgos{Raw: 0},
			CloseRemainderTo: recipient,
		},
	}

	var ad transactions.ApplyData
	err := Payment(tx.PaymentTxnFields, tx.Header, mockBal, transactions.SpecialAddresses{}, &ad)
	require.ErrorContains(t, err, "outstanding delegated assets")
}
