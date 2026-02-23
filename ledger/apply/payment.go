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

package apply

import (
	"fmt"

	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
)

// Payment changes the balances according to this transaction.
// The ApplyData argument should reflect the changes made by
// apply().  It may already include changes made by the caller
// (i.e., Transaction.Apply), so apply() must update it rather
// than overwriting it.  For example, Transaction.Apply() may
// have updated ad.SenderRewards, and this function should only
// add to ad.SenderRewards (if needed), but not overwrite it.
func Payment(payment transactions.PaymentTxnFields, header transactions.Header, balances Balances, spec transactions.SpecialAddresses, ad *transactions.ApplyData) error {
	// move tx money
	if !payment.Amount.IsZero() || payment.Receiver != (basics.Address{}) {
		err := balances.Move(header.Sender, payment.Receiver, payment.Amount, &ad.SenderRewards, &ad.ReceiverRewards)
		if err != nil {
			return err
		}
	}

	// Bootstrap a new account
	if payment.AccountBootstrap == transactions.BootstrapAccount {
		rcvRecord, err := balances.Get(payment.Receiver, false)
		if err != nil {
			return err
		}
		// if !rcvRecord.Bootstrapper.IsZero() {
		// 	return fmt.Errorf("cannot bootstrap account: account is already bootstrapped by %s", rcvRecord.Bootstrapper.String())
		// }
		if !rcvRecord.IsZero() {
			return fmt.Errorf("cannot bootstrap account: account already exists")
		}
		rcvRecord.Bootstrapper = header.Sender
		err = balances.Put(payment.Receiver, rcvRecord)
		if err != nil {
			return err
		}

		sndRecord, err := balances.Get(header.Sender, false)
		if err != nil {
			return err
		}
		sndRecord.TotalAccountsBootstrapping = basics.AddSaturate(sndRecord.TotalAccountsBootstrapping, 1)
		err = balances.Put(header.Sender, sndRecord)
		if err != nil {
			return err
		}
	}

	// Rescind a bootstrapped account
	if payment.AccountBootstrap == transactions.RescindAccount {
		rcvRecord, err := balances.Get(payment.Receiver, false)
		if err != nil {
			return err
		}
		// if !rcvRecord.MicroAlgos.IsZero() {
		// 	return fmt.Errorf("balance %d still not zero after CloseRemainderTo", rcvRecord.MicroAlgos.Raw)
		// }
		proto := balances.ConsensusParams()
		minBal := rcvRecord.MinBalance(&proto).Raw
		if minBal > 0 {
			return fmt.Errorf("cannot rescind account bootstrap: account has non-zero minimum balance requirement %d", minBal)
		}

		if rcvRecord.TotalAssetsDelegated > 0 {
			return fmt.Errorf("cannot rescind account bootstrap: %d outstanding delegated assets", rcvRecord.TotalAssetsDelegated)
		}

		// Clear out entire bootstrapped account record
		err = balances.CloseAccount(payment.Receiver)
		if err != nil {
			return err
		}

		sndRecord, err := balances.Get(header.Sender, false)
		if err != nil {
			return err
		}
		sndRecord.TotalAccountsBootstrapping = basics.SubSaturate(sndRecord.TotalAccountsBootstrapping, 1)
		err = balances.Put(header.Sender, sndRecord)
		if err != nil {
			return err
		}
	}

	if payment.CloseRemainderTo != (basics.Address{}) {
		rec, err := balances.Get(header.Sender, true)
		if err != nil {
			return err
		}

		closeAmount := rec.MicroAlgos
		ad.ClosingAmount = closeAmount
		err = balances.Move(header.Sender, payment.CloseRemainderTo, closeAmount, &ad.SenderRewards, &ad.CloseRewards)
		if err != nil {
			return err
		}

		// Confirm that we have no balance left
		rec, err = balances.Get(header.Sender, true)
		if err != nil {
			return err
		}
		if !rec.MicroAlgos.IsZero() {
			return fmt.Errorf("balance %d still not zero after CloseRemainderTo", rec.MicroAlgos.Raw)
		}

		totalAccountsBootstrapping := rec.TotalAccountsBootstrapping
		if totalAccountsBootstrapping > 0 {
			if totalAccountsBootstrapping > 1 {
				return fmt.Errorf("cannot close: %d outstanding bootstrapped accounts", totalAccountsBootstrapping)
			}

			if ad.ClosingAmount.Raw < balances.ConsensusParams().MinBalance {
				return fmt.Errorf("cannot close: insufficient balance (%d) to remove account bootstrap", ad.ClosingAmount.Raw)
			}

			closeRecord, err2 := balances.Get(payment.CloseRemainderTo, false)
			if err2 != nil {
				return err2
			}
			if header.Sender != closeRecord.Bootstrapper {
				return fmt.Errorf("cannot close: account is bootstrapping another account")
			}
			closeRecord.Bootstrapper = basics.Address{}
			err2 = balances.Put(payment.CloseRemainderTo, closeRecord)
			if err2 != nil {
				return err2
			}
		}

		// Confirm that there are no delegating asset holdings by the account.
		if rec.TotalAssetsDelegating != 0 {
			return fmt.Errorf("cannot close: %d outstanding delegating assets", rec.TotalAssetsDelegating)
		}
		// Confirm that there are no delegated asset holdings by the account.
		if rec.TotalAssetsDelegated != 0 {
			return fmt.Errorf("cannot close: %d outstanding delegated assets", rec.TotalAssetsDelegated)
		}

		// Confirm that there is no asset-related state in the account
		totalAssets := rec.TotalAssets
		if totalAssets > 0 {
			return fmt.Errorf("cannot close: %d outstanding assets", totalAssets)
		}

		totalAssetParams := rec.TotalAssetParams
		if totalAssetParams > 0 {
			// This should be impossible because every asset created
			// by an account (in AssetParams) must also appear in Assets,
			// which we checked above.
			return fmt.Errorf("cannot close: %d outstanding created assets", totalAssetParams)
		}

		// Confirm that there is no application-related state remaining
		totalAppLocalStates := rec.TotalAppLocalStates
		if totalAppLocalStates > 0 {
			return fmt.Errorf("cannot close: %d outstanding applications opted in. Please opt out or clear them", totalAppLocalStates)
		}

		// Confirm that there is no box-related state in the account
		if rec.TotalBoxes > 0 {
			return fmt.Errorf("cannot close: %d outstanding boxes", rec.TotalBoxes)
		}
		if rec.TotalBoxBytes > 0 {
			// This should be impossible because every box byte comes from the existence of a box.
			return fmt.Errorf("cannot close: %d outstanding box bytes", rec.TotalBoxBytes)
		}

		// Can't have created apps remaining either
		totalAppParams := rec.TotalAppParams
		if totalAppParams > 0 {
			return fmt.Errorf("cannot close: %d outstanding created applications", totalAppParams)
		}

		// Clear out entire account record, to allow the DB to GC it
		err = balances.CloseAccount(header.Sender)
		if err != nil {
			return err
		}
	}

	return nil
}
