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
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
)

// SponsorSig contains a signature that sponsors a transaction fee payment.
type SponsorSig struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	Sig      crypto.Signature   `codec:"sig"`
	Msig     crypto.MultisigSig `codec:"msig"`
	Lsig     LogicSig           `codec:"lsig"`
	AuthAddr basics.Address     `codec:"sgnr"`
}

// Blank returns true if there is no content in this LogicSig
func (ssig *SponsorSig) Blank() bool {
	return ssig.Sig.Blank() && ssig.Msig.Blank() && ssig.Lsig.Blank()
}

// Equal returns true if two SponsorSig are equal
func (ssig *SponsorSig) Equal(b *SponsorSig) bool {
	return ssig.Sig == b.Sig && ssig.Msig.Equal(b.Msig) && ssig.Lsig.Equal(&b.Lsig)
}
