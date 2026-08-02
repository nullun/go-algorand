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

// outputFlagNames are the flags goal commands use to write a transaction to a
// file. A command offering --fee-sponsored must have exactly one of them, since
// a sponsored transaction can only leave goal as a file.
var outputFlagNames = []string{"out", "txfile"}

// feeSponsoredCommands returns every command offering --fee-sponsored, split by
// whether the command can honour it.
func feeSponsoredCommands() (supported, rejected []*cobra.Command) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Flags().Lookup("fee-sponsored") != nil {
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

// outputFlagOf returns the single file-output flag a command uses.
func outputFlagOf(t *testing.T, cmd *cobra.Command) string {
	t.Helper()

	var found []string
	for _, name := range outputFlagNames {
		if cmd.Flags().Lookup(name) != nil {
			found = append(found, name)
		}
	}
	require.Lenf(t, found, 1, "%s offers --fee-sponsored but has %v file-output flags, expected exactly one of %v",
		cmd.CommandPath(), found, outputFlagNames)
	return found[0]
}

// TestFeeSponsoredFlagIsGuarded pins the two ways --fee-sponsored can be misused:
// asking for it on a command that cannot apply it, and asking for it without a
// file to write to, which would broadcast a transaction no sponsor has signed.
// The guard has to name whichever output flag the command actually uses, which
// is --txfile for the keyreg commands and --out for the rest.
func TestFeeSponsoredFlagIsGuarded(t *testing.T) {
	partitiontest.PartitionTest(t)

	supported, rejected := feeSponsoredCommands()
	require.NotEmpty(t, supported, "no command offers --fee-sponsored")

	defer func(sponsored bool) { feeSponsored = sponsored }(feeSponsored)

	for _, cmd := range supported {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			require.NotNil(t, cmd.PreRunE, "%s has no --fee-sponsored guard", cmd.CommandPath())
			out := outputFlagOf(t, cmd)
			defer func() {
				feeSponsored = false
				require.NoError(t, cmd.Flags().Set(out, ""))
			}()

			feeSponsored = false
			require.NoError(t, cmd.Flags().Set(out, ""))
			require.NoError(t, cmd.PreRunE(cmd, nil), "unsponsored use must not be blocked")

			feeSponsored = true
			err := cmd.PreRunE(cmd, nil)
			require.ErrorContains(t, err, "--"+out,
				"%s must point at its own output flag", cmd.CommandPath())
			require.ErrorContains(t, err, "requires",
				"%s would broadcast a transaction with no sponsor signature", cmd.CommandPath())

			require.NoError(t, cmd.Flags().Set(out, "txn.out"))
			require.NoError(t, cmd.PreRunE(cmd, nil))
		})
	}

	for _, cmd := range rejected {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			require.True(t, cmd.Flags().Lookup("fee-sponsored").Hidden,
				"%s rejects --fee-sponsored but still advertises it", cmd.CommandPath())
			defer func() { feeSponsored = false }()

			feeSponsored = true
			require.ErrorContains(t, cmd.PreRunE(cmd, nil), "not supported")
		})
	}
}

// TestKeyregCommandsOfferFeeSponsorship pins that the keyreg commands able to
// write a transaction to a file offer sponsorship. Going online with incentive
// eligibility costs Payouts.GoOnlineFee, the fee most likely to be sponsored.
func TestKeyregCommandsOfferFeeSponsorship(t *testing.T) {
	partitiontest.PartitionTest(t)

	supported, _ := feeSponsoredCommands()
	paths := make(map[string]bool, len(supported))
	for _, cmd := range supported {
		paths[cmd.CommandPath()] = true
	}

	for _, path := range []string{"goal account changeonlinestatus", "goal account marknonparticipating"} {
		require.Truef(t, paths[path], "%s no longer offers --fee-sponsored", path)
	}
}
