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

	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
)

// ConsensusParams specifies settings that might vary based on the
// particular version of the consensus protocol.
type ConsensusParams struct {
	// Consensus protocol upgrades.  Votes for upgrades are collected for
	// UpgradeVoteRounds.  If the number of positive votes is over
	// UpgradeThreshold, the proposal is accepted.
	//
	// UpgradeVoteRounds needs to be long enough to collect an
	// accurate sample of participants, and UpgradeThreshold needs
	// to be high enough to ensure that there are sufficient participants
	// after the upgrade.
	//
	// A consensus protocol upgrade may specify the delay between its
	// acceptance and its execution.  This gives clients time to notify
	// users.  This delay is specified by the upgrade proposer and must
	// be between MinUpgradeWaitRounds and MaxUpgradeWaitRounds (inclusive)
	// in the old protocol's parameters.  Note that these parameters refer
	// to the representation of the delay in a block rather than the actual
	// delay: if the specified delay is zero, it is equivalent to
	// DefaultUpgradeWaitRounds.
	//
	// The maximum length of a consensus version string is
	// MaxVersionStringLen.
	UpgradeVoteRounds        uint64
	UpgradeThreshold         uint64
	DefaultUpgradeWaitRounds uint64
	MinUpgradeWaitRounds     uint64
	MaxUpgradeWaitRounds     uint64
	MaxVersionStringLen      int

	// MaxTxnBytesPerBlock determines the maximum number of bytes
	// that transactions can take up in a block.  Specifically,
	// the sum of the lengths of encodings of each transaction
	// in a block must not exceed MaxTxnBytesPerBlock.
	MaxTxnBytesPerBlock int

	// MaxTxnNoteBytes is the maximum size of a transaction's Note field in
	// a "basic transaction".  Larger notes require extra fees.
	MaxTxnNoteBytes int

	// MaxAbsoluteTxnNoteBytes is the absolute maximum size of a transaction's
	// Note field, even with extra fees paid. Provides DoS protection. When set
	// equal to MaxTxnNoteBytes, effectively disables large notes. When set
	// higher, allows notes up to this size with appropriate fees.
	MaxAbsoluteTxnNoteBytes int

	// MaxTxnLife is how long a transaction can be live for:
	// the maximum difference between LastValid and FirstValid.
	//
	// Note that in a protocol upgrade, the ledger must first be upgraded
	// to hold more past blocks for this value to be raised.
	MaxTxnLife uint64

	// ApprovedUpgrades describes the upgrade proposals that this protocol
	// implementation will vote for, along with their delay value
	// (in rounds).  A delay value of zero is the same as a delay of
	// DefaultUpgradeWaitRounds.
	ApprovedUpgrades map[protocol.ConsensusVersion]uint64

	// SupportGenesisHash indicates support for the GenesisHash
	// fields in transactions (and requires them in blocks).
	SupportGenesisHash bool

	// RequireGenesisHash indicates that GenesisHash must be present
	// in every transaction.
	RequireGenesisHash bool

	// DefaultKeyDilution specifies the granularity of top-level ephemeral
	// keys. KeyDilution is the number of second-level keys in each batch,
	// signed by a top-level "batch" key.  The default value can be
	// overridden in the account state.
	DefaultKeyDilution uint64

	// MinBalance specifies the minimum balance that can appear in
	// an account.  To spend money below MinBalance requires issuing
	// an account-closing transaction, which transfers all of the
	// money from the account, and deletes the account state.
	MinBalance uint64

	// MinTxnFee specifies the minimum fee allowed on a transaction.
	// A minimum fee is necessary to prevent DoS. In some sense this is
	// a way of making the spender subsidize the cost of storing this transaction.
	MinTxnFee uint64

	// EnableAppCostPooling specifies that the sum of fees for application calls
	// in a group is checked against the sum of the budget for application calls,
	// rather than check each individual app call is within the budget.
	EnableAppCostPooling bool

	// EnableLogicSigCostPooling specifies LogicSig budgets are pooled across a
	// group. The total available is len(group) * LogicSigMaxCost
	EnableLogicSigCostPooling bool

	// RewardUnit specifies the number of MicroAlgos corresponding to one reward
	// unit.
	//
	// Rewards are received by whole reward units.  Fractions of
	// RewardUnits do not receive rewards.
	//
	// Ensure both considerations below  are taken into account if RewardUnit is planned for change:
	// 1. RewardUnits should not be changed without touching all accounts to apply their rewards
	// based on the old RewardUnits and then use the new RewardUnits for all subsequent calculations.
	// 2. Having a consistent RewardUnit is also important for preserving
	// a constant amount of total algos in the system:
	// the block header tracks how many reward units worth of algos are in existence
	// and have logically received rewards.
	RewardUnit uint64

	// RewardsRateRefreshInterval is the number of rounds after which the
	// rewards level is recomputed for the next RewardsRateRefreshInterval rounds.
	RewardsRateRefreshInterval uint64

	// seed-related parameters
	SeedLookback        uint64 // how many blocks back we use seeds from in sortition. delta_s in the spec
	SeedRefreshInterval uint64 // how often an old block hash is mixed into the seed. delta_r in the spec

	// ledger retention policy
	MaxBalLookback uint64 // (current round - MaxBalLookback) is the oldest round the ledger must answer balance queries for

	// sortition threshold factors
	NumProposers           uint64
	SoftCommitteeSize      uint64
	SoftCommitteeThreshold uint64
	CertCommitteeSize      uint64
	CertCommitteeThreshold uint64
	NextCommitteeSize      uint64 // for any non-FPR votes >= deadline step, committee sizes and thresholds are constant
	NextCommitteeThreshold uint64
	LateCommitteeSize      uint64
	LateCommitteeThreshold uint64
	RedoCommitteeSize      uint64
	RedoCommitteeThreshold uint64
	DownCommitteeSize      uint64
	DownCommitteeThreshold uint64

	// time for nodes to wait for block proposal headers for period > 0, value should be set to 2 * SmallLambda
	AgreementFilterTimeout time.Duration
	// time for nodes to wait for block proposal headers for period = 0, value should be configured to suit best case
	// critical path
	AgreementFilterTimeoutPeriod0 time.Duration
	// Duration of the second agreement step for period=0, value should be configured to suit best case critical path
	AgreementDeadlineTimeoutPeriod0 time.Duration

	FastRecoveryLambda time.Duration // time between fast recovery attempts

	// how to commit to the payset: flat or merkle tree
	PaysetCommit PaysetCommitType

	MaxTimestampIncrement int64 // maximum time between timestamps on successive blocks

	// support for the efficient encoding in SignedTxnInBlock
	SupportSignedTxnInBlock bool

	// force the FeeSink address to be non-participating in the genesis balances.
	ForceNonParticipatingFeeSink bool

	// support for ApplyData in SignedTxnInBlock
	ApplyData bool

	// track reward distributions in ApplyData
	RewardsInApplyData bool

	// domain-separated credentials
	CredentialDomainSeparationEnabled bool

	// support for transactions that mark an account non-participating
	SupportBecomeNonParticipatingTransactions bool

	// fix the rewards calculation by avoiding subtracting too much from the rewards pool
	PendingResidueRewards bool

	// asset support
	Asset bool

	// max number of assets per account
	MaxAssetsPerAccount int

	// max length of asset name
	MaxAssetNameBytes int

	// max length of asset unit name
	MaxAssetUnitNameBytes int

	// max length of asset url
	MaxAssetURLBytes int

	// support sequential transaction counter TxnCounter
	TxnCounter bool

	// transaction groups
	SupportTxGroups bool

	// max group size
	MaxTxGroupSize int

	// support for transaction leases
	// note: if FixTransactionLeases is not set, the transaction
	// leases supported are faulty; specifically, they do not
	// enforce exclusion correctly when the FirstValid of
	// transactions do not match.
	SupportTransactionLeases bool
	FixTransactionLeases     bool

	// 0 for no support, otherwise highest version supported
	LogicSigVersion uint64

	// LogicSigMaxSize is the legacy LogicSig size unit used for the per-LogicSig
	// args allowance and to compute group size pools and free program-byte
	// allowance.
	LogicSigMaxSize uint64

	// MaxAbsoluteLogicSigProgramSize is the absolute maximum size of a LogicSig
	// program.
	MaxAbsoluteLogicSigProgramSize uint64

	// sum of estimated op cost must be less than this
	LogicSigMaxCost uint64

	LogicSigMsig  bool
	LogicSigLMsig bool

	// max decimal precision for assets
	MaxAssetDecimals uint32

	// SupportRekeying indicates support for account rekeying (the RekeyTo and AuthAddr fields)
	SupportRekeying bool

	// EnforceAuthAddrSenderDiff requires that AuthAddr must be empty or different from Sender
	EnforceAuthAddrSenderDiff bool

	// application support
	Application bool

	// max number of ApplicationArgs for an ApplicationCall transaction
	MaxAppArgs int

	// max sum([len(arg) for arg in txn.ApplicationArgs]) w/o paying extra
	MaxAppTotalArgLen int

	// MaxAbsoluteTotalArgLen is the absolute maximum length of app args,
	// with anything longer than MaxAppTotalArgLen costing extra
	MaxAbsoluteTotalArgLen int

	// maximum byte len of application approval program or clear state
	// When MaxExtraAppProgramPages > 0, this is the size of those pages.
	// So two "extra pages" would mean 3*MaxAppProgramLen bytes are available.
	MaxAppProgramLen int

	// maximum total length of an application's programs (approval + clear state)
	// When MaxExtraAppProgramPages > 0, this is the size of those pages.
	// So two "extra pages" would mean 3*MaxAppTotalProgramLen bytes are available.
	MaxAppTotalProgramLen int

	// extra length for application program in pages. A page is MaxAppProgramLen bytes
	MaxExtraAppProgramPages int

	// MaxAbsoluteExtraProgramPages is the absolute maximum number of extra pages allowed,
	// even with extra fees paid. Provides DoS protection.
	MaxAbsoluteExtraProgramPages int

	// maximum number of accounts in the ApplicationCall Accounts field.
	// this determines, in part, the maximum number of balance records
	// accessed by a single transaction
	MaxAppTxnAccounts int

	// maximum number of app ids in the ApplicationCall ForeignApps field.
	// these are the only applications besides the called application for
	// which global state may be read in the transaction
	MaxAppTxnForeignApps int

	// maximum number of asset ids in the ApplicationCall ForeignAssets
	// field. these are the only assets for which the asset parameters may
	// be read in the transaction
	MaxAppTxnForeignAssets int

	// maximum number of "foreign references" (accounts, asa, app, boxes) that
	// can be attached to a single app call.  Modern transactions can use
	// MaxAppAccess references in txn.Access to access more.
	MaxAppTotalTxnReferences int

	// maximum cost of application approval program or clear state program
	MaxAppProgramCost int

	// maximum length of a key used in an application's global or local
	// key/value store
	MaxAppKeyLen int

	// maximum length of a bytes value used in an application's global or
	// local key/value store
	MaxAppBytesValueLen int

	// maximum sum of the lengths of the key and value of one app state entry
	MaxAppSumKeyValueLens int

	// maximum number of inner transactions that can be created by an app call.
	// with EnableInnerTransactionPooling, limit is multiplied by MaxTxGroupSize
	// and enforced over the whole group.
	MaxInnerTransactions int

	// should the number of inner transactions be pooled across group?
	EnableInnerTransactionPooling bool

	// provide greater isolation for clear state programs
	IsolateClearState bool

	// The minimum app version that can be called in an inner transaction
	MinInnerApplVersion uint64

	// maximum number of applications a single account can create and store
	// AppParams for at once
	MaxAppsCreated int

	// maximum number of applications a single account can opt in to and
	// store AppLocalState for at once
	MaxAppsOptedIn int

	// flat MinBalance requirement for creating a single application and
	// storing its AppParams
	AppFlatParamsMinBalance uint64

	// flat MinBalance requirement for opting in to a single application
	// and storing its AppLocalState
	AppFlatOptInMinBalance uint64

	// MinBalance requirement per key/value entry in LocalState or
	// GlobalState key/value stores, regardless of value type
	SchemaMinBalancePerEntry uint64

	// MinBalance requirement (in addition to SchemaMinBalancePerEntry) for
	// integer values stored in LocalState or GlobalState key/value stores
	SchemaUintMinBalance uint64

	// MinBalance requirement (in addition to SchemaMinBalancePerEntry) for
	// []byte values stored in LocalState or GlobalState key/value stores
	SchemaBytesMinBalance uint64

	// Maximum length of a box (Does not include name/key length. That is capped by MaxAppKeyLen)
	MaxBoxSize uint64

	// Minimum Balance Requirement (MBR) per box created (this accounts for a
	// bit of overhead used to store the box bytes)
	BoxFlatMinBalance uint64

	// MBR per byte of box storage. MBR is incremented by BoxByteMinBalance * (len(name)+len(value))
	BoxByteMinBalance uint64

	// Number of box references allowed
	MaxAppBoxReferences int

	// Number of references allowed in txn.Access
	MaxAppAccess int

	// Amount added to a txgroup's box I/O budget per box ref supplied.
	// For reads: the sum of the sizes of all boxes in the group must be less than I/O budget
	// For writes: the sum of the sizes of all boxes created or written must be less than I/O budget
	// In both cases, what matters is the sizes of the boxes touched, not the
	// number of times they are touched, or the size of the touches.
	BytesPerBoxReference uint64

	// maximum number of total key/value pairs allowed by a given
	// LocalStateSchema (and therefore allowed in LocalState)
	MaxLocalSchemaEntries uint64

	// maximum number of total key/value pairs allowed by a given
	// GlobalStateSchema (and therefore allowed in GlobalState)
	MaxGlobalSchemaEntries uint64

	// maximum total minimum balance requirement for an account, used
	// to limit the maximum size of a single balance record
	MaximumMinimumBalance uint64

	// StateProofInterval defines the frequency with which state
	// proofs are generated.  Every round that is a multiple
	// of StateProofInterval, the block header will include a vector
	// commitment to the set of online accounts (that can vote after
	// another StateProofInterval rounds), and that block will be signed
	// (forming a state proof) by the voters from the previous
	// such vector commitment.  A value of zero means no state proof.
	StateProofInterval uint64

	// StateProofTopVoters is a bound on how many online accounts get to
	// participate in forming the state proof, by including the
	// top StateProofTopVoters accounts (by normalized balance) into the
	// vector commitment.
	StateProofTopVoters uint64

	// StateProofVotersLookback is the number of blocks we skip before
	// publishing a vector commitment to the online accounts.  Namely,
	// if block number N contains a vector commitment to the online
	// accounts (which, incidentally, means N%StateProofInterval=0),
	// then the balances reflected in that commitment must come from
	// block N-StateProofVotersLookback.  This gives each node some
	// time (StateProofVotersLookback blocks worth of time) to
	// construct this vector commitment, so as to avoid placing the
	// construction of this vector commitment (and obtaining the requisite
	// accounts and balances) in the critical path.
	StateProofVotersLookback uint64

	// StateProofWeightThreshold specifies the fraction of top voters weight
	// that must sign the message (block header) for security.  The state
	// proof ensures this threshold holds; however, forming a valid
	// state proof requires a somewhat higher number of signatures,
	// and the more signatures are collected, the smaller the state proof
	// can be.
	//
	// This threshold can be thought of as the maximum fraction of
	// malicious weight that state proofs defend against.
	//
	// The threshold is computed as StateProofWeightThreshold/(1<<32).
	StateProofWeightThreshold uint32

	// StateProofStrengthTarget represents either k+q (for pre-quantum security) or k+2q (for post-quantum security)
	StateProofStrengthTarget uint64

	// StateProofMaxRecoveryIntervals represents the number of state proof intervals that the network will try to catch-up with.
	// When the difference between the latest state proof and the current round will be greater than value, Nodes will
	// release resources allocated for creating state proofs.
	StateProofMaxRecoveryIntervals uint64

	// StateProofExcludeTotalWeightWithRewards specifies whether to subtract rewards from excluded online accounts along with
	// their account balances.
	StateProofExcludeTotalWeightWithRewards bool

	// StateProofBlockHashInLightHeader specifies that the LightBlockHeader
	// committed to by state proofs should contain the BlockHash of each
	// block, instead of the seed.
	StateProofBlockHashInLightHeader bool

	// EnableAssetCloseAmount adds an extra field to the ApplyData. The field contains the amount of the remaining
	// asset that were sent to the close-to address.
	EnableAssetCloseAmount bool

	// update the initial rewards rate calculation to take the reward pool minimum balance into account
	InitialRewardsRateCalculation bool

	// NoEmptyLocalDeltas updates how ApplyDelta.EvalDelta.LocalDeltas are stored
	NoEmptyLocalDeltas bool

	// EnableKeyregCoherencyCheck enable the following extra checks on key registration transactions:
	// 1. checking that [VotePK/SelectionPK/VoteKeyDilution] are all set or all clear.
	// 2. checking that the VoteFirst is less or equal to VoteLast.
	// 3. checking that in the case of going offline, both the VoteFirst and VoteLast are clear.
	// 4. checking that in the case of going online the VoteLast is non-zero and greater then the current network round.
	// 5. checking that in the case of going online the VoteFirst is less or equal to the LastValid+1.
	// 6. checking that in the case of going online the VoteFirst is less or equal to the next network round.
	EnableKeyregCoherencyCheck bool

	// When extra pages were introduced, a bug prevented the extra pages of an
	// app from being properly removed from the creator upon deletion.
	EnableProperExtraPageAccounting bool

	// Autoincrements an app's version when the app is updated, careful callers
	// may avoid making inner calls to apps that have changed.
	EnableAppVersioning bool

	// MaxProposedExpiredOnlineAccounts is the maximum number of online accounts
	// that a proposer can take offline for having expired voting keys.
	MaxProposedExpiredOnlineAccounts int

	// EnableLedgerDataUpdateRound enables the support for setting the UpdateRound on account and
	// resource data in the ledger. The UpdateRound is encoded in account/resource data types used
	// on disk and in catchpoint snapshots, and also used to construct catchpoint merkle trie keys,
	// but does not appear in on-chain state.
	EnableLedgerDataUpdateRound bool

	// When rewards rate changes, use the new value immediately.
	RewardsCalculationFix bool

	// EnableStateProofKeyregCheck enables the check for stateProof key on key registration
	EnableStateProofKeyregCheck bool

	// MaxKeyregValidPeriod defines the longest period (in rounds) allowed for a keyreg transaction.
	// This number sets a limit to prevent the number of StateProof keys generated by the user from being too large, and also checked by the WellFormed method.
	// The hard-limit for number of StateProof keys is derived from the maximum depth allowed for the merkle signature scheme's tree - 2^16.
	// More keys => deeper merkle tree => longer proof required => infeasible for our SNARK.
	MaxKeyregValidPeriod uint64

	// UnifyInnerTxIDs enables a consistent, unified way of computing inner transaction IDs
	UnifyInnerTxIDs bool

	// EnableSHA256TxnCommitmentHeader enables the creation of a transaction vector commitment tree using SHA256 hash function. (vector commitment extends Merkle tree by having a position binding property).
	// This new header is in addition to the existing SHA512_256 merkle root.
	// It is useful for verifying transaction on different blockchains, as some may not support SHA512_256 OPCODE natively but SHA256 is common.
	EnableSHA256TxnCommitmentHeader bool

	// CatchpointLookback specifies a round lookback to take catchpoints at.
	// Accounts snapshot for round X will be taken at X-CatchpointLookback
	CatchpointLookback uint64

	// DeeperBlockHeaderHistory defines number of rounds in addition to MaxTxnLife
	// available for lookup for smart contracts and smart signatures.
	// Setting it to 1 for example allows querying data up to MaxTxnLife + 1 rounds back from the Latest.
	DeeperBlockHeaderHistory uint64

	// UnfundedSenders ensures that accounts with no balance (so they don't even
	// "exist") can still be a transaction sender by avoiding updates to rewards
	// state for accounts with no algos. The actual change implemented to allow
	// this is to avoid updating an account if the only change would have been
	// the rewardsLevel, but the rewardsLevel has no meaning because the account
	// has fewer than RewardUnit algos.
	UnfundedSenders bool

	// EnablePrecheckECDSACurve means that ecdsa_verify opcode will bail early,
	// returning false, if pubkey is not on the curve.
	EnablePrecheckECDSACurve bool

	// EnableBareBudgetError specifies that I/O budget overruns should not be considered EvalError
	EnableBareBudgetError bool

	// StateProofUseTrackerVerification specifies whether the node will use data from state proof verification tracker
	// in order to verify state proofs.
	StateProofUseTrackerVerification bool

	// EnableCatchpointsWithSPContexts specifies when to re-enable version 7 catchpoints.
	// Version 7 includes state proof verification contexts
	EnableCatchpointsWithSPContexts bool

	// EnableCatchpointsWithOnlineAccounts specifies when to enable version 8 catchpoints.
	// Version 8 includes onlineaccounts and onlineroundparams amounts, for historical stake lookups.
	EnableCatchpointsWithOnlineAccounts bool

	// AppForbidLowResources enforces a rule that prevents apps from accessing
	// asas and apps below 256, in an effort to decrease the ambiguity of
	// opcodes that accept IDs or slot indexes. Simultaneously, the first ID
	// allocated in new chains is raised to 1001.
	AppForbidLowResources bool

	// EnableBoxRefNameError specifies that box ref names should be validated early
	EnableBoxRefNameError bool

	// ExcludeExpiredCirculation excludes expired stake from the total online stake
	// used by agreement for Circulation, and updates the calculation of StateProofOnlineTotalWeight used
	// by state proofs to use the same method (rather than excluding stake from the top N stakeholders as before).
	ExcludeExpiredCirculation bool

	// DynamicFilterTimeout indicates whether the filter timeout is set
	// dynamically, at run time, according to the recent history of credential
	// arrival times or is set to a static value. Even if this flag disables the
	// dynamic filter, it will be calculated and logged (but not used).
	DynamicFilterTimeout bool

	// Payouts contains parameters for amounts and eligibility for block proposer
	// payouts. It excludes information about the "unsustainable" payouts
	// described in BonusPlan.
	Payouts ProposerPayoutRules

	// Bonus contains parameters related to the extra payout made to block
	// proposers, unrelated to the fees paid in that block.  For it to actually
	// occur, extra funds need to be put into the FeeSink.  The bonus amount
	// decays exponentially.
	Bonus BonusPlan

	// Heartbeat support
	Heartbeat bool

	// EnableSha512BlockHash adds an additional SHA-512 hash to the block header.
	EnableSha512BlockHash bool

	// AppSizeUpdates allows application update transactions to change
	// the extra-program-pages and global schema sizes. Since it enables newly
	// legal transactions, this parameter can be removed and assumed true after
	// the first consensus release in which it is set true.
	AppSizeUpdates bool

	// AllowZeroLocalAppRef allows for a 0 in a LocalRef of the access list to
	// specify the current app. This parameter can be removed and assumed true
	// after the first consensus release in which it is set true.
	AllowZeroLocalAppRef bool

	// LoadTracking enables header values that track Load that grows/shrinks
	// when blocks are more/less than half full.
	LoadTracking bool

	// PerByteTxnSurcharge specifies the fee surcharge per byte for transactions
	// with large notes, app args, programs, or other fields that can go beyond
	// the basic Max sizes (they allow up to the "Absolute" Maxes). It is
	// expressed in fraction of a basic min fee.
	PerByteTxnSurcharge basics.Micros

	// EnablePQSchemeFalcon1024 enables native Falcon-1024 transaction
	// authorization for the f1 PQ scheme.
	EnablePQSchemeFalcon1024 bool

	// EnableSelectF128 changes the sortition algorithm to use a 128-bit software
	// floating point binomial CDF implementation for committee selection.
	EnableSelectF128 bool
}

