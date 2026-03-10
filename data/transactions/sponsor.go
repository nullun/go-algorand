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

package transactions

import (
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
)

// SponsorSig contains a signature that sponsors a transaction fee payment.
type SponsorSig struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	Sponsor basics.Address `codec:"addr"`
	SignatureFields
}

// Blank returns true if there is no content in this SponsorSig.
func (ssig *SponsorSig) Blank() bool {
	return ssig.Sponsor.IsZero() && ssig.Sig.Blank() && ssig.Msig.Blank() && ssig.Lsig.Blank() && ssig.AuthAddr.IsZero()
}

// Equal returns true if two SponsorSig are equal, including Address and AuthAddr.
func (ssig *SponsorSig) Equal(b *SponsorSig) bool {
	return ssig.Sponsor == b.Sponsor && ssig.Sig == b.Sig && ssig.Msig.Equal(b.Msig) && ssig.Lsig.Equal(&b.Lsig) && ssig.AuthAddr == b.AuthAddr
}

// SponsoredTransaction wraps a transaction with the sponsor address for domain-separated signing.
// This ensures sponsor signatures commit to the sponsor address, preventing replay attacks
// where a signature could be moved to a different sponsor account.
type SponsoredTransaction struct {
	_struct struct{}       `codec:",omitempty,omitemptyarray"`
	Txn     Transaction    `codec:"txn"`
	Sponsor basics.Address `codec:"sponsor"`
}

// ToBeHashed implements the crypto.Hashable interface for SponsoredTransaction.
func (st SponsoredTransaction) ToBeHashed() (protocol.HashID, []byte) {
	return protocol.FeeSponsor, protocol.Encode(&st)
}
