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

package wallet

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// mockMultiAccountWallet is a mock implementation of MultiAccountWallet for testing
type mockMultiAccountWallet struct {
	// Embed a nil Wallet interface to satisfy Wallet methods we don't need to test
	accounts map[uint32]crypto.Digest
}

func (m *mockMultiAccountWallet) Init(pw []byte) error {
	return nil
}

func (m *mockMultiAccountWallet) CheckPassword(pw []byte) error {
	return nil
}

func (m *mockMultiAccountWallet) ExportMasterDerivationKey(pw []byte) (crypto.MasterDerivationKey, error) {
	return crypto.MasterDerivationKey{}, nil
}

func (m *mockMultiAccountWallet) Metadata() (Metadata, error) {
	return Metadata{
		ID:                   []byte("mock-wallet-id"),
		Name:                 []byte("Mock Multi-Account Wallet"),
		DriverName:           "mock",
		SupportsMultiAccount: true,
	}, nil
}

func (m *mockMultiAccountWallet) ListKeys() ([]crypto.Digest, error) {
	keys := make([]crypto.Digest, 0, len(m.accounts))
	for _, digest := range m.accounts {
		keys = append(keys, digest)
	}
	return keys, nil
}

func (m *mockMultiAccountWallet) ImportKey(sk crypto.PrivateKey) (crypto.Digest, error) {
	return crypto.Digest{}, nil
}

func (m *mockMultiAccountWallet) ExportKey(pk crypto.Digest, pw []byte) (crypto.PrivateKey, error) {
	return crypto.PrivateKey{}, nil
}

func (m *mockMultiAccountWallet) GenerateKey(displayMnemonic bool) (crypto.Digest, error) {
	return crypto.Digest{}, nil
}

func (m *mockMultiAccountWallet) DeleteKey(pk crypto.Digest, pw []byte) error {
	return nil
}

func (m *mockMultiAccountWallet) ImportMultisigAddr(version, threshold uint8, pks []crypto.PublicKey) (crypto.Digest, error) {
	return crypto.Digest{}, nil
}

func (m *mockMultiAccountWallet) LookupMultisigPreimage(crypto.Digest) (version, threshold uint8, pks []crypto.PublicKey, err error) {
	return 0, 0, nil, nil
}

func (m *mockMultiAccountWallet) ListMultisigAddrs() (addrs []crypto.Digest, err error) {
	return nil, nil
}

func (m *mockMultiAccountWallet) DeleteMultisigAddr(addr crypto.Digest, pw []byte) error {
	return nil
}

func (m *mockMultiAccountWallet) SignTransaction(tx transactions.Transaction, pk crypto.PublicKey, pw []byte) ([]byte, error) {
	return nil, nil
}

func (m *mockMultiAccountWallet) MultisigSignTransaction(tx transactions.Transaction, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, signer crypto.Digest) (crypto.MultisigSig, error) {
	return crypto.MultisigSig{}, nil
}

func (m *mockMultiAccountWallet) SignProgram(program []byte, src crypto.Digest, pw []byte) ([]byte, error) {
	return nil, nil
}

func (m *mockMultiAccountWallet) MultisigSignProgram(program []byte, src crypto.Digest, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, useLegacyMsig bool) (crypto.MultisigSig, error) {
	return crypto.MultisigSig{}, nil
}

// MultiAccountWallet methods
func (m *mockMultiAccountWallet) GetPublicKeyForAccount(accountIndex uint32) (crypto.Digest, error) {
	if digest, ok := m.accounts[accountIndex]; ok {
		return digest, nil
	}
	// Generate a deterministic key for the account index
	var digest crypto.Digest
	digest[0] = byte(accountIndex)
	return digest, nil
}

func (m *mockMultiAccountWallet) ListKeysForAccounts(accountIndices []uint32) ([]crypto.Digest, error) {
	keys := make([]crypto.Digest, len(accountIndices))
	for i, idx := range accountIndices {
		key, err := m.GetPublicKeyForAccount(idx)
		if err != nil {
			return nil, err
		}
		keys[i] = key
	}
	return keys, nil
}

func (m *mockMultiAccountWallet) SignTransactionWithAccount(tx transactions.Transaction, pk crypto.PublicKey, pw []byte, accountIndex uint32) ([]byte, error) {
	return []byte("mock-signature"), nil
}

func (m *mockMultiAccountWallet) MultisigSignTransactionWithAccount(tx transactions.Transaction, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, signer crypto.Digest, accountIndex uint32) (crypto.MultisigSig, error) {
	return crypto.MultisigSig{}, nil
}

// TestMultiAccountWalletInterface verifies that the MultiAccountWallet interface
// can be properly used with type assertions
func TestMultiAccountWalletInterface(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Create a mock multi-account wallet
	mock := &mockMultiAccountWallet{
		accounts: map[uint32]crypto.Digest{
			0: {0x00},
			1: {0x01},
			5: {0x05},
		},
	}

	// Test that it satisfies both Wallet and MultiAccountWallet interfaces
	var w Wallet = mock
	require.NotNil(t, w)

	maw, ok := w.(MultiAccountWallet)
	require.True(t, ok, "mockMultiAccountWallet should satisfy MultiAccountWallet interface")

	// Test GetPublicKeyForAccount
	key0, err := maw.GetPublicKeyForAccount(0)
	require.NoError(t, err)
	require.Equal(t, byte(0x00), key0[0])

	key1, err := maw.GetPublicKeyForAccount(1)
	require.NoError(t, err)
	require.Equal(t, byte(0x01), key1[0])

	// Test ListKeysForAccounts
	keys, err := maw.ListKeysForAccounts([]uint32{0, 1, 5})
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, byte(0x00), keys[0][0])
	require.Equal(t, byte(0x01), keys[1][0])
	require.Equal(t, byte(0x05), keys[2][0])

	// Test Metadata reports SupportsMultiAccount
	meta, err := maw.Metadata()
	require.NoError(t, err)
	require.True(t, meta.SupportsMultiAccount)
}

// TestNonMultiAccountWallet verifies that a regular wallet does not
// satisfy the MultiAccountWallet interface
func TestNonMultiAccountWallet(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// A simple non-multi-account wallet mock
	type simpleWallet struct {
		Wallet
	}

	// Note: This will fail at runtime if you try to type assert
	// We're just showing that a Wallet that doesn't explicitly implement
	// MultiAccountWallet won't satisfy the interface
	var w Wallet = &simpleWallet{}
	_, ok := w.(MultiAccountWallet)
	require.False(t, ok, "simpleWallet should NOT satisfy MultiAccountWallet interface")
}