// ProposerPayoutRules puts several related consensus parameters in one place. The same
// care for backward compatibility with old blocks must be taken.
type ProposerPayoutRules struct {
	// Enabled turns on several things needed for paying block incentives,
	// including tracking of the proposer and fees collected.
	Enabled bool

	// GoOnlineFee imparts a small cost on moving from offline to online. This
	// will impose a cost to running unreliable nodes that get suspended and
	// then come back online.
	GoOnlineFee uint64

	// Percent specifies the percent of fees paid in a block that go to the
	// proposer instead of the FeeSink.
	Percent uint64

	// MinBalance is the minimum balance an account must have to be eligible for
	// incentives. It ensures that smaller accounts continue to operate for the
	// same motivations they had before block incentives were
	// introduced. Without that assurance, it is difficult to model their
	// behaviour - might many participants join for the hope of easy financial
	// rewards, but without caring enough to run a high-quality node?
	MinBalance uint64

	// MaxBalance is the maximum balance an account can have to be eligible for
	// incentives. It encourages large accounts to split their stake to add
	// resilience to consensus in the case of outages.  Nothing in protocol can
	// prevent such accounts from running nodes that share fate (same machine,
	// same data center, etc), but this serves as a gentle reminder.
	MaxBalance uint64

	// MaxMarkAbsent is the maximum number of online accounts, that a proposer
	// can suspend for not proposing "lately" (In 10x expected interval, or
	// within a grace period from being challenged)
	MaxMarkAbsent int

	// Challenges occur once every challengeInterval rounds.
	ChallengeInterval uint64
	// Suspensions happen between 1 and 2 grace periods after a challenge. Must
	// be less than half MaxTxnLife to ensure the Block header will be cached
	// and less than half ChallengeInterval to avoid overlapping challenges. A larger
	// grace period means larger stake nodes will probably propose before they
	// need to consider an active heartbeat.
	ChallengeGracePeriod uint64
	// An account is challenged if the first challengeBits match the start of
	// the account address. An online account will be challenged about once
	// every interval*2^bits rounds.
	ChallengeBits int
}

