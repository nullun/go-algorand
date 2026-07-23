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
	"time"

	"github.com/algorand/go-algorand/protocol"
)

// initAlphanetProtocols defines the vAlphaX versions, a separate series of
// consensus parameters and versions for AlphaNet.
func initAlphanetProtocols() {
	vAlpha1 := consensusFrom(protocol.ConsensusV32)
	vAlpha1.AgreementFilterTimeoutPeriod0 = 2 * time.Second
	vAlpha1.MaxTxnBytesPerBlock = 5000000
	Consensus[protocol.ConsensusVAlpha1] = vAlpha1

	vAlpha2 := vAlpha1.branch()
	vAlpha2.AgreementFilterTimeoutPeriod0 = 3500 * time.Millisecond
	vAlpha2.MaxTxnBytesPerBlock = 5 * 1024 * 1024
	Consensus[protocol.ConsensusVAlpha2] = vAlpha2
	vAlpha1.ApprovedUpgrades[protocol.ConsensusVAlpha2] = 10000

	// vAlpha3 and vAlpha4 use the same parameters as v33 and v34
	vAlpha3 := consensusFrom(protocol.ConsensusV33)
	Consensus[protocol.ConsensusVAlpha3] = vAlpha3
	vAlpha2.ApprovedUpgrades[protocol.ConsensusVAlpha3] = 10000

	vAlpha4 := consensusFrom(protocol.ConsensusV34)
	Consensus[protocol.ConsensusVAlpha4] = vAlpha4
	vAlpha3.ApprovedUpgrades[protocol.ConsensusVAlpha4] = 10000

	// vAlpha5 uses the same parameters as v36
	vAlpha5 := consensusFrom(protocol.ConsensusV36)
	Consensus[protocol.ConsensusVAlpha5] = vAlpha5
	vAlpha4.ApprovedUpgrades[protocol.ConsensusVAlpha5] = 10000
}
