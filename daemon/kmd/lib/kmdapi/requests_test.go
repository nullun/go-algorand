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

package kmdapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/test/partitiontest"
)

// TestKeyListRequestAccountIndex verifies that the AccountIndex field
// is properly serialized/deserialized in JSON
func TestKeyListRequestAccountIndex(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Test with nil AccountIndex (standard behavior)
	req1 := APIV1POSTKeyListRequest{
		WalletHandleToken: "test-token",
	}
	data1, err := json.Marshal(req1)
	require.NoError(t, err)

	var decoded1 APIV1POSTKeyListRequest
	err = json.Unmarshal(data1, &decoded1)
	require.NoError(t, err)
	require.Nil(t, decoded1.AccountIndex)
	require.Equal(t, "test-token", decoded1.WalletHandleToken)

	// Test with AccountIndex set
	accountIdx := uint32(5)
	req2 := APIV1POSTKeyListRequest{
		WalletHandleToken: "test-token-2",
		AccountIndex:      &accountIdx,
	}
	data2, err := json.Marshal(req2)
	require.NoError(t, err)

	var decoded2 APIV1POSTKeyListRequest
	err = json.Unmarshal(data2, &decoded2)
	require.NoError(t, err)
	require.NotNil(t, decoded2.AccountIndex)
	require.Equal(t, uint32(5), *decoded2.AccountIndex)
}

// TestTransactionSignRequestAccountIndex verifies that the AccountIndex field
// is properly serialized/deserialized in JSON
func TestTransactionSignRequestAccountIndex(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Test with nil AccountIndex (standard behavior)
	req1 := APIV1POSTTransactionSignRequest{
		WalletHandleToken: "test-token",
		WalletPassword:    "test-password",
	}
	data1, err := json.Marshal(req1)
	require.NoError(t, err)

	var decoded1 APIV1POSTTransactionSignRequest
	err = json.Unmarshal(data1, &decoded1)
	require.NoError(t, err)
	require.Nil(t, decoded1.AccountIndex)

	// Test with AccountIndex set
	accountIdx := uint32(3)
	req2 := APIV1POSTTransactionSignRequest{
		WalletHandleToken: "test-token-2",
		WalletPassword:    "test-password",
		AccountIndex:      &accountIdx,
	}
	data2, err := json.Marshal(req2)
	require.NoError(t, err)

	var decoded2 APIV1POSTTransactionSignRequest
	err = json.Unmarshal(data2, &decoded2)
	require.NoError(t, err)
	require.NotNil(t, decoded2.AccountIndex)
	require.Equal(t, uint32(3), *decoded2.AccountIndex)
}

// TestMultisigTransactionSignRequestAccountIndex verifies that the AccountIndex field
// is properly serialized/deserialized in JSON
func TestMultisigTransactionSignRequestAccountIndex(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Test with nil AccountIndex (standard behavior)
	req1 := APIV1POSTMultisigTransactionSignRequest{
		WalletHandleToken: "test-token",
		WalletPassword:    "test-password",
	}
	data1, err := json.Marshal(req1)
	require.NoError(t, err)

	var decoded1 APIV1POSTMultisigTransactionSignRequest
	err = json.Unmarshal(data1, &decoded1)
	require.NoError(t, err)
	require.Nil(t, decoded1.AccountIndex)

	// Test with AccountIndex set
	accountIdx := uint32(10)
	req2 := APIV1POSTMultisigTransactionSignRequest{
		WalletHandleToken: "test-token-2",
		WalletPassword:    "test-password",
		AccountIndex:      &accountIdx,
	}
	data2, err := json.Marshal(req2)
	require.NoError(t, err)

	var decoded2 APIV1POSTMultisigTransactionSignRequest
	err = json.Unmarshal(data2, &decoded2)
	require.NoError(t, err)
	require.NotNil(t, decoded2.AccountIndex)
	require.Equal(t, uint32(10), *decoded2.AccountIndex)
}

// TestAPIV1WalletSupportsMultiAccount verifies that the SupportsMultiAccount field
// is properly serialized/deserialized in JSON
func TestAPIV1WalletSupportsMultiAccount(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Test with SupportsMultiAccount = false (default)
	wallet1 := APIV1Wallet{
		ID:   "wallet-1",
		Name: "Test Wallet 1",
	}
	data1, err := json.Marshal(wallet1)
	require.NoError(t, err)

	var decoded1 APIV1Wallet
	err = json.Unmarshal(data1, &decoded1)
	require.NoError(t, err)
	require.False(t, decoded1.SupportsMultiAccount)

	// Test with SupportsMultiAccount = true
	wallet2 := APIV1Wallet{
		ID:                   "wallet-2",
		Name:                 "Ledger Wallet",
		SupportsMultiAccount: true,
	}
	data2, err := json.Marshal(wallet2)
	require.NoError(t, err)

	var decoded2 APIV1Wallet
	err = json.Unmarshal(data2, &decoded2)
	require.NoError(t, err)
	require.True(t, decoded2.SupportsMultiAccount)
}
