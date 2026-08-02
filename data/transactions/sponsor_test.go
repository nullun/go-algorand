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

package transactions

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// TestSignatureFieldsBlankCoversEveryField pins that Blank inspects every field
// of SignatureFields. A field omitted from Blank is a field that can be carried
// over the wire without ever being verified, since Blank is what decides whether
// there is a signature to check at all.
func TestSignatureFieldsBlankCoversEveryField(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	fixture := makePQSigTestFixture(t, 14)

	populate := map[string]func(*SignatureFields){
		"Sig":      func(s *SignatureFields) { s.Sig[0] = 1 },
		"Msig":     func(s *SignatureFields) { s.Msig = crypto.MultisigSig{Version: 1} },
		"Lsig":     func(s *SignatureFields) { s.Lsig = LogicSig{Logic: []byte{1}} },
		"PQsig":    func(s *SignatureFields) { s.PQsig = fixture.pqSig },
		"AuthAddr": func(s *SignatureFields) { s.AuthAddr = basics.Address{1} },
	}

	// Every exported field must be exercised, so a newly added signature type
	// cannot silently escape this test.
	typ := reflect.TypeFor[SignatureFields]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if name == "_struct" {
			continue
		}
		require.Containsf(t, populate, name, "SignatureFields.%s is not covered by this test", name)
	}

	require.True(t, (&SignatureFields{}).Blank())

	for name, set := range populate {
		t.Run(name, func(t *testing.T) {
			var s SignatureFields
			set(&s)
			require.Falsef(t, s.Blank(), "Blank ignores %s", name)

			var other SignatureFields
			require.Falsef(t, s.Equal(&other), "Equal ignores %s", name)
			require.True(t, s.Equal(&s))

			ssig := SponsorSig{SignatureFields: s}
			require.Falsef(t, ssig.Blank(), "SponsorSig.Blank ignores %s", name)
			require.Falsef(t, ssig.Equal(&SponsorSig{}), "SponsorSig.Equal ignores %s", name)
		})
	}

	// The sponsor address alone also counts as content.
	require.False(t, (&SponsorSig{Sponsor: basics.Address{1}}).Blank())
	require.True(t, (&SponsorSig{}).Blank())
}
