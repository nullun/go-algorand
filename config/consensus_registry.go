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
	"fmt"
	"maps"
	"time"

	"github.com/algorand/go-algorand/config/bounds"
	"github.com/algorand/go-algorand/protocol"
)

// ConsensusProtocols defines a set of supported protocol versions and their
// corresponding parameters.
type ConsensusProtocols map[protocol.ConsensusVersion]ConsensusParams

// Consensus tracks the protocol-level settings for different versions of the
// consensus protocol.
var Consensus ConsensusProtocols

func checkSetMax(value int, curMax *int) {
	if value > *curMax {
		*curMax = value
	}
}

// checkSetAllocBounds sets some global variables used during msgpack decoding
// to enforce memory allocation limits. The values should be generous to
// prevent correctness bugs, but not so large that DoS attacks are trivial
func checkSetAllocBounds(p ConsensusParams) {
	checkSetMax(int(p.SoftCommitteeThreshold), &bounds.MaxVoteThreshold)
	checkSetMax(int(p.CertCommitteeThreshold), &bounds.MaxVoteThreshold)
	checkSetMax(int(p.NextCommitteeThreshold), &bounds.MaxVoteThreshold)
	checkSetMax(int(p.LateCommitteeThreshold), &bounds.MaxVoteThreshold)
	checkSetMax(int(p.RedoCommitteeThreshold), &bounds.MaxVoteThreshold)
	checkSetMax(int(p.DownCommitteeThreshold), &bounds.MaxVoteThreshold)

	// These bounds could be tighter, but since these values are just to
	// prevent DoS, setting them to be the maximum number of allowed
	// executed TEAL instructions should be fine (order of ~1000)
	checkSetMax(p.MaxAppProgramLen, &bounds.MaxStateDeltaKeys)
	checkSetMax(p.MaxAppProgramLen, &bounds.MaxEvalDeltaAccounts)
	checkSetMax(p.MaxAppProgramLen, &bounds.MaxAppProgramLen)
	checkSetMax((int(p.LogicSigMaxSize) * p.MaxTxGroupSize), &bounds.MaxLogicSigMaxSize)
	checkSetMax(int(p.MaxAbsoluteLogicSigProgramSize), &bounds.MaxLogicSigMaxSize)
	checkSetMax(p.MaxAbsoluteTxnNoteBytes, &bounds.MaxTxnNoteBytes)
	checkSetMax(p.MaxTxGroupSize, &bounds.MaxTxGroupSize)
	// MaxBytesKeyValueLen is max of MaxAppKeyLen and MaxAppBytesValueLen
	checkSetMax(p.MaxAppKeyLen, &bounds.MaxBytesKeyValueLen)
	checkSetMax(p.MaxAppBytesValueLen, &bounds.MaxBytesKeyValueLen)
	checkSetMax(p.MaxAbsoluteExtraProgramPages, &bounds.MaxExtraAppProgramLen)
	// MaxAvailableAppProgramLen is the max of supported app program size
	bounds.MaxAvailableAppProgramLen = bounds.MaxAppProgramLen * (1 + bounds.MaxExtraAppProgramLen)
	// There is no consensus parameter for MaxLogCalls and MaxAppProgramLen as an approximation
	// Its value is much larger than any possible reasonable MaxLogCalls value in future
	checkSetMax(p.MaxAppProgramLen, &bounds.MaxLogCalls)
	checkSetMax(p.MaxInnerTransactions*p.MaxTxGroupSize, &bounds.MaxInnerTransactionsPerDelta)
	checkSetMax(p.MaxProposedExpiredOnlineAccounts, &bounds.MaxProposedExpiredOnlineAccounts)
	checkSetMax(p.Payouts.MaxMarkAbsent, &bounds.MaxMarkAbsent)

	// These bounds are exported to make them available to the msgp generator for calculating
	// maximum valid message size for each message going across the wire.
	checkSetMax(p.MaxAbsoluteTotalArgLen, &bounds.MaxAppTotalArgLen)
	checkSetMax(p.MaxAssetNameBytes, &bounds.MaxAssetNameBytes)
	checkSetMax(p.MaxAssetUnitNameBytes, &bounds.MaxAssetUnitNameBytes)
	checkSetMax(p.MaxAssetURLBytes, &bounds.MaxAssetURLBytes)
	checkSetMax(p.MaxAppBytesValueLen, &bounds.MaxAppBytesValueLen)
	checkSetMax(p.MaxAppKeyLen, &bounds.MaxAppBytesKeyLen)
	checkSetMax(int(p.StateProofTopVoters), &bounds.StateProofTopVoters)
	checkSetMax(p.MaxTxnBytesPerBlock, &bounds.MaxTxnBytesPerBlock)

	checkSetMax(p.MaxAppTxnForeignApps, &bounds.MaxAppTxnForeignApps)
	checkSetMax(p.MaxAppAccess, &bounds.MaxAppAccess)
}

// DeepCopy creates a deep copy of a consensus protocols map.
func (cp ConsensusProtocols) DeepCopy() ConsensusProtocols {
	staticConsensus := make(ConsensusProtocols)
	for consensusVersion, consensusParams := range cp {
		// recreate the ApprovedUpgrades map since we don't want to modify the original one.
		consensusParams.ApprovedUpgrades = maps.Clone(consensusParams.ApprovedUpgrades)
		staticConsensus[consensusVersion] = consensusParams
	}
	return staticConsensus
}

