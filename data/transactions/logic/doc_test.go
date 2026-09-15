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

package logic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestOpDocs(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	opsSeen := make(map[string]bool, len(OpSpecs))
	for _, op := range OpSpecs {
		opsSeen[op.Name] = false
	}
	for name := range opDescByName {
		if _, ok := opsSeen[name]; !ok { // avoid assert.Contains: printing opsSeen is waste
			assert.Fail(t, "opDescByName contains strange opcode", "%#v", name)
		}
		opsSeen[name] = true
	}
	for op, seen := range opsSeen {
		assert.True(t, seen, "opDescByName is missing description for %#v", op)
	}

	require.Len(t, OnCompletionDescriptions, len(OnCompletionNames))
	require.Len(t, TypeNameDescriptions, len(TxnTypeNames))
}

func TestOpGroupCoverage(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	opsSeen := make(map[string]int, len(OpSpecs))
	for _, op := range OpSpecs {
		opsSeen[op.Name] = 0
	}
	for _, names := range OpGroups {
		for _, name := range names {
			_, exists := opsSeen[name]
			if !exists {
				t.Errorf("op %#v in group list but not in OpSpecs\n", name)
				continue
			}
			opsSeen[name]++
		}
	}
	for name, seen := range opsSeen {
		if seen == 0 {
			t.Errorf("op %#v not in any group of OpGroups\n", name)
		}
		if seen > 1 {
			t.Errorf("op %#v in %d groups of OpGroups\n", name, seen)
		}
	}
}

func TestOpDoc(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	xd := OpDescOf("txn")
	require.NotEmpty(t, xd)
	xd = OpDescOf("NOT AN INSTRUCTION")
	require.Empty(t, xd)
}

func TestOpImmediateDetails(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	for _, os := range OpSpecs {
		deets := OpImmediateDetailsFromSpec(os)
		require.Equal(t, len(os.Immediates), len(deets))

		for idx, d := range deets {
			imm := os.Immediates[idx]
			require.NotEmpty(t, d.Comment)
			require.Equal(t, strings.ToLower(d.Name), imm.Name)
			require.Equal(t, d.Encoding, imm.kind.String())

			if imm.Group != nil {
				require.Equal(t, d.Reference, imm.Group.Heading())
			}
		}
	}
}

func TestOpDocExtra(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	require.NotEmpty(t, OpDocExtra("bnz", 1))
	require.Empty(t, OpDocExtra("-", LogicVersion))

	// every version of bnz explains its encoding, but only its own encoding
	for v := uint64(1); v <= 12; v++ {
		require.Contains(t, OpDocExtra("bnz", v), "16 bit offset")
		require.NotContains(t, OpDocExtra("bnz", v), "Varint")
	}
	require.Contains(t, OpDocExtra("bnz", 13), "Varint")
	require.NotContains(t, OpDocExtra("bnz", 13), "16 bit offset")

	// backward branches arrived at v4
	require.Contains(t, OpDocExtra("bnz", 3), "forward branches only")
	require.NotContains(t, OpDocExtra("bnz", 3), "signed")
	require.Contains(t, OpDocExtra("bnz", 4), "signed")
	require.NotContains(t, OpDocExtra("bnz", 4), "forward branches only")

	// branching to the end of the program became legal at v2
	require.Contains(t, OpDocExtra("bnz", 1), "is illegal")
	require.Contains(t, OpDocExtra("bnz", 2), "is allowed")

	// the state access opcodes accept direct references only from v4 on
	require.NotContains(t, OpDocExtra("balance", 3), "account address")
	require.Contains(t, OpDocExtra("balance", 4), "account address")
	require.NotContains(t, OpDocExtra("asset_holding_get", 3), "ForeignAssets")
	require.Contains(t, OpDocExtra("asset_holding_get", 4), "ForeignAssets")
	// and no version's docs defer to another version any more
	for v := uint64(2); v <= LogicVersion; v++ {
		require.NotContains(t, OpDocExtra("app_local_del", v), "since v4")
	}

	// effects fields: inner-only at v5, past top-level app calls from v6
	require.Contains(t, OpDocExtra("txn", 5), "only be read from inner")
	require.NotContains(t, OpDocExtra("txn", 5), "top-level application call may be read")
	require.Contains(t, OpDocExtra("txn", 6), "earlier in the group")
	require.Empty(t, OpDocExtra("txn", 4))
	require.Contains(t, OpDocExtra("itxn", 5), "GroupIndex")
	require.Empty(t, OpDocExtra("itxn", 6))

	// min_balance counts boxes only once boxes exist
	require.NotContains(t, OpDocExtra("min_balance", 7), "Box")
	require.Contains(t, OpDocExtra("min_balance", 8), "Box")
	require.NotContains(t, OpDocExtra("balance", 4), "inner")
	require.Contains(t, OpDocExtra("balance", 5), "itxn_submit")

	// versioned notes must name real opcodes, like opDescByName entries
	opsSeen := make(map[string]bool, len(OpSpecs))
	for _, op := range OpSpecs {
		opsSeen[op.Name] = true
	}
	for name := range opVersionedExtras {
		assert.True(t, opsSeen[name], "opVersionedExtras contains strange opcode %#v", name)
	}
}
