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
	"fmt"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
)

// AccountBootstrap indicates the type of account bootstrap operation.
type AccountBootstrap uint8

// BootstrapAccount indicates that the Receiver's initial minimum
// balance requirement will be bootstrapped by the Sender.
const BootstrapAccount AccountBootstrap = 1

// RescindAccount indicates that the Receiver's initial minimum
// balance requirement will no longer be bootstrapped by the Sender (who must be
// the current Bootstrapper).
// This will only succeed if the Receiver's holdings are zero.
// TODO: Should it be possible for someone else takeover an existing Account
// Bootstrap? How would you prevent someone immediately taking over and
// rescinding someone who temporarily has zero units but may intend to hold more
// again soon?
const RescindAccount AccountBootstrap = 2

// PaymentTxnFields captures the fields used by payment transactions.
type PaymentTxnFields struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	Receiver basics.Address    `codec:"rcv"`
	Amount   basics.MicroAlgos `codec:"amt"`

	// When CloseRemainderTo is set, it indicates that the
	// transaction is requesting that the account should be
	// closed, and all remaining funds be transferred to this
	// address.
	CloseRemainderTo basics.Address `codec:"close"`

	// AccountBootstrap indicates the type of account bootstrap operation.
	AccountBootstrap AccountBootstrap `codec:"boot"`
}

// wellFormed performs some stateless checks on the Sender of a pay transaction
func (payment PaymentTxnFields) wellFormed(header Header, spec SpecialAddresses, proto config.ConsensusParams) error {
	if header.Sender == payment.CloseRemainderTo {
		return fmt.Errorf("transaction cannot close account to its sender %v", header.Sender)
	}

	// the FeeSink account may only spend to the IncentivePool (not at all, if Payouts.Enabled)
	if header.Sender == spec.FeeSink {
		if proto.Payouts.Enabled {
			return fmt.Errorf("cannot spend from fee sink address %v", header.Sender)
		}
		if payment.Receiver != spec.RewardsPool {
			return fmt.Errorf("cannot spend from fee sink's address %v to non incentive pool address %v", header.Sender, payment.Receiver)
		}
		if !payment.CloseRemainderTo.IsZero() {
			return fmt.Errorf("cannot close fee sink %v to %v", header.Sender, payment.CloseRemainderTo)
		}
	}
	return nil
}
