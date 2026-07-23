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

package config

import (
	"github.com/algorand/go-algorand/protocol"
)

// initFnetProtocols defines the vFnetX versions: the genesis and historical
// protocols for AF's FNet network. A mainline node needs them to open the
// FNet genesis ledger (proto fnet1) and to replay the pre-V40 chain. The
// on-chain upgrade path was fnet1->fnet2->fnet3->fnet4->V40.
func initFnetProtocols() {
	vFnet1 := consensusFrom(protocol.ConsensusV39)
	vFnet1.LogicSigVersion = 11 // When moving this to a release, put a new higher LogicSigVersion here
	vFnet1.Payouts.Enabled = true
	vFnet1.Payouts.Percent = 75
	vFnet1.Payouts.GoOnlineFee = 2_000_000         // 2 algos
	vFnet1.Payouts.MinBalance = 30_000_000_000     // 30,000 algos
	vFnet1.Payouts.MaxBalance = 70_000_000_000_000 // 70M algos
	vFnet1.Payouts.MaxMarkAbsent = 32
	vFnet1.Payouts.ChallengeInterval = 1000
	vFnet1.Payouts.ChallengeGracePeriod = 200
	vFnet1.Payouts.ChallengeBits = 5
	vFnet1.Bonus.BaseAmount = 10_000_000 // 10 Algos
	vFnet1.Bonus.DecayInterval = 250_000 // .99^(10.8/0.25) ~ .648. So 35% decay per year
	Consensus[protocol.ConsensusVFnet1] = vFnet1

	// vFnet2 guards against a future change in block opcodes that the fnet1 client did not support. No change in parameters
	vFnet2 := vFnet1.branch()
	Consensus[protocol.ConsensusVFnet2] = vFnet2
	vFnet1.ApprovedUpgrades[protocol.ConsensusVFnet2] = 10000

	// vFnet3 disabled challenges - without heartbeats, participating accounts are being evicted
	vFnet3 := vFnet2.branch()
	vFnet3.Payouts.ChallengeInterval = 0
	Consensus[protocol.ConsensusVFnet3] = vFnet3
	vFnet2.ApprovedUpgrades[protocol.ConsensusVFnet3] = 10000

	// vFnet4: challenges and heartbeats
	vFnet4 := vFnet1.branch()
	Consensus[protocol.ConsensusVFnet4] = vFnet4
	vFnet3.ApprovedUpgrades[protocol.ConsensusVFnet4] = 10000

	vFnet4.ApprovedUpgrades[protocol.ConsensusV40] = 10000
}
