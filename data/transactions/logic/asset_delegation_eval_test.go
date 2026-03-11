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

package logic

import (
	"fmt"
	"testing"

	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestAssetDelegationVisibility(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// Testing with versions around the introduction of SupportAssetDelegation (version 13)
	testLogicRange(t, 10, 13, func(t *testing.T, ep *EvalParams, tx *transactions.Transaction, ledger *Ledger) {
		test := func(source string) {
			t.Helper()
			testApp(t, source, ep)
		}

		addr := tx.Sender
		ledger.NewAccount(addr, 10_000_000)

		// Create 2 normal assets
		ledger.NewAsset(addr, 1001, basics.AssetParams{})
		ledger.NewAsset(addr, 1002, basics.AssetParams{})
		br := ledger.balances[addr]
		br.holdings[1001] = basics.AssetHolding{Amount: 100}
		br.holdings[1002] = basics.AssetHolding{Amount: 200}

		// Add 1 delegated asset (sponsored by someone else TO this account)
		sponsor := basics.Address{3: 1} // dummy address
		br.holdings[1003] = basics.AssetHolding{Amount: 300, Delegator: sponsor}
		br.delegated = 1

		// Add 1 asset being delegated BY this account (sponsoring someone else)
		br.delegating = 1

		// Add bootstrapping info
		bootstrapper := basics.Address{4: 1} // dummy address
		br.bootstrapper = bootstrapper
		br.bootstrapping = 5 // this account is bootstrapping 5 others

		ledger.balances[addr] = br

		// Make assets available to the program
		tx.ApplicationCallTxnFields.ForeignAssets = []basics.AssetIndex{1001, 1002, 1003}

		// Re-create ep to ensure resources are filled with the new ForeignAssets
		ep = defaultAppParamsWithVersion(ep.Proto.LogicSigVersion, transactions.SignedTxn{Txn: *tx})
		ep.Ledger = ledger
		ep.SigLedger = ledger
		ep.UnnamedResources = &mockUnnamedResourcePolicy{allowEverything: true}

		// Set delegation/bootstrap fields in the transaction itself to test txn fields
		tx = &ep.TxnGroup[0].Txn
		tx.Type = protocol.AssetTransferTx
		tx.AssetTransferTxnFields.AssetDelegation = transactions.ApproveAssetDelegation
		tx.PaymentTxnFields.AccountBootstrap = transactions.BootstrapAccount

		if ep.Proto.LogicSigVersion < 13 {
			// Version < 13: Should only see 2 assets in count
			test("txn Sender; acct_params_get AcctTotalAssets; assert; int 2; ==")

			// Should NOT see the delegated asset in asset_holding_get
			test("txn Sender; int 1003; asset_holding_get AssetBalance; swap; pop; int 0; ==")

			// Min balance should be calculated as if there are 2 assets and 0 delegated/delegating/bootstrapped
			// Base (1001) + 2 assets (1001 * 2) = 3003
			test("txn Sender; acct_params_get AcctMinBalance; assert; int 3003; ==")
			test("txn Sender; min_balance; int 3003; ==")

			// Programs < 13 should not even assemble the new fields
			// (testApp internally assembles, so we expect failure if we try to use them)
			// But for now, we've already verified the hiding logic for existing fields.
		} else {
			// Version >= 13: Should see all 3 assets in the holdings count
			test("txn Sender; acct_params_get AcctTotalAssets; assert; int 3; ==")

			// Should see the delegated asset in asset_holding_get
			test("txn Sender; int 1003; asset_holding_get AssetBalance; assert; int 300; ==")

			// Should see the delegator
			test(fmt.Sprintf("txn Sender; int 1003; asset_holding_get AssetDelegator; assert; byte 0x%x; ==", sponsor[:]))

			// Min balance should be calculated with delegation and bootstrapping
			// (3 assets - 1 delegated + 1 delegating) * 1001 = 3003 (assets portion)
			// + 5 accounts bootstrapping * 1001 = 5005
			// + base MBR (0 because we are bootstrapped) = 0
			// Total = 3003 + 5005 = 8008
			test("txn Sender; acct_params_get AcctMinBalance; assert; int 8008; ==")
			test("txn Sender; min_balance; int 8008; ==")

			// Test new account params
			test("txn Sender; acct_params_get AcctTotalAssetsDelegated; assert; int 1; ==")
			test("txn Sender; acct_params_get AcctTotalAssetsDelegating; assert; int 1; ==")
			test("txn Sender; acct_params_get AcctTotalAccountsBootstrapping; assert; int 5; ==")
			test(fmt.Sprintf("txn Sender; acct_params_get AcctBootstrapper; assert; byte 0x%x; ==", bootstrapper[:]))

			// Test new txn fields
			test("txn AssetDelegation; int 1; ==")
			test("txn AccountBootstrap; int 1; ==")
		}
	})
}
