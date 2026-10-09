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
	"fmt"
	"io"

	"github.com/nullun/go-sqisign"
	"golang.org/x/crypto/sha3"
)

// SQIsign (NIST round 3 submission, spec v3.0) at NIST security level I,
// through the pure Go port github.com/nullun/go-sqisign.
//
// The reference signs with fresh randomness. Like the Falcon schemes, the
// s1 scheme uses a deterministic signing profile instead: the signing
// randomness is a SHAKE256 stream keyed by the private key and the message.
// Verification does not depend on how the signer drew its randomness, so
// s1 signatures are ordinary SQIsign signatures.

const (
	// SQIsignSeedSize is the size in bytes of a SQIsign keygen seed.
	SQIsignSeedSize = 32

	// SQIsign1PublicKeySize is the size in bytes of a SQIsign NIST-I public key.
	SQIsign1PublicKeySize = 83
	// SQIsign1PrivateKeySize is the size in bytes of a SQIsign NIST-I private key.
	SQIsign1PrivateKeySize = 270
	// SQIsign1SignatureSize is the size in bytes of a SQIsign NIST-I signature.
	SQIsign1SignatureSize = 200
)

// Domain separators for the SHAKE256 streams that feed SQIsign keygen and
// signing.
const (
	sqisign1KeygenDomain = "ALGO-SQISIGN1-KEYGEN"
	sqisign1SignDomain   = "ALGO-SQISIGN1-SIGN"
)

type (
	// SQIsignSeed is the seed of a deterministic SQIsign key generation.
	//msgp:ignore SQIsignSeed
	SQIsignSeed [SQIsignSeedSize]byte

	// SQIsign1PublicKey is an encoded SQIsign NIST-I public key.
	//msgp:ignore SQIsign1PublicKey
	SQIsign1PublicKey [SQIsign1PublicKeySize]byte
	// SQIsign1PrivateKey is an encoded SQIsign NIST-I private key. Its
	// encoding starts with the encoding of its public key.
	//msgp:ignore SQIsign1PrivateKey
	SQIsign1PrivateKey [SQIsign1PrivateKeySize]byte
	// SQIsign1Signature is a SQIsign NIST-I signature.
	//msgp:ignore SQIsign1Signature
	SQIsign1Signature []byte
)

// SQIsign1Signer signs with a SQIsign NIST-I key pair.
//
//msgp:ignore SQIsign1Signer
type SQIsign1Signer struct {
	PublicKey  SQIsign1PublicKey
	PrivateKey SQIsign1PrivateKey
}

// shakeStream returns a SHAKE256 stream over domain and the parts.
func shakeStream(domain string, parts ...[]byte) io.Reader {
	h := sha3.NewShake256()
	h.Write([]byte(domain))
	for _, p := range parts {
		h.Write(p)
	}
	return h
}

// GenerateSQIsign1Signer deterministically generates a SQIsign NIST-I signer
// from seed.
func GenerateSQIsign1Signer(seed SQIsignSeed) (SQIsign1Signer, error) {
	pub, priv, err := sqisign.Level1.GenerateKey(shakeStream(sqisign1KeygenDomain, seed[:]))
	if err != nil {
		return SQIsign1Signer{}, err
	}
	var signer SQIsign1Signer
	copy(signer.PublicKey[:], pub.Bytes())
	copy(signer.PrivateKey[:], priv.Bytes())
	return signer, nil
}

// NewSQIsign1Signer creates a SQIsign NIST-I signer from a random seed.
func NewSQIsign1Signer() (*SQIsign1Signer, error) {
	var seed SQIsignSeed
	RandBytes(seed[:])
	signer, err := GenerateSQIsign1Signer(seed)
	if err != nil {
		return &SQIsign1Signer{}, err
	}
	return &signer, nil
}

// Sign signs the to-be-hashed representation of message.
func (s *SQIsign1Signer) Sign(message Hashable) (SQIsign1Signature, error) {
	return s.SignBytes(HashRep(message))
}

// SignBytes signs data. The signature is a deterministic function of the
// private key and data.
func (s *SQIsign1Signer) SignBytes(data []byte) (SQIsign1Signature, error) {
	sk, err := sqisign.Level1.NewPrivateKey(s.PrivateKey[:])
	if err != nil {
		return nil, err
	}
	return sk.Sign(shakeStream(sqisign1SignDomain, s.PrivateKey[:], data), data, nil)
}

// VerifySQIsign1 verifies a SQIsign NIST-I signature over message.
func VerifySQIsign1(message Hashable, publicKey []byte, signature []byte) error {
	return VerifySQIsign1Bytes(HashRep(message), publicKey, signature)
}

// VerifySQIsign1Bytes verifies a SQIsign NIST-I signature over data.
func VerifySQIsign1Bytes(data []byte, publicKey []byte, signature []byte) error {
	if len(publicKey) != SQIsign1PublicKeySize {
		return fmt.Errorf("sqisign-1 %w: public key size %d, want %d", ErrSigInvalid, len(publicKey), SQIsign1PublicKeySize)
	}
	if len(signature) != SQIsign1SignatureSize {
		return fmt.Errorf("sqisign-1 %w: signature size %d, want %d", ErrSigInvalid, len(signature), SQIsign1SignatureSize)
	}
	pk, err := sqisign.Level1.NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("sqisign-1 %w: %w", ErrSigInvalid, err)
	}
	if !pk.Verify(data, signature) {
		return fmt.Errorf("sqisign-1 %w", ErrSigInvalid)
	}
	return nil
}
