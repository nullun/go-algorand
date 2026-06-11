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

package agreement

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

func testSetup(periodCount uint64) (player, rootRouter, testAccountData, testBlockFactory, Ledger) {
	ledger, addresses, vrfSecrets, otSecrets := readOnlyFixture10()
	accs := testAccountData{addresses: addresses, vrfs: vrfSecrets, ots: otSecrets}
	round := ledger.NextRound()
	period := period(periodCount)
	player := player{Round: round, Period: period, Step: soft}

	var p actor = ioLoggedActor{checkedActor{actor: &player, actorContract: playerContract{}}, playerTracer}
	router := routerFixture
	router.root = p
	f := testBlockFactory{Owner: 1} // TODO this should change with given address

	return player, router, accs, f, ledger
}

func createProposalsTesting(accs testAccountData, round basics.Round, period period, factory BlockFactory, ledger Ledger) (ps []proposal, vs []vote) {
	ve, err := factory.AssembleBlock(round, accs.addresses)
	if err != nil {
		logging.Base().Errorf("Could not generate a proposal for round %d: %v", round, err)
		return nil, nil
	}

	// TODO this common code should be refactored out
	var votes []vote
	proposals := make([]proposal, 0)
	for i := range accs.addresses {
		payload, proposal, _ := proposalForBlock(accs.addresses[i], accs.vrfs[i], ve, period, ledger)

		// attempt to make the vote
		rv := rawVote{Sender: accs.addresses[i], Round: round, Period: period, Step: propose, Proposal: proposal}
		uv, err := makeVote(rv, accs.ots[i], accs.vrfs[i], ledger)
		if err != nil {
			logging.Base().Errorf("AccountManager.makeVotes: Could not create vote: %v", err)
			return
		}
		vote, err := uv.verify(ledger)
		if err != nil {
			continue
		}

		// create the block proposal
		proposals = append(proposals, payload)
		votes = append(votes, vote)
	}
	return proposals, votes
}

func createProposalEvents(t *testing.T, player player, accs testAccountData, f testBlockFactory, ledger Ledger) (voteBatch []event, payloadBatch []event, lowestProposal proposalValue) {
	payloads, votes := createProposalsTesting(accs, player.Round, player.Period, f, ledger)
	if len(votes) == 0 {
		return
	}

	for i := range votes {
		vote := votes[i]
		msg := message{Tag: protocol.AgreementVoteTag, Vote: vote}
		e := messageEvent{T: voteVerified, Input: msg}
		voteBatch = append(voteBatch, e)

		payload := payloads[i]
		msg = message{Tag: protocol.ProposalPayloadTag, Proposal: payload}
		e = messageEvent{T: payloadVerified, Input: msg}
		payloadBatch = append(payloadBatch, e)
	}

	lowestCredential := votes[0].Cred
	lowestProposal = votes[0].R.Proposal
	for _, vote := range votes {
		if vote.Cred.Less(lowestCredential) {
			lowestCredential = vote.Cred
			lowestProposal = vote.R.Proposal
		}
	}
	return
}

func TestProposalCreation(t *testing.T) {
	partitiontest.PartitionTest(t)

	player, router, accounts, factory, ledger := testSetup(0)

	proposalVoteEventBatch, _, _ := createProposalEvents(t, player, accounts, factory, ledger)

	simulateProposalVotes(t, &router, &player, proposalVoteEventBatch)
}

func TestProposalFunctions(t *testing.T) {
	partitiontest.PartitionTest(t)

	player, _, accs, factory, ledger := testSetup(0)
	round := player.Round
	period := player.Period
	ve, err := factory.AssembleBlock(player.Round, accs.addresses)
	require.NoError(t, err, "Could not generate a proposal for round %d: %v", round, err)

	validator := testBlockValidator{}

	for i := range accs.addresses {
		proposal, proposalValue, _ := proposalForBlock(accs.addresses[i], accs.vrfs[i], ve, period, ledger)

		//validate returning unauthenticatedProposal from proposalPayload
		unauthenticatedProposalResult := proposal
		require.NotNil(t, unauthenticatedProposalResult)

		//  validate unauthenticatedProposal
		unauthenticatedProposal := proposal.u()
		validatedProposal, err := unauthenticatedProposal.validate(context.Background(), round, ledger, validator)
		require.NoError(t, err)
		require.NotNil(t, validatedProposal)

		// validate checking for corrupted digest
		digest := proposalValue.BlockDigest
		encDigest := proposalValue.EncodingDigest
		err = proposalValue.matches(digest, encDigest)
		require.NoError(t, err)

		err = proposalValue.matches(encDigest, encDigest)
		require.Error(t, err)

		err = proposalValue.matches(digest, digest)
		require.Error(t, err)

	}
}

