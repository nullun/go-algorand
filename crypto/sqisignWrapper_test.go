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

package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/nullun/go-sqisign"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/test/partitiontest"
)

// sqisign1TestSigner caches one key pair: SQIsign keygen takes tens of
// milliseconds.
var sqisign1TestSigner = func() SQIsign1Signer {
	signer, err := GenerateSQIsign1Signer(SQIsignSeed{1})
	if err != nil {
		panic(err)
	}
	return signer
}()

func TestSQIsign1Sizes(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	require.Equal(t, sqisign.Level1.PublicKeySize(), SQIsign1PublicKeySize)
	require.Equal(t, sqisign.Level1.PrivateKeySize(), SQIsign1PrivateKeySize)
	require.Equal(t, sqisign.Level1.SignatureSize(), SQIsign1SignatureSize)
}

func TestSQIsign1SignVerify(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	signer := sqisign1TestSigner
	msg := TestingHashable{data: []byte("sqisign round trip")}
	sig, err := signer.Sign(msg)
	require.NoError(t, err)
	require.Len(t, sig, SQIsign1SignatureSize)

	require.NoError(t, VerifySQIsign1(msg, signer.PublicKey[:], sig))

	// The signature is an ordinary SQIsign signature over HashRep(msg).
	pk, err := sqisign.Level1.NewPublicKey(signer.PublicKey[:])
	require.NoError(t, err)
	require.True(t, pk.Verify(HashRep(msg), sig))

	require.ErrorIs(t, VerifySQIsign1(TestingHashable{data: []byte("other")}, signer.PublicKey[:], sig), ErrSigInvalid)
	require.ErrorIs(t, VerifySQIsign1(msg, signer.PublicKey[:], nil), ErrSigInvalid)
	require.ErrorIs(t, VerifySQIsign1(msg, signer.PublicKey[:], sig[:len(sig)-1]), ErrSigInvalid)
	require.ErrorIs(t, VerifySQIsign1(msg, signer.PublicKey[:], append(sig, 0)), ErrSigInvalid)
	require.ErrorIs(t, VerifySQIsign1(msg, nil, sig), ErrSigInvalid)
	require.ErrorIs(t, VerifySQIsign1(msg, signer.PublicKey[:len(signer.PublicKey)-1], sig), ErrSigInvalid)

	other, err := GenerateSQIsign1Signer(SQIsignSeed{2})
	require.NoError(t, err)
	require.ErrorIs(t, VerifySQIsign1(msg, other.PublicKey[:], sig), ErrSigInvalid)

	v, ok := LookupPQScheme(protocol.PQSchemeSQIsign1)
	require.True(t, ok)
	require.NoError(t, v.Verify(msg, signer.PublicKey[:], sig))
	require.ErrorIs(t, v.Verify(msg, other.PublicKey[:], sig), ErrSigInvalid)
}

// TestSQIsign1Deterministic checks that keygen is a function of the seed and
// signing a function of the key and message.
func TestSQIsign1Deterministic(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	again, err := GenerateSQIsign1Signer(SQIsignSeed{1})
	require.NoError(t, err)
	require.Equal(t, sqisign1TestSigner, again)

	msg := []byte("deterministic")
	sig1, err := sqisign1TestSigner.SignBytes(msg)
	require.NoError(t, err)
	sig2, err := again.SignBytes(msg)
	require.NoError(t, err)
	require.Equal(t, sig1, sig2)

	sig3, err := sqisign1TestSigner.SignBytes([]byte("deterministic!"))
	require.NoError(t, err)
	require.NotEqual(t, sig1, sig3)
}

// TestSQIsign1KnownAnswer pins the key and signature derived from a fixed
// seed, so that a change in the derivation or in go-sqisign is noticed.
// Addresses of s1 accounts depend on the public key.
func TestSQIsign1KnownAnswer(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	signer, err := GenerateSQIsign1Signer(SQIsignSeed{0xaa})
	require.NoError(t, err)
	sig, err := signer.SignBytes([]byte("algorand"))
	require.NoError(t, err)

	pkHash := Hash(signer.PublicKey[:])
	sigHash := Hash(sig)
	require.Equal(t, "834a0b14a222124888e071aa9ace981412136d30eed9f7cffe3ea1e6c5b57985", hex.EncodeToString(pkHash[:]))
	require.Equal(t, "507393e2bd55b2c562c110cefcf8e2f00fddb42ccd6f555305f11fdf699629e1", hex.EncodeToString(sigHash[:]))
}

// TestSQIsign1MutatedSignature flips each byte of a valid signature and of
// its public key. Verification must reject every mutation without panicking.
func TestSQIsign1MutatedSignature(t *testing.T) {
	partitiontest.PartitionTest(t)
	if testing.Short() {
		t.Skip("verifies about 280 signatures")
	}
	t.Parallel()

	signer := sqisign1TestSigner
	msg := []byte("mutations")
	sig, err := signer.SignBytes(msg)
	require.NoError(t, err)

	for i := range sig {
		bad := append(SQIsign1Signature(nil), sig...)
		bad[i] ^= 0x01
		require.ErrorIs(t, VerifySQIsign1Bytes(msg, signer.PublicKey[:], bad), ErrSigInvalid, "signature byte %d", i)
	}
	for i := range signer.PublicKey {
		bad := signer.PublicKey
		bad[i] ^= 0x01
		require.ErrorIs(t, VerifySQIsign1Bytes(msg, bad[:], sig), ErrSigInvalid, "public key byte %d", i)
	}
}

// FuzzVerifySQIsign1 feeds arbitrary public keys and signatures to the
// verifier, which parses attacker-controlled bytes in the consensus path.
func FuzzVerifySQIsign1(f *testing.F) {
	signer := sqisign1TestSigner
	msg := []byte("fuzz")
	sig, err := signer.SignBytes(msg)
	require.NoError(f, err)
	f.Add(signer.PublicKey[:], []byte(sig), msg)
	f.Add(make([]byte, SQIsign1PublicKeySize), make([]byte, SQIsign1SignatureSize), msg)

	// Inputs are fitted to the exact sizes, so that every one gets past the
	// length checks and into the decoder.
	fit := func(b []byte, n int) []byte {
		out := make([]byte, n)
		copy(out, b)
		return out
	}
	f.Fuzz(func(t *testing.T, pk, s, m []byte) {
		pk = fit(pk, SQIsign1PublicKeySize)
		s = fit(s, SQIsign1SignatureSize)
		err := VerifySQIsign1Bytes(m, pk, s)
		if err == nil {
			// Only a real signature verifies.
			require.Equal(t, signer.PublicKey[:], pk)
		}
	})
}

func BenchmarkSQIsign1Verify(b *testing.B) {
	signer := sqisign1TestSigner
	msg := TestingHashable{data: []byte("bench")}
	sig, err := signer.Sign(msg)
	require.NoError(b, err)
	for b.Loop() {
		if err := VerifySQIsign1(msg, signer.PublicKey[:], sig); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSQIsign1Sign(b *testing.B) {
	signer := sqisign1TestSigner
	msg := TestingHashable{data: []byte("bench")}
	for b.Loop() {
		if _, err := signer.Sign(msg); err != nil {
			b.Fatal(err)
		}
	}
}