// BonusPlan describes how the "extra" proposer payouts are to be made.  It
// specifies an exponential decay in which the bonus decreases by 1% every n
// rounds.  If we need to change the decay rate (only), we would create a new
// plan like:
//
//	BaseAmount: 0, DecayInterval: XXX
//
// by using a zero baseAmount, the amount is not affected.
// For a bigger change, we'd use a plan like:
//
//	BaseRound:  <FUTURE round>, BaseAmount: <new amount>, DecayInterval: <new>
//
// or just
//
//	BaseAmount: <new amount>, DecayInterval: <new>
//
// the new decay rate would go into effect at upgrade time, and the new
// amount would be set at baseRound or at upgrade time.
type BonusPlan struct {
	// BaseRound is the earliest round this plan can apply. Of course, the
	// consensus update must also have happened. So using a low value makes it
	// go into effect immediately upon upgrade.
	BaseRound uint64
	// BaseAmount is the bonus to be paid when this plan first applies (see
	// baseRound). If it is zero, then no explicit change is made to the bonus
	// (useful for only changing the decay rate).
	BaseAmount uint64
	// DecayInterval is the time in rounds between 1% decays. For simplicity,
	// decay occurs based on round % BonusDecayInterval, so a decay can happen right
	// after going into effect. The BonusDecayInterval goes into effect at upgrade
	// time, regardless of `baseRound`.
	DecayInterval uint64
}