func TestProposalUnauthenticated(t *testing.T) {
	partitiontest.PartitionTest(t)

	player, _, accounts, factory, ledger := testSetup(0)

	round := player.Round
	period := player.Period
	testBlockFactory, err := factory.AssembleBlock(player.Round, accounts.addresses)
	require.NoError(t, err, "Could not generate a proposal for round %d: %v", round, err)

	validator := testBlockValidator{}

	accountIndex := 0

	proposal, _, _ := proposalForBlock(accounts.addresses[accountIndex], accounts.vrfs[accountIndex], testBlockFactory, period, ledger)
	accountIndex++

	// validate a good unauthenticated proposal
	unauthenticatedProposal := proposal.u()
	block := unauthenticatedProposal.Block
	require.NotNil(t, block)
	proposal, err = unauthenticatedProposal.validate(context.Background(), round, ledger, validator)
	require.NotNil(t, proposal)
	require.NoError(t, err)

	// test bad round number
	proposal, err = unauthenticatedProposal.validate(context.Background(), round+1, ledger, validator)
	require.Error(t, err)
	proposal, err = unauthenticatedProposal.validate(context.Background(), round, ledger, validator)
	require.NotNil(t, proposal)
	require.NoError(t, err)

	// validate a good unauthenticated proposal
	proposal, _, _ = proposalForBlock(accounts.addresses[accountIndex], accounts.vrfs[accountIndex], testBlockFactory, period, ledger)
	accountIndex++
	unauthenticatedProposal = proposal.u()
	block = unauthenticatedProposal.Block
	require.NotNil(t, block)

	// validate corruption of SeedProof
	proposal3, _, _ := proposalForBlock(accounts.addresses[accountIndex], accounts.vrfs[accountIndex], testBlockFactory, period, ledger)
	accountIndex++
	unauthenticatedProposal3 := proposal3.u()
	unauthenticatedProposal3.SeedProof = unauthenticatedProposal.SeedProof
	_, err = unauthenticatedProposal3.validate(context.Background(), round, ledger, validator)
	require.Error(t, err)

	// validate mismatch proposer address between block and unauthenticatedProposal
	proposal4, _, _ := proposalForBlock(accounts.addresses[accountIndex], accounts.vrfs[accountIndex], testBlockFactory, period, ledger)
	accountIndex++
	unauthenticatedProposal4 := proposal4.u()
	unauthenticatedProposal4.OriginalProposer = accounts.addresses[0] // set to the wrong address
	require.NotEqual(t, unauthenticatedProposal4.OriginalProposer, unauthenticatedProposal4.Block.Proposer())
	_, err = unauthenticatedProposal4.validate(context.Background(), round, ledger, validator)
	require.ErrorContains(t, err, "wrong proposer")
}

// TestProposalValueEncodingDigestMemo pins the consensus-identity invariants
// of the encodingDigestMemo optimization: a memoized EncodingDigest must be
// byte-identical to a fresh crypto.HashObj recompute, the memo must never be
// serialized, and a mutated or decoded copy must never reuse a foreign memo.
func TestProposalValueEncodingDigestMemo(t *testing.T) {
	partitiontest.PartitionTest(t)

	player, _, accounts, factory, ledger := testSetup(0)
	round := player.Round
	period := player.Period
	ve, err := factory.AssembleBlock(round, accounts.addresses)
	require.NoError(t, err)

	prop, pv, err := proposalForBlock(accounts.addresses[0], accounts.vrfs[0], ve, period, ledger)
	require.NoError(t, err)
	up := prop.u()

	// proposalForBlock stamps the memo with the same digest it signs into the
	// proposal votes
	require.NotNil(t, up.encodingDigestMemo)
	require.Equal(t, pv.EncodingDigest, *up.encodingDigestMemo)
	require.Equal(t, pv.EncodingDigest, up.value().EncodingDigest)

	// memo cold path: a copy without the memo recomputes identically
	cold := up
	cold.encodingDigestMemo = nil
	require.Equal(t, crypto.HashObj(cold), cold.value().EncodingDigest)
	require.Equal(t, pv.EncodingDigest, cold.value().EncodingDigest)
	require.Equal(t, cold.Digest(), cold.value().BlockDigest)

	// validate() stamps the memo onto the returned proposal; the warm path
	// must still equal a fresh recompute and the original proposalValue
	validated, err := cold.validate(context.Background(), round, ledger, testBlockValidator{})
	require.NoError(t, err)
	require.NotNil(t, validated.encodingDigestMemo)
	require.Equal(t, crypto.HashObj(validated.u()), validated.value().EncodingDigest)
	require.Equal(t, pv.EncodingDigest, validated.value().EncodingDigest)

	// the memo must not change the serialized bytes
	unstamped := validated.unauthenticatedProposal
	unstamped.encodingDigestMemo = nil
	require.Equal(t, protocol.Encode(&unstamped), protocol.Encode(&validated.unauthenticatedProposal))

	// Nothing enforces memo consistency under mutation: code that copies and
	// mutates a stamped proposal must clear (or restamp) the memo itself, as
	// done here. Production code never mutates encoded fields after stamping
	// (proposals are only produced by the construction sites and by decoding);
	// this sub-case pins the required pattern, not an automated defense.
	mutated := validated.unauthenticatedProposal
	mutated.encodingDigestMemo = nil
	mutated.OriginalPeriod++
	require.NotEqual(t, validated.value().EncodingDigest, mutated.value().EncodingDigest)
	require.Equal(t, crypto.HashObj(mutated), mutated.value().EncodingDigest)

	// decode round-trip drops the memo and recomputes identically
	var decoded unauthenticatedProposal
	err = protocol.Decode(protocol.Encode(&validated.unauthenticatedProposal), &decoded)
	require.NoError(t, err)
	require.Nil(t, decoded.encodingDigestMemo)
	require.Equal(t, validated.value(), decoded.value())
}

