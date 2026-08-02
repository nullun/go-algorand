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
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/algorand/go-algorand/data/basics"
)

const (
	stdoutFilenameValue = "-"
	stdinFileNameValue  = "-"
)

// validateNoPosArgsFn is a reusable cobra positional argument validation function
// for generating proper error messages when commands see unexpected arguments when they expect no args.
// We don't use cobra.NoArgs directly, in case we want to customize behavior later.
var validateNoPosArgsFn = cobra.NoArgs

// transaction validity period margins
var firstValid basics.Round
var lastValid basics.Round

// numValidRounds specifies validity period for a transaction and used to calculate last valid round
var numValidRounds basics.Round // also used in account and asset

var (
	fee             uint64
	outFilename     string
	sign            bool
	noteBase64      string
	noteText        string
	lease           string
	noWaitAfterSend bool
	rekeyToAddress  string
	feeSponsored    bool
)

func addTxnFlags(cmd *cobra.Command) {
	cmd.Flags().Uint64Var(&fee, "fee", 0, "The transaction fee (automatically determined by default), in microAlgos")
	cmd.Flags().Uint64Var((*uint64)(&firstValid), "firstvalid", 0, "The first round where the transaction may be committed to the ledger")
	cmd.Flags().Uint64Var((*uint64)(&numValidRounds), "validrounds", 0, "The number of rounds for which the transaction will be valid")
	cmd.Flags().Uint64Var((*uint64)(&lastValid), "lastvalid", 0, "The last round where the transaction may be committed to the ledger")
	cmd.Flags().StringVarP(&outFilename, "out", "o", "", "Write transaction to this file")
	cmd.Flags().BoolVarP(&sign, "sign", "s", false, "Use with -o to indicate that the dumped transaction should be signed")
	cmd.Flags().StringVar(&noteBase64, "noteb64", "", "Note (URL-base64 encoded)")
	cmd.Flags().StringVarP(&noteText, "note", "n", "", "Note text (ignored if --noteb64 used also)")
	cmd.Flags().StringVarP(&lease, "lease", "x", "", "Lease value (base64, optional): no transaction may also acquire this lease until lastvalid")
	cmd.Flags().BoolVarP(&noWaitAfterSend, "no-wait", "N", false, "Don't wait for transaction to commit")
	cmd.Flags().StringVarP(&signerAddress, "signer", "S", "", "Address of key to sign with, if different from transaction \"from\" address due to rekeying")
	cmd.Flags().StringVar(&rekeyToAddress, "rekey-to", "", "Rekey account to the given spending key/address. (Future transactions from this account will need to be signed with the new key.)")
	addFeeSponsoredFlag(cmd, "out")
}

// addFeeSponsoredFlag offers --fee-sponsored on a command that builds a single
// transaction. outputFlag names the flag that command uses to write the
// transaction to a file, which is not always "out".
func addFeeSponsoredFlag(cmd *cobra.Command, outputFlag string) {
	cmd.Flags().BoolVar(&feeSponsored, "fee-sponsored", false,
		fmt.Sprintf("Mark transaction as fee-sponsored (fee will be paid by a sponsor). Requires --%s, since the sponsor must sign the transaction with \"goal clerk sponsor\" before it can be submitted", outputFlag))

	// Attached here so the checks cover exactly the commands offering the flag.
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if !feeSponsored {
			return nil
		}
		if _, ok := cmd.Annotations[noFeeSponsorship]; ok {
			return errors.New("--fee-sponsored is not supported by " + cmd.CommandPath())
		}
		// A fee-sponsored transaction is incomplete until its sponsor signs it, so
		// it cannot be broadcast from here. Say so, rather than letting the node
		// reject the transaction.
		f := cmd.Flags().Lookup(outputFlag)
		if f == nil {
			return fmt.Errorf("--fee-sponsored is wired to --%s, which %s does not have", outputFlag, cmd.CommandPath())
		}
		if f.Value.String() == "" {
			name := "--" + outputFlag
			if f.Shorthand != "" {
				name += " (-" + f.Shorthand + ")"
			}
			return errors.New("--fee-sponsored requires " + name + ": the transaction must be signed by its sponsor with \"goal clerk sponsor\" before it can be submitted")
		}
		return nil
	}
}

// noFeeSponsorship marks a command that takes the shared transaction flags but
// cannot honour --fee-sponsored. Set it with markNoFeeSponsorship.
const noFeeSponsorship = "noFeeSponsorship"

// markNoFeeSponsorship hides --fee-sponsored on a command and makes using it an
// error. For commands that produce something other than one signed transaction,
// since a sponsor signature covers a single transaction.
func markNoFeeSponsorship(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[noFeeSponsorship] = "true"
	if err := cmd.Flags().MarkHidden("fee-sponsored"); err != nil {
		panic(err) // only fails if the flag is missing, which would be a bug here
	}
}

func parseRekey(rekeyToAddress string) basics.Address {
	if rekeyToAddress == "" {
		return basics.Address{}
	}
	rekeyTo, err := basics.UnmarshalChecksumAddress(rekeyToAddress)
	if err != nil {
		reportErrorln(err)
	}
	return rekeyTo
}
