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

// Directive provides additional constraints and effects for transactions.
type Directive uint8

// FeeSponsored enforces the transaction fee to be paid by a sponsor and not
// the sender. A transaction with this constraint is considered invalid until
// a sponsor signature is attached.
const FeeSponsored Directive = 1

// AssetSponsor allows for an Asset's OptIn and Minimum Balance Requirement to
// be allocated to the transaction Sender.
const AssetSponsor Directive = 2

// AssetRevoke will remove an existing Asset sponsorship from the
// AssetReceiver, if the sender is the Sponsor and the AssetReceiver's holdings
// are zero. If the AssetReceiver holds a non-zero amount, then the Sponsor can
// only be removed by the sponsored party Opting In or Closing Out of the asset
// themselves.
const AssetRevoke Directive = 3