// Merge merges a configurable consensus on top of the existing consensus protocol and return
// a new consensus protocol without modify any of the incoming structures.
func (cp ConsensusProtocols) Merge(configurableConsensus ConsensusProtocols) ConsensusProtocols {
	staticConsensus := cp.DeepCopy()

	for consensusVersion, consensusParams := range configurableConsensus {
		if consensusParams.ApprovedUpgrades == nil {
			// if we were provided with an empty ConsensusParams, delete the existing reference to this consensus version
			for cVer, cParam := range staticConsensus {
				if cVer == consensusVersion {
					delete(staticConsensus, cVer)
				} else {
					// delete upgrade to deleted version
					delete(cParam.ApprovedUpgrades, consensusVersion)
				}
			}
		} else {
			// need to add/update entry
			staticConsensus[consensusVersion] = consensusParams
		}
	}

	return staticConsensus
}

// The consensus protocols known to this binary are defined across several
// files, one per network's series of versions:
//
//	consensus_mainline.go - v7 onward plus vFuture: the series shared by
//	    MainNet, TestNet, and BetaNet. vFuture is the gated staging area
//	    for the next version in this series.
//	consensus_alphanet.go - vAlpha1..: AlphaNet's series
//	consensus_fnet.go     - vFnet1..: FNet's series
//
// Each file declares itself with registerConsensusNetwork, naming the
// versions from other networks that it branches from. Releasing a new
// mainline version only touches consensus_mainline.go (moving the released
// features out of vFuture). Adding a new network is a single new
// consensus_<network>.go file; no shared file changes.
//
// These are the only valid and tested consensus values and transitions. Other
// settings are not tested and may lead to unexpected behavior.
type consensusNetwork struct {
	name  string
	bases []protocol.ConsensusVersion
	init  func()
}

// consensusNetworks collects the networks registered by the
// consensus_<network>.go files in this package.
var consensusNetworks []consensusNetwork

// registerConsensusNetwork records a network's series of consensus versions
// to be defined by initConsensusProtocols. bases lists the versions from
// other networks that init derives from (via consensusFrom) or upgrades to;
// the network runs only once all of them have been registered. It is called
// from package-level var declarations in each network's file.
func registerConsensusNetwork(name string, init func(), bases ...protocol.ConsensusVersion) bool {
	consensusNetworks = append(consensusNetworks, consensusNetwork{name: name, bases: bases, init: init})
	return true
}

// initConsensusProtocols assembles the Consensus map by running every
// registered network, in an order that satisfies each network's declared base
// versions. It panics if any network's bases cannot be satisfied.
func initConsensusProtocols() {
	pending := make([]consensusNetwork, len(consensusNetworks))
	copy(pending, consensusNetworks)
	for len(pending) > 0 {
		var blocked []consensusNetwork
		for _, network := range pending {
			ready := true
			for _, base := range network.bases {
				if _, ok := Consensus[base]; !ok {
					ready = false
					break
				}
			}
			if ready {
				network.init()
			} else {
				blocked = append(blocked, network)
			}
		}
		if len(blocked) == len(pending) {
			names := make([]string, len(blocked))
			for i, network := range blocked {
				names[i] = network.name
			}
			panic(fmt.Sprintf("initConsensusProtocols: networks %v declare base versions that no network provides", names))
		}
		pending = blocked
	}
}

// branch returns a copy of these consensus parameters with a fresh, empty
// ApprovedUpgrades map, suitable as the starting point for a new version
// derived from an existing one. Plain struct assignment is not enough for
// that: it would share the ApprovedUpgrades map with the original.
func (proto ConsensusParams) branch() ConsensusParams {
	proto.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	return proto
}

// consensusFrom returns a branch (see above) of an already-registered
// consensus version, allowing a network to derive from a version defined in
// another file. The base version must appear in the network's
// registerConsensusNetwork declaration, which guarantees it is registered
// before the network's init runs.
func consensusFrom(v protocol.ConsensusVersion) ConsensusParams {
	proto, ok := Consensus[v]
	if !ok {
		panic(fmt.Sprintf("consensusFrom: %s is not registered (missing from this network's registerConsensusNetwork bases?)", v))
	}
	return proto.branch()
}

// Global defines global Algorand protocol parameters which should not be overridden.
type Global struct {
	SmallLambda time.Duration // min amount of time to wait for leader's credential (i.e., time to propagate one credential)
	BigLambda   time.Duration // max amount of time to wait for leader's proposal (i.e., time to propagate one block)
}

// Protocol holds the global configuration settings for the agreement protocol,
// initialized with our current defaults. This is used across all nodes we create.
var Protocol = Global{
	SmallLambda: 2000 * time.Millisecond,
	BigLambda:   15000 * time.Millisecond,
}

// MaxAVMBytesSize is the longest allowable AVM byteslice value It is not
// consensus values because it has never been changed (and would be very, very
// hard to change!).  No point in carrying it around in ConsensusParams.  But it
// does appear in various places around the system now, we put it here for
// accessibility.
const MaxAVMBytesSize = 4096 // Just to match largest AVM size

func init() {
	Consensus = make(ConsensusProtocols)

	initConsensusProtocols()

	// Set allocation limits
	for _, p := range Consensus {
		checkSetAllocBounds(p)
	}

}
