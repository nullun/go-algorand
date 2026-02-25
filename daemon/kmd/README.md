# kmd - Key Management Daemon

## Overview
kmd is the Key Management Daemon, the process responsible for securely managing spending keys.

## Useful facts
- kmd has a data directory separate from algod's data directory. By default, however, the kmd data directory is in the `kmd` subdirectory of algod's data directory.
- kmd starts an HTTP API server on `localhost:7833` by default.
- You talk to the HTTP API by sending json-serialized request structs from the `kmdapi` package.

## Preventing memory from swapping to disk
kmd tries to ensure that secret keys never touch the disk unencrypted. At startup, kmd tries to call [`mlockall`](https://linux.die.net/man/2/mlockall) in order to prevent the kernel from swapping memory to disk. You can check `kmd.log` after starting kmd to see if the call succeeded.

In order for the `mlockall` call to succeed, your kernel must support `mlockall`, and the user running kmd must be able to lock the necessary amount of memory. On many linux distributions, you can achieve this by calling `sudo setcap cap_ipc_lock+ep /path/to/kmd`. We also provide a make target for this: run `make capabilities` from the `go-algorand` project root.

## Ledger Hardware Wallet Support

kmd supports Ledger hardware wallets with multi-account functionality. The Ledger wallet driver implements BIP-44 derivation paths allowing you to use multiple accounts from a single device.

### BIP-44 Path Structure
Algorand uses the following BIP-44 derivation path:
```
m/44'/283'/<account>'/0/0
```
Where `<account>` is the account index (0, 1, 2, ...).

### Using Multiple Ledger Accounts

#### Listing Addresses for Different Accounts
Use the `--ledger-account` flag with `goal account list` to query a specific account index, or `--ledger-accounts` to display multiple accounts at once:

```bash
# Query the default account (index 0)
goal account list -w ledger --ledger-account 0

# Query account index 5
goal account list -w ledger --ledger-account 5

# Display the first 10 accounts (indices 0-9)
goal account list -w ledger --ledger-accounts 10

# Display accounts with balance information
goal account list -w ledger --ledger-accounts 5 --info
```

#### Signing Transactions with Different Accounts

The `--ledger-account` flag is supported on many goal commands that create and sign transactions, enabling one-step transaction creation and signing:

**Payment transactions:**
```bash
# Send payment using Ledger account index 2
goal clerk send -a 1000000 -f <from-address> -t <to-address> -w ledger --ledger-account 2
```

**Application transactions:**
```bash
# Create app using Ledger account index 3
goal app create --creator <address> --approval-prog approval.teal --clear-prog clear.teal -w ledger --ledger-account 3

# Call app using Ledger account index 1
goal app call --app-id 123 --from <address> -w ledger --ledger-account 1
```

**Asset transactions:**
```bash
# Create asset using Ledger account index 0
goal asset create --creator <address> --total 1000 --unitname TOK -w ledger --ledger-account 0

# Send asset using Ledger account index 2
goal asset send --assetid 456 --from <from-address> --to <to-address> --amount 10 -w ledger --ledger-account 2
```

**Account status changes:**
```bash
# Go online using Ledger account index 1
goal account changeonlinestatus --address <address> --online -w ledger --ledger-account 1
```

**Two-step signing (create unsigned transaction, then sign):**
```bash
# Create an unsigned transaction
goal clerk send -a 1000000 -f <from-address> -t <to-address> -o unsigned.txn -s

# Sign with Ledger account index 2
goal clerk sign -i unsigned.txn -o signed.txn -w ledger --ledger-account 2
```

### API Support
The KMD API supports an optional `account_index` parameter in the following requests:
- `APIV1POSTKeyListRequest` - List keys for a specific account index
- `APIV1POSTTransactionSignRequest` - Sign a transaction with a specific account
- `APIV1POSTMultisigTransactionSignRequest` - Sign a multisig transaction with a specific account

Wallets that support multi-account operations will have `supports_multi_account: true` in their metadata.

## Project structure
- `./`
	- `api/v1/`
		- This folder contains all of the HTTP handlers for the kmd API V1. In general, these handlers each parse a `kmdapi.APIV1Request`, and use it to run commands against a wallet.
		- Initializing these handlers requires passing a `session.Manager` to handle wallet auth and persistent state between requests.
	- `client/`
		- The `client` package provides `client.KMDClient`. `client.KMDClient.DoV1Request` infers the HTTP endpoint and method from the request type, serializes the request with msgpack, makes the request over the unix socket, and deserializes a `kmdapi.APIV1Response`.
		- The `client` package also provides wrappers for these API calls in `wrappers.go`
	- `config/`
		- This folder contains code that parses `kmd_config.json` and merges values from that file with any default values.
	- `lib/`
		- This folder contains the `kmdapi` package, which provides the canonical structs used for requests and responses.
	- `server/`
		- The `server` package is in charge of starting and stopping the kmd API server.
	- `session/`
		- The `session` package provides `session.Manager`, which allows users to interact with wallets without having to enter a password repeatedly. It achieves this by temporarily storing wallet keys in memory once they have been decrypted.
	- `wallet/`
		- `driver`
			- This folder contains the definitions of a "Wallet Driver", as well as the "SQLite Wallet Driver", kmd's default wallet backend.
			- Wallet Drivers are responsible for creating and retrieving Wallets, which store, retrieve, generate, and perform cryptographic operations on spending keys.
