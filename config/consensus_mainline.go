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

// initMainlineProtocols defines the parameters for the series of consensus
// versions used by MainNet, TestNet, and BetaNet, how values change between
// versions, and vFuture: the in-development successor of the latest released
// version.
func initMainlineProtocols() {
	// WARNING: copying a ConsensusParams by value into a new variable
	// does not copy the ApprovedUpgrades map.  Make sure that each new
	// ConsensusParams structure gets a fresh ApprovedUpgrades map.

	// Base consensus protocol version, v7.
	v7 := ConsensusParams{
		UpgradeVoteRounds:        10000,
		UpgradeThreshold:         9000,
		DefaultUpgradeWaitRounds: 10000,
		MaxVersionStringLen:      64,

		MinBalance:              10000,
		MinTxnFee:               1000,
		MaxTxnLife:              1000,
		MaxTxnNoteBytes:         1024,
		MaxAbsoluteTxnNoteBytes: 1024,
		MaxTxnBytesPerBlock:     1000000,
		DefaultKeyDilution:      10000,

		MaxTimestampIncrement: 25,

		RewardUnit:                 1e6,
		RewardsRateRefreshInterval: 5e5,

		ApprovedUpgrades: map[protocol.ConsensusVersion]uint64{},

		NumProposers:           30,
		SoftCommitteeSize:      2500,
		SoftCommitteeThreshold: 1870,
		CertCommitteeSize:      1000,
		CertCommitteeThreshold: 720,
		NextCommitteeSize:      10000,
		NextCommitteeThreshold: 7750,
		LateCommitteeSize:      10000,
		LateCommitteeThreshold: 7750,
		RedoCommitteeSize:      10000,
		RedoCommitteeThreshold: 7750,
		DownCommitteeSize:      10000,
		DownCommitteeThreshold: 7750,

		AgreementFilterTimeout:          4 * time.Second,
		AgreementFilterTimeoutPeriod0:   4 * time.Second,
		AgreementDeadlineTimeoutPeriod0: Protocol.BigLambda + Protocol.SmallLambda,

		FastRecoveryLambda: 5 * time.Minute,

		SeedLookback:        2,
		SeedRefreshInterval: 100,

		MaxBalLookback: 320,

		MaxTxGroupSize: 1,
	}

	v7.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV7] = v7

	// v8 uses parameters and a seed derivation policy (the "twin seeds") from Georgios' new analysis
	v8 := v7

	v8.SeedRefreshInterval = 80
	v8.NumProposers = 9
	v8.SoftCommitteeSize = 2990
	v8.SoftCommitteeThreshold = 2267
	v8.CertCommitteeSize = 1500
	v8.CertCommitteeThreshold = 1112
	v8.NextCommitteeSize = 5000
	v8.NextCommitteeThreshold = 3838
	v8.LateCommitteeSize = 5000
	v8.LateCommitteeThreshold = 3838
	v8.RedoCommitteeSize = 5000
	v8.RedoCommitteeThreshold = 3838
	v8.DownCommitteeSize = 5000
	v8.DownCommitteeThreshold = 3838

	v8.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV8] = v8

	// v7 can be upgraded to v8.
	v7.ApprovedUpgrades[protocol.ConsensusV8] = 0

	// v9 increases the minimum balance to 100,000 microAlgos.
	v9 := v8
	v9.MinBalance = 100000
	v9.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV9] = v9

	// v8 can be upgraded to v9.
	v8.ApprovedUpgrades[protocol.ConsensusV9] = 0

	// v10 introduces fast partition recovery (and also raises NumProposers).
	v10 := v9
	v10.NumProposers = 20
	v10.LateCommitteeSize = 500
	v10.LateCommitteeThreshold = 320
	v10.RedoCommitteeSize = 2400
	v10.RedoCommitteeThreshold = 1768
	v10.DownCommitteeSize = 6000
	v10.DownCommitteeThreshold = 4560
	v10.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV10] = v10

	// v9 can be upgraded to v10.
	v9.ApprovedUpgrades[protocol.ConsensusV10] = 0

	// v11 introduces SignedTxnInBlock.
	v11 := v10
	v11.SupportSignedTxnInBlock = true
	v11.PaysetCommit = PaysetCommitFlat
	v11.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV11] = v11

	// v10 can be upgraded to v11.
	v10.ApprovedUpgrades[protocol.ConsensusV11] = 0

	// v12 increases the maximum length of a version string.
	v12 := v11
	v12.MaxVersionStringLen = 128
	v12.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV12] = v12

	// v11 can be upgraded to v12.
	v11.ApprovedUpgrades[protocol.ConsensusV12] = 0

	// v13 makes the consensus version a meaningful string.
	v13 := v12
	v13.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV13] = v13

	// v12 can be upgraded to v13.
	v12.ApprovedUpgrades[protocol.ConsensusV13] = 0

	// v14 introduces tracking of closing amounts in ApplyData, and enables
	// GenesisHash in transactions.
	v14 := v13
	v14.ApplyData = true
	v14.SupportGenesisHash = true
	v14.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV14] = v14

	// v13 can be upgraded to v14.
	v13.ApprovedUpgrades[protocol.ConsensusV14] = 0

	// v15 introduces tracking of reward distributions in ApplyData.
	v15 := v14
	v15.RewardsInApplyData = true
	v15.ForceNonParticipatingFeeSink = true
	v15.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV15] = v15

	// v14 can be upgraded to v15.
	v14.ApprovedUpgrades[protocol.ConsensusV15] = 0

	// v16 fixes domain separation in credentials.
	v16 := v15
	v16.CredentialDomainSeparationEnabled = true
	v16.RequireGenesisHash = true
	v16.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV16] = v16

	// v15 can be upgraded to v16.
	v15.ApprovedUpgrades[protocol.ConsensusV16] = 0

	// ConsensusV17 points to 'final' spec commit
	v17 := v16
	v17.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV17] = v17

	// v16 can be upgraded to v17.
	v16.ApprovedUpgrades[protocol.ConsensusV17] = 0

	// ConsensusV18 points to reward calculation spec commit
	v18 := v17
	v18.PendingResidueRewards = true
	v18.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v18.TxnCounter = true
	v18.Asset = true
	v18.LogicSigVersion = 1
	v18.LogicSigMaxSize = 1000
	v18.MaxAbsoluteLogicSigProgramSize = 1000
	v18.LogicSigMaxCost = 20000
	v18.LogicSigMsig = true
	v18.MaxAssetsPerAccount = 1000
	v18.SupportTxGroups = true
	v18.MaxTxGroupSize = 16
	v18.SupportTransactionLeases = true
	v18.SupportBecomeNonParticipatingTransactions = true
	v18.MaxAssetNameBytes = 32
	v18.MaxAssetUnitNameBytes = 8
	v18.MaxAssetURLBytes = 32
	Consensus[protocol.ConsensusV18] = v18

	// ConsensusV19 is the official spec commit ( teal, assets, group tx )
	v19 := v18
	v19.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	Consensus[protocol.ConsensusV19] = v19

	// v18 can be upgraded to v19.
	v18.ApprovedUpgrades[protocol.ConsensusV19] = 0
	// v17 can be upgraded to v19.
	v17.ApprovedUpgrades[protocol.ConsensusV19] = 0

	// v20 points to adding the precision to the assets.
	v20 := v19
	v20.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v20.MaxAssetDecimals = 19
	// we want to adjust the upgrade time to be roughly one week.
	// one week, in term of rounds would be:
	// 140651 = (7 * 24 * 60 * 60 / 4.3)
	// for the sake of future manual calculations, we'll round that down
	// a bit :
	v20.DefaultUpgradeWaitRounds = 140000
	Consensus[protocol.ConsensusV20] = v20

	// v19 can be upgraded to v20.
	v19.ApprovedUpgrades[protocol.ConsensusV20] = 0

	// v21 fixes a bug in Credential.lowestOutput that would cause larger accounts to be selected to propose disproportionately more often than small accounts
	v21 := v20
	v21.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	Consensus[protocol.ConsensusV21] = v21
	// v20 can be upgraded to v21.
	v20.ApprovedUpgrades[protocol.ConsensusV21] = 0

	// v22 is an upgrade which allows tuning the number of rounds to wait to execute upgrades.
	v22 := v21
	v22.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v22.MinUpgradeWaitRounds = 10000
	v22.MaxUpgradeWaitRounds = 150000
	Consensus[protocol.ConsensusV22] = v22

	// v23 is an upgrade which fixes the behavior of leases so that
	// it conforms with the intended spec.
	v23 := v22
	v23.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v23.FixTransactionLeases = true
	Consensus[protocol.ConsensusV23] = v23
	// v22 can be upgraded to v23.
	v22.ApprovedUpgrades[protocol.ConsensusV23] = 10000
	// v21 can be upgraded to v23.
	v21.ApprovedUpgrades[protocol.ConsensusV23] = 0

	// v24 is the stateful teal and rekeying upgrade
	v24 := v23
	v24.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v24.LogicSigVersion = 2

	// Enable application support
	v24.Application = true

	// Although Inners were not allowed yet, this gates downgrade checks, which must be allowed
	v24.MinInnerApplVersion = 6

	// Enable rekeying
	v24.SupportRekeying = true

	// 100.1 Algos (MinBalance for creating 1,000 assets)
	v24.MaximumMinimumBalance = 100100000

	v24.MaxAppArgs = 16
	v24.MaxAppTotalArgLen = 2048
	v24.MaxAbsoluteTotalArgLen = 2048
	v24.MaxAppProgramLen = 1024
	v24.MaxAppTotalProgramLen = 2048 // No effect until v28, when MaxAppProgramLen increased
	v24.MaxAppKeyLen = 64
	v24.MaxAppBytesValueLen = 64
	v24.MaxAppSumKeyValueLens = 128 // Set here to have no effect until MaxAppBytesValueLen increases

	// 0.1 Algos (Same min balance cost as an Asset)
	v24.AppFlatParamsMinBalance = 100000
	v24.AppFlatOptInMinBalance = 100000

	// Can look up Sender + 4 other balance records per Application txn
	v24.MaxAppTxnAccounts = 4

	// Can look up 2 other app creator balance records to see global state
	v24.MaxAppTxnForeignApps = 2

	// Can look up 2 assets to see asset parameters
	v24.MaxAppTxnForeignAssets = 2

	// Intended to have no effect in v24 (it's set to accounts +
	// asas + apps). In later vers, it allows increasing the
	// individual limits while maintaining same max references.
	v24.MaxAppTotalTxnReferences = 8

	// 64 byte keys @ ~333 microAlgos/byte + delta
	v24.SchemaMinBalancePerEntry = 25000

	// 9 bytes @ ~333 microAlgos/byte + delta
	v24.SchemaUintMinBalance = 3500

	// 64 byte values @ ~333 microAlgos/byte + delta
	v24.SchemaBytesMinBalance = 25000

	// Maximum number of key/value pairs per local key/value store
	v24.MaxLocalSchemaEntries = 16

	// Maximum number of key/value pairs per global key/value store
	v24.MaxGlobalSchemaEntries = 64

	// Maximum cost of ApprovalProgram/ClearStateProgram
	v24.MaxAppProgramCost = 700

	// Maximum number of apps a single account can create
	v24.MaxAppsCreated = 10

	// Maximum number of apps a single account can opt into
	v24.MaxAppsOptedIn = 10
	Consensus[protocol.ConsensusV24] = v24

	// v23 can be upgraded to v24, with an update delay of 7 days ( see calculation above )
	v23.ApprovedUpgrades[protocol.ConsensusV24] = 140000

	// v25 enables AssetCloseAmount in the ApplyData
	v25 := v24
	v25.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable AssetCloseAmount field
	v25.EnableAssetCloseAmount = true
	Consensus[protocol.ConsensusV25] = v25

	// v26 adds support for teal3
	v26 := v25
	v26.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable the InitialRewardsRateCalculation fix
	v26.InitialRewardsRateCalculation = true

	// Enable transaction Merkle tree.
	v26.PaysetCommit = PaysetCommitMerkle

	// Enable teal3
	v26.LogicSigVersion = 3

	Consensus[protocol.ConsensusV26] = v26

	// v25 or v24 can be upgraded to v26, with an update delay of 7 days ( see calculation above )
	v25.ApprovedUpgrades[protocol.ConsensusV26] = 140000
	v24.ApprovedUpgrades[protocol.ConsensusV26] = 140000

	// v27 updates ApplyDelta.EvalDelta.LocalDeltas format
	v27 := v26
	v27.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable the ApplyDelta.EvalDelta.LocalDeltas fix
	v27.NoEmptyLocalDeltas = true

	Consensus[protocol.ConsensusV27] = v27

	// v26 can be upgraded to v27, with an update delay of 3 days
	// 60279 = (3 * 24 * 60 * 60 / 4.3)
	// for the sake of future manual calculations, we'll round that down
	// a bit :
	v26.ApprovedUpgrades[protocol.ConsensusV27] = 60000

	// v28 introduces new TEAL features, larger program size, fee pooling and longer asset max URL
	v28 := v27
	v28.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable TEAL 4 / AVM 0.9
	v28.LogicSigVersion = 4
	// Enable support for larger app program size
	v28.MaxExtraAppProgramPages = 3
	v28.MaxAbsoluteExtraProgramPages = 3
	v28.MaxAppProgramLen = 2048
	// Increase asset URL length to allow for IPFS URLs
	v28.MaxAssetURLBytes = 96
	// Let the bytes value take more space. Key+Value is still limited to 128
	v28.MaxAppBytesValueLen = 128

	// Individual limits raised
	v28.MaxAppTxnForeignApps = 8
	v28.MaxAppTxnForeignAssets = 8

	// MaxAppTxnAccounts has not been raised yet.  It is already
	// higher (4) and there is a multiplicative effect in
	// "reachability" between accounts and creatables, so we
	// retain 4 x 4 as worst case.

	v28.EnableKeyregCoherencyCheck = true

	Consensus[protocol.ConsensusV28] = v28

	// v27 can be upgraded to v28, with an update delay of 7 days ( see calculation above )
	v27.ApprovedUpgrades[protocol.ConsensusV28] = 140000

	// v29 fixes application update by using ExtraProgramPages in size calculations
	v29 := v28
	v29.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Fix the accounting bug
	v29.EnableProperExtraPageAccounting = true

	Consensus[protocol.ConsensusV29] = v29

	// v28 can be upgraded to v29, with an update delay of 3 days ( see calculation above )
	v28.ApprovedUpgrades[protocol.ConsensusV29] = 60000

	// v30 introduces AVM 1.0 and TEAL 5, increases the app opt in limit to 50,
	// and allows costs to be pooled in grouped stateful transactions.
	v30 := v29
	v30.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable TEAL 5 / AVM 1.0
	v30.LogicSigVersion = 5

	// Enable App calls to pool budget in grouped transactions
	v30.EnableAppCostPooling = true

	// Enable Inner Transactions, and set maximum number. 0 value is
	// disabled.  Value > 0 also activates storage of creatable IDs in
	// ApplyData, as that is required to support REST API when inner
	// transactions are activated.
	v30.MaxInnerTransactions = 16

	// Allow 50 app opt ins
	v30.MaxAppsOptedIn = 50

	Consensus[protocol.ConsensusV30] = v30

	// v29 can be upgraded to v30, with an update delay of 7 days ( see calculation above )
	v29.ApprovedUpgrades[protocol.ConsensusV30] = 140000

	v31 := v30
	v31.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}
	v31.RewardsCalculationFix = true
	v31.MaxProposedExpiredOnlineAccounts = 32

	// Enable TEAL 6 / AVM 1.1
	v31.LogicSigVersion = 6
	v31.EnableInnerTransactionPooling = true
	v31.IsolateClearState = true

	// stat proof key registration
	v31.EnableStateProofKeyregCheck = true

	// Maximum validity period for key registration, to prevent generating too many StateProof keys
	v31.MaxKeyregValidPeriod = 256*(1<<16) - 1

	Consensus[protocol.ConsensusV31] = v31

	// v30 can be upgraded to v31, with an update delay of 7 days ( see calculation above )
	v30.ApprovedUpgrades[protocol.ConsensusV31] = 140000

	v32 := v31
	v32.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable extended application storage; binaries that contain support for this
	// flag would already be restructuring their internal storage for extended
	// application storage, and therefore would not produce catchpoints and/or
	// catchpoint labels prior to this feature being enabled.
	v32.EnableLedgerDataUpdateRound = true

	// Remove limits on MinimumBalance
	v32.MaximumMinimumBalance = 0

	// Remove limits on assets / account.
	v32.MaxAssetsPerAccount = 0

	// Remove limits on maximum number of apps a single account can create
	v32.MaxAppsCreated = 0

	// Remove limits on maximum number of apps a single account can opt into
	v32.MaxAppsOptedIn = 0

	Consensus[protocol.ConsensusV32] = v32

	// v31 can be upgraded to v32, with an update delay of 7 days ( see calculation above )
	v31.ApprovedUpgrades[protocol.ConsensusV32] = 140000

	v33 := v32
	v33.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Make the accounts snapshot for round X at X-CatchpointLookback
	// order to guarantee all nodes produce catchpoint at the same round.
	v33.CatchpointLookback = 320

	// Require MaxTxnLife + X blocks and headers preserved by a node
	v33.DeeperBlockHeaderHistory = 1

	v33.MaxTxnBytesPerBlock = 5 * 1024 * 1024

	Consensus[protocol.ConsensusV33] = v33

	// v32 can be upgraded to v33, with an update delay of 7 days ( see calculation above )
	v32.ApprovedUpgrades[protocol.ConsensusV33] = 140000

	v34 := v33
	v34.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Enable state proofs.
	v34.StateProofInterval = 256
	v34.StateProofTopVoters = 1024
	v34.StateProofVotersLookback = 16
	v34.StateProofWeightThreshold = (1 << 32) * 30 / 100
	v34.StateProofStrengthTarget = 256
	v34.StateProofMaxRecoveryIntervals = 10

	v34.LogicSigVersion = 7
	v34.MinInnerApplVersion = 4

	v34.UnifyInnerTxIDs = true

	v34.EnableSHA256TxnCommitmentHeader = true

	v34.UnfundedSenders = true

	v34.AgreementFilterTimeoutPeriod0 = 3400 * time.Millisecond

	Consensus[protocol.ConsensusV34] = v34

	v35 := v34
	v35.StateProofExcludeTotalWeightWithRewards = true

	v35.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	Consensus[protocol.ConsensusV35] = v35

	// v33 and v34 can be upgraded to v35, with an update delay of 12h:
	// 10046 = (12 * 60 * 60 / 4.3)
	// for the sake of future manual calculations, we'll round that down a bit :
	v33.ApprovedUpgrades[protocol.ConsensusV35] = 10000
	v34.ApprovedUpgrades[protocol.ConsensusV35] = 10000

	v36 := v35
	v36.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// Boxes (unlimited global storage)
	v36.LogicSigVersion = 8
	v36.MaxBoxSize = 32768
	v36.BoxFlatMinBalance = 2500
	v36.BoxByteMinBalance = 400
	v36.MaxAppBoxReferences = 8
	v36.BytesPerBoxReference = 1024

	Consensus[protocol.ConsensusV36] = v36

	v35.ApprovedUpgrades[protocol.ConsensusV36] = 140000

	v37 := v36
	v37.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	Consensus[protocol.ConsensusV37] = v37

	// v36 can be upgraded to v37, with an update delay of 7 days ( see calculation above )
	v36.ApprovedUpgrades[protocol.ConsensusV37] = 140000

	v38 := v37
	v38.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	// enables state proof recoverability
	v38.StateProofUseTrackerVerification = true
	v38.EnableCatchpointsWithSPContexts = true

	// online circulation on-demand expiration
	v38.ExcludeExpiredCirculation = true

	// TEAL resources sharing and other features
	v38.LogicSigVersion = 9
	v38.EnablePrecheckECDSACurve = true
	v38.AppForbidLowResources = true
	v38.EnableBareBudgetError = true
	v38.EnableBoxRefNameError = true

	v38.AgreementFilterTimeoutPeriod0 = 3000 * time.Millisecond

	Consensus[protocol.ConsensusV38] = v38

	// v37 can be upgraded to v38, with an update delay of 12h:
	// 10046 = (12 * 60 * 60 / 4.3)
	// for the sake of future manual calculations, we'll round that down a bit :
	v37.ApprovedUpgrades[protocol.ConsensusV38] = 10000

	v39 := v38
	v39.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	v39.LogicSigVersion = 10
	v39.EnableLogicSigCostPooling = true

	v39.AgreementDeadlineTimeoutPeriod0 = 4 * time.Second

	v39.DynamicFilterTimeout = true

	v39.StateProofBlockHashInLightHeader = true

	// For future upgrades, round times will likely be shorter so giving ourselves some buffer room
	v39.MaxUpgradeWaitRounds = 250000

	Consensus[protocol.ConsensusV39] = v39

	// v38 can be upgraded to v39, with an update delay of 7d:
	// 157000 = (7 * 24 * 60 * 60 / 3.3 round times currently)
	// but our current max is 150000 so using that :
	v38.ApprovedUpgrades[protocol.ConsensusV39] = 150000

	v40 := v39
	v40.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	v40.LogicSigVersion = 11
	v40.MaxAbsoluteLogicSigProgramSize = 16000

	v40.Payouts.Enabled = true
	v40.Payouts.Percent = 50
	v40.Payouts.GoOnlineFee = 2_000_000         // 2 algos
	v40.Payouts.MinBalance = 30_000_000_000     // 30,000 algos
	v40.Payouts.MaxBalance = 70_000_000_000_000 // 70M algos
	v40.Payouts.MaxMarkAbsent = 32
	v40.Payouts.ChallengeInterval = 1000
	v40.Payouts.ChallengeGracePeriod = 200
	v40.Payouts.ChallengeBits = 5

	v40.Bonus.BaseAmount = 10_000_000 // 10 Algos
	// 2.9 sec rounds gives about 10.8M rounds per year.
	v40.Bonus.DecayInterval = 1_000_000 // .99^(10.8M/1M) ~ .897. So ~10% decay per year

	v40.Heartbeat = true

	v40.EnableCatchpointsWithOnlineAccounts = true

	Consensus[protocol.ConsensusV40] = v40

	// v39 can be upgraded to v40, with an update delay of 7d:
	// 208000 = (7 * 24 * 60 * 60 / 2.9 ballpark round times)
	// our current max is 250000
	v39.ApprovedUpgrades[protocol.ConsensusV40] = 208000

	v41 := v40
	v41.ApprovedUpgrades = map[protocol.ConsensusVersion]uint64{}

	v41.LogicSigVersion = 12

	v41.EnableAppVersioning = true
	v41.EnableSha512BlockHash = true

	// txn.Access work
	v41.MaxAppTxnAccounts = 8       // Accounts are no worse than others, they should be the same
	v41.MaxAppAccess = 16           // Twice as many, though cross products are explicit
	v41.BytesPerBoxReference = 2048 // Count is more important that bytes, loosen up
	v41.LogicSigMsig = false
	v41.LogicSigLMsig = true

	Consensus[protocol.ConsensusV41] = v41

	// v40 can be upgraded to v41, with an update delay of 7d:
	// 208000 = (7 * 24 * 60 * 60 / 2.9 ballpark round times)
	// our current max is 250000
	v40.ApprovedUpgrades[protocol.ConsensusV41] = 208000

	// ConsensusFuture is used to test features that are implemented
	// but not yet released in a production protocol version. It is the
	// gated staging area for the next version in this series.
	vFuture := v41.branch()

	vFuture.LogicSigVersion = 13 // When moving this to a release, put a new higher LogicSigVersion here
	vFuture.AppSizeUpdates = true
	vFuture.AllowZeroLocalAppRef = true
	vFuture.EnforceAuthAddrSenderDiff = true
	vFuture.EnablePQSchemeFalcon1024 = true
	vFuture.LoadTracking = true
	vFuture.MaxAbsoluteTxnNoteBytes = 4096   // same as largest AVM value
	vFuture.MaxAbsoluteExtraProgramPages = 7 // Allow larger programs with extra fees
	vFuture.MaxAbsoluteTotalArgLen = 16384   // We _could_ make this as high as 16*4k
	vFuture.PerByteTxnSurcharge = 100        // Each charged byte adds 0.000100 of min fee
	vFuture.EnableSelectF128 = true

	Consensus[protocol.ConsensusFuture] = vFuture
}
