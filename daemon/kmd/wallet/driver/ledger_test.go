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

package driver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/daemon/kmd/wallet"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// TestLedgerImplementsMultiAccountWallet verifies that the Ledger driver
// implements the MultiAccountWallet interface
func TestLedgerImplementsMultiAccountWallet(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Create a ledger wallet (without actual hardware connection)
	// We just need to verify the interface is satisfied
	var w wallet.Wallet = &LedgerWallet{}

	// Verify it implements MultiAccountWallet
	_, ok := w.(wallet.MultiAccountWallet)
	require.True(t, ok, "LedgerWallet should implement MultiAccountWallet interface")
}

// TestAccountIDToBytes tests the accountIDToBytes helper function
func TestAccountIDToBytes(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	testCases := []struct {
		name     string
		input    uint32
		expected []byte
	}{
		{
			name:     "zero",
			input:    0,
			expected: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "one",
			input:    1,
			expected: []byte{0x00, 0x00, 0x00, 0x01},
		},
		{
			name:     "256",
			input:    256,
			expected: []byte{0x00, 0x00, 0x01, 0x00},
		},
		{
			name:     "max uint32",
			input:    0xFFFFFFFF,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "typical account index",
			input:    5,
			expected: []byte{0x00, 0x00, 0x00, 0x05},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := accountIDToBytes(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestLedgerWalletMetadataSupportsMultiAccount verifies that the Ledger wallet
// reports SupportsMultiAccount as true
func TestLedgerWalletMetadataSupportsMultiAccount(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	lw := &LedgerWallet{}
	meta, err := lw.Metadata()
	require.NoError(t, err)
	require.True(t, meta.SupportsMultiAccount, "Ledger wallet should report SupportsMultiAccount as true")
}