// MinFee simply returns the MinTxnFee as a basics.MicroAlgos
func (proto ConsensusParams) MinFee() basics.MicroAlgos {
	return basics.MicroAlgos{Raw: proto.MinTxnFee}
}

// PQSchemeEnabled returns whether a post-quantum signature scheme is enabled
// under these consensus parameters.
func (proto ConsensusParams) PQSchemeEnabled(scheme protocol.PQScheme) bool {
	switch scheme {
	case protocol.PQSchemeFalcon1024:
		return proto.EnablePQSchemeFalcon1024
	default:
		return false
	}
}

// PQSchemeFeeContribution is the additional fee factor charged for a transaction
// authorized with the given PQ scheme, as a fixed-point multiple of the basic
// min fee (1e6 == one basic min fee). Making it a method (rather than exported
// constants) leaves room to vary it by proto later without changing call sites.
func (proto ConsensusParams) PQSchemeFeeContribution(scheme protocol.PQScheme) basics.Micros {
	switch scheme {
	case protocol.PQSchemeFalcon1024:
		return 2e6
	case protocol.PQSchemeFalcon512:
		return 1e6 // kept below the Falcon-1024 contribution
	default:
		return 0
	}
}

// TxnSizePricingEnabled reports whether transactions can exceed size limits by
// paying a per-byte surcharge.
func (proto ConsensusParams) TxnSizePricingEnabled() bool {
	return proto.PerByteTxnSurcharge != 0
}

