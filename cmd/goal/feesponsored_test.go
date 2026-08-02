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

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/test/partitiontest"
)

// feeSponsoredCommands returns every command offering --fee-sponsored, split by
// whether the command can honour it.
func feeSponsoredCommands(t *testing.T) (supported, rejected []*cobra.Command) {
	t.Helper()

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if f := c.Flags().Lookup("fee-sponsored"); f != nil {
			if _, no := c.Annotations[noFeeSponsorship]; no {
				rejected = append(rejected, c)
			} else {
				supported = append(supported, c)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	return
}

// TestFeeSponsoredFlagIsGuarded pins the two ways --fee-sponsored can be misused:
// asking for it on a command that cannot apply it, and asking for it without -o,
// which would broadcast a transaction that no sponsor has signed.
func TestFeeSponsoredFlagIsGuarded(t *testing.T) {
	partitiontest.PartitionTest(t)

	supported, rejected := feeSponsoredCommands(t)
	require.NotEmpty(t, supported, "no command offers --fee-sponsored")

	defer func(sponsored bool, out string) {
		feeSponsored, outFilename = sponsored, out
	}(feeSponsored, outFilename)

	for _, cmd := range supported {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			require.NotNil(t, cmd.PreRunE, "%s has no --fee-sponsored guard", cmd.CommandPath())

			feeSponsored, outFilename = false, ""
			require.NoError(t, cmd.PreRunE(cmd, nil), "unsponsored use must not be blocked")

			feeSponsored, outFilename = true, ""
			require.ErrorContains(t, cmd.PreRunE(cmd, nil), "requires -o",
				"%s would broadcast a transaction with no sponsor signature", cmd.CommandPath())

			feeSponsored, outFilename = true, "out.txn"
			require.NoError(t, cmd.PreRunE(cmd, nil))
		})
	}

	for _, cmd := range rejected {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			require.True(t, cmd.Flags().Lookup("fee-sponsored").Hidden,
				"%s rejects --fee-sponsored but still advertises it", cmd.CommandPath())

			feeSponsored, outFilename = true, "out.txn"
			require.ErrorContains(t, cmd.PreRunE(cmd, nil), "not supported")
		})
	}
}
