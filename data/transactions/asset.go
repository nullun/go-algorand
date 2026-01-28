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
	"errors"
	"fmt"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
)

// AssetSponsorship indicates the type of sponsorship operation.
type AssetSponsorship uint8

// ApproveAssetSponsorship indicates that the AssetReceiver's asset holdings
// will be sponsored by the Sender. Placing the asset holdings minimum balance
// requirement on the Sender.
const ApproveAssetSponsorship AssetSponsorship = 1

// RevokeAssetSponsorship indicates that the AssetReceiver's asset holdings
// will no longer be sponsored by the Sender (who must be the current Sponsor).
// This will only succeed if the AssetReceiver's asset holdings are zero.
// TODO: Should it be possible for someone else takeover an existing Asset
// Sponsorship? How would you prevent someone immediately taking over and
// revoking someone who temporarily has zero units but may intend to hold more
// again soon?
const RevokeAssetSponsorship AssetSponsorship = 2

// AssetConfigTxnFields captures the fields used for asset
// allocation, re-configuration, and destruction.
type AssetConfigTxnFields struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	// ConfigAsset is the asset being configured or destroyed.
	// A zero value means allocation
	ConfigAsset basics.AssetIndex `codec:"caid"`

	// AssetParams are the parameters for the asset being
	// created or re-configured.  A zero value means destruction.
	AssetParams basics.AssetParams `codec:"apar"`
}

// AssetTransferTxnFields captures the fields used for asset transfers.
type AssetTransferTxnFields struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	XferAsset basics.AssetIndex `codec:"xaid"`

	// AssetAmount is the amount of asset to transfer.
	// A zero amount transferred to self allocates that asset
	// in the account's Assets map.
	AssetAmount uint64 `codec:"aamt"`

	// AssetSender is the sender of the transfer.  If this is not
	// a zero value, the real transaction sender must be the Clawback
	// address from the AssetParams.  If this is the zero value,
	// the asset is sent from the transaction's Sender.
	AssetSender basics.Address `codec:"asnd"`

	// AssetReceiver is the recipient of the transfer.
	AssetReceiver basics.Address `codec:"arcv"`

	// AssetCloseTo indicates that the asset should be removed
	// from the account's Assets map, and specifies where the remaining
	// asset holdings should be transferred.  It's always valid to transfer
	// remaining asset holdings to the creator account.
	AssetCloseTo basics.Address `codec:"aclose"`

	// AssetSponsorship indicates the type of sponsorship operation.
	AssetSponsorship AssetSponsorship `codec:"aspsr"`
}

// AssetFreezeTxnFields captures the fields used for freezing asset slots.
type AssetFreezeTxnFields struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	// FreezeAccount is the address of the account whose asset
	// slot is being frozen or un-frozen.
	FreezeAccount basics.Address `codec:"fadd"`

	// FreezeAsset is the asset ID being frozen or un-frozen.
	FreezeAsset basics.AssetIndex `codec:"faid"`

	// AssetFrozen is the new frozen value.
	AssetFrozen bool `codec:"afrz"`
}

func (ac AssetConfigTxnFields) wellFormed(proto config.ConsensusParams) error {
	if len(ac.AssetParams.AssetName) > proto.MaxAssetNameBytes {
		return fmt.Errorf("transaction asset name too big: %d > %d", len(ac.AssetParams.AssetName), proto.MaxAssetNameBytes)
	}

	if len(ac.AssetParams.UnitName) > proto.MaxAssetUnitNameBytes {
		return fmt.Errorf("transaction asset unit name too big: %d > %d", len(ac.AssetParams.UnitName), proto.MaxAssetUnitNameBytes)
	}

	if len(ac.AssetParams.URL) > proto.MaxAssetURLBytes {
		return fmt.Errorf("transaction asset url too big: %d > %d", len(ac.AssetParams.URL), proto.MaxAssetURLBytes)
	}

	if ac.AssetParams.Decimals > proto.MaxAssetDecimals {
		return fmt.Errorf("transaction asset decimals is too high (max is %d)", proto.MaxAssetDecimals)
	}

	return nil
}

func (ax AssetTransferTxnFields) wellFormed(proto config.ConsensusParams) error {
	if ax.XferAsset == 0 && ax.AssetAmount != 0 {
		return errors.New("asset ID cannot be zero")
	}

	if !ax.AssetSender.IsZero() && !ax.AssetCloseTo.IsZero() {
		return errors.New("cannot close asset by clawback")
	}

	if !proto.SupportAssetSponsorship && ax.AssetSponsorship != 0 {
		return errors.New("transaction tries to set asset sponsorship, but asset sponsorship is not supported")
	}

	return nil
}

func (af AssetFreezeTxnFields) wellFormed() error {
	if af.FreezeAsset == 0 {
		return errors.New("asset ID cannot be zero")
	}

	if af.FreezeAccount.IsZero() {
		return errors.New("freeze account cannot be empty")
	}

	return nil
}