// EffectiveKeyDilution returns the key dilution for this account,
// returning the default key dilution if not explicitly specified.
func (proto ConsensusParams) EffectiveKeyDilution(kd uint64) uint64 {
	if kd != 0 {
		return kd
	}
	return proto.DefaultKeyDilution
}

// BalanceRequirements returns all the consensus values that determine min balance.
func (proto ConsensusParams) BalanceRequirements() basics.BalanceRequirements {
	return basics.BalanceRequirements{
		MinBalance:               proto.MinBalance,
		AppFlatParamsMinBalance:  proto.AppFlatParamsMinBalance,
		AppFlatOptInMinBalance:   proto.AppFlatOptInMinBalance,
		BoxFlatMinBalance:        proto.BoxFlatMinBalance,
		BoxByteMinBalance:        proto.BoxByteMinBalance,
		SchemaMinBalancePerEntry: proto.SchemaMinBalancePerEntry,
		SchemaUintMinBalance:     proto.SchemaUintMinBalance,
		SchemaBytesMinBalance:    proto.SchemaBytesMinBalance,
	}
}

// PaysetCommitType enumerates possible ways for the block header to commit to
// the set of transactions in the block.
type PaysetCommitType int

const (
	// PaysetCommitUnsupported is the zero value, reflecting the fact
	// that some early protocols used a Merkle tree to commit to the
	// transactions in a way that we no longer support.
	PaysetCommitUnsupported PaysetCommitType = iota

	// PaysetCommitFlat hashes the entire payset array.
	PaysetCommitFlat

	// PaysetCommitMerkle uses merkle array to commit to the payset.
	PaysetCommitMerkle
)