// mutatingBlockValidator violates the BlockValidator contract by returning a
// block whose content differs from its input.
type mutatingBlockValidator struct{}

func (v mutatingBlockValidator) Validate(ctx context.Context, e bookkeeping.Block) (ValidatedBlock, error) {
	mutated := e
	mutated.Payset = append(transactions.Payset(nil), e.Payset...)
	mutated.Payset = append(mutated.Payset, transactions.SignedTxnInBlock{})
	return testValidatedBlock{Inside: mutated}, nil
}

// TestProposalValidateMutatingValidator pins validate()'s behavior against a
// (contract-violating) BlockValidator that returns a block whose content
// differs from its input: the memo is computed from the returned proposal
// itself, so it must equal a fresh recompute of the actual (mutated) content
// rather than masking the mutation with the input's digest. The digest
// mismatch is thus preserved, and the proposalStore -- whose assemblers are
// keyed by the proposal-value derived from votes -- rejects the mutated
// payload exactly as it did before memoization, while still binding the
// honestly-validated one.
func TestProposalValidateMutatingValidator(t *testing.T) {
	partitiontest.PartitionTest(t)

	player, router, accounts, factory, ledger := testSetup(0)
	round := player.Round
	per := player.Period
	ve, err := factory.AssembleBlock(round, accounts.addresses)
	require.NoError(t, err)

	prop, pv, err := proposalForBlock(accounts.addresses[0], accounts.vrfs[0], ve, per, ledger)
	require.NoError(t, err)
	up := prop.u()

	mutValidated, err := up.validate(context.Background(), round, ledger, mutatingBlockValidator{})
	require.NoError(t, err)
	require.NotNil(t, mutValidated.encodingDigestMemo)
	require.Equal(t, crypto.HashObj(mutValidated.u()), mutValidated.value().EncodingDigest)
	require.NotEqual(t, up.value().EncodingDigest, mutValidated.value().EncodingDigest)

	// a store waiting on the vote-derived proposal-value finds no assembler
	// for the mutated payload and rejects it
	store := proposalStore{
		Relevant: map[period]proposalValue{per: pv},
		Pinned:   pv,
		Assemblers: map[proposalValue]blockAssembler{
			pv: {Pipeline: up, Authenticators: []vote{}},
		},
	}
	rHandle := routerHandle{t: &proposalStoreTracer, r: &router, src: proposalMachinePeriod}
	mutMsg := message{Tag: protocol.ProposalPayloadTag, Proposal: mutValidated, UnauthenticatedProposal: mutValidated.u()}
	ev := store.handle(rHandle, player, messageEvent{T: payloadVerified, Input: mutMsg})
	require.Equal(t, payloadRejected, ev.(payloadProcessedEvent).T)

	// while the honestly-validated payload binds to its assembler
	validated, err := up.validate(context.Background(), round, ledger, testBlockValidator{})
	require.NoError(t, err)
	honestMsg := message{Tag: protocol.ProposalPayloadTag, Proposal: validated, UnauthenticatedProposal: validated.u()}
	ev = store.handle(rHandle, player, messageEvent{T: payloadVerified, Input: honestMsg})
	require.Equal(t, payloadAccepted, ev.(payloadProcessedEvent).T)
}

func unauthenticatedProposalBlockPanicWrapper(t *testing.T, message string, uap unauthenticatedProposal, validator BlockValidator) (block bookkeeping.Block) {
	logging.Base().SetOutput(nullWriter{})
	require.Panics(t, func() { block = uap.Block })
	logging.Base().SetOutput(os.Stderr)
	return
}
