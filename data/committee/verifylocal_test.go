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

package committee

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// TestVerifyLocalMatchesVerify checks VerifyLocal against Verify on many
// selectors and stakes, including accounts that are and are not selected.
func TestVerifyLocalMatchesVerify(t *testing.T) {
	partitiontest.PartitionTest(t)
	proto := config.Consensus[protocol.ConsensusCurrentVersion]
	selected := 0
	for i := 0; i < 2000; i++ {
		var seed [32]byte
		crypto.RandBytes(seed[:])
		pk, sk := crypto.VrfKeygenFromSeed(seed)
		var addr basics.Address
		crypto.RandBytes(addr[:])
		total := basics.MicroAlgos{Raw: 1e16}
		m := Membership{
			Record:     BalanceRecord{OnlineAccountData: basics.OnlineAccountData{MicroAlgosWithRewards: basics.MicroAlgos{Raw: uint64(i+1) * 1e12}, VotingData: basics.VotingData{SelectionID: pk}}, Addr: addr},
			Selector:   AgreementSelector{Round: basics.Round(i), Step: 0}, // propose step
			TotalMoney: total,
		}
		cred := MakeCredential(&sk, m.Selector)
		want, wantErr := cred.Verify(proto, m)
		got, gotErr := cred.VerifyLocal(proto, m)
		require.Equal(t, wantErr == nil, gotErr == nil, "i=%d", i)
		require.Equal(t, want, got, "i=%d", i)
		if wantErr == nil {
			selected++
		}
	}
	t.Logf("%d of 2000 selected", selected)
}
