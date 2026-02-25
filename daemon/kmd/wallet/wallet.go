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

package wallet

import (
	"crypto/rand"
	"fmt"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
)

const (
	walletIDBytes = 16
)

// Wallet represents the interface that any wallet technology must satisfy in
// order to be used with KMD. Wallets start in a locked state until they are
// initialized with Init.
type Wallet interface {
	Init(pw []byte) error
	CheckPassword(pw []byte) error
	ExportMasterDerivationKey(pw []byte) (crypto.MasterDerivationKey, error)

	Metadata() (Metadata, error)

	ListKeys() ([]crypto.Digest, error)

	ImportKey(sk crypto.PrivateKey) (crypto.Digest, error)
	ExportKey(pk crypto.Digest, pw []byte) (crypto.PrivateKey, error)
	GenerateKey(displayMnemonic bool) (crypto.Digest, error)
	DeleteKey(pk crypto.Digest, pw []byte) error

	ImportMultisigAddr(version, threshold uint8, pks []crypto.PublicKey) (crypto.Digest, error)
	LookupMultisigPreimage(crypto.Digest) (version, threshold uint8, pks []crypto.PublicKey, err error)
	ListMultisigAddrs() (addrs []crypto.Digest, err error)
	DeleteMultisigAddr(addr crypto.Digest, pw []byte) error

	SignTransaction(tx transactions.Transaction, pk crypto.PublicKey, pw []byte) ([]byte, error)

	MultisigSignTransaction(tx transactions.Transaction, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, signer crypto.Digest) (crypto.MultisigSig, error)

	SignProgram(program []byte, src crypto.Digest, pw []byte) ([]byte, error)
	MultisigSignProgram(program []byte, src crypto.Digest, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, useLegacyMsig bool) (crypto.MultisigSig, error)
}

// MultiAccountWallet is an optional interface for wallets that support
// multiple accounts via BIP-44 derivation paths (like hardware wallets).
// Wallets implementing this interface can derive multiple addresses from
// a single master seed using different account indices.
//
// The account index corresponds to the third component in the BIP-44 path:
// m/44'/283'/<accountIndex>'/0/0
//
// To check if a wallet supports multi-account operations, use a type assertion:
//
//	if maw, ok := wallet.(MultiAccountWallet); ok {
//	    key, err := maw.GetPublicKeyForAccount(1)
//	}
type MultiAccountWallet interface {
	Wallet

	// GetPublicKeyForAccount retrieves the public key for a specific account index.
	// Account indices start at 0 (the default account).
	GetPublicKeyForAccount(accountIndex uint32) (crypto.Digest, error)

	// ListKeysForAccounts retrieves public keys for multiple account indices.
	// This is useful for account discovery operations.
	ListKeysForAccounts(accountIndices []uint32) ([]crypto.Digest, error)

	// SignTransactionWithAccount signs a transaction using the key at the
	// specified account index.
	SignTransactionWithAccount(tx transactions.Transaction, pk crypto.PublicKey, pw []byte, accountIndex uint32) ([]byte, error)

	// MultisigSignTransactionWithAccount signs a transaction for a multisig
	// using the key at the specified account index.
	MultisigSignTransactionWithAccount(tx transactions.Transaction, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte, signer crypto.Digest, accountIndex uint32) (crypto.MultisigSig, error)
}

// Metadata represents high-level information about a wallet, like its name, id
// and what operations it supports
type Metadata struct {
	ID                    []byte
	Name                  []byte
	DriverName            string
	DriverVersion         uint32
	SupportsMnemonicUX    bool
	SupportsMasterKey     bool
	SupportedTransactions []protocol.TxType
	// SupportsMultiAccount indicates whether this wallet supports
	// multiple accounts via BIP-44 derivation (implements MultiAccountWallet)
	SupportsMultiAccount bool
}

// GenerateWalletID generates a random hex wallet ID
func GenerateWalletID() ([]byte, error) {
	bytes := make([]byte, walletIDBytes)
	_, err := rand.Read(bytes)
	if err != nil {
		return []byte(""), err
	}
	return []byte(fmt.Sprintf("%x", bytes)), nil
}
