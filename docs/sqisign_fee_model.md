# SQIsign Fee Cost Model

This note derives the fee that the SQIsign NIST-I (`s1`) PQ scheme should
carry, from measurements of verification cost. The proof of concept charges a
placeholder of 10 extra min fees (`PQSchemeFeeContribution` in
`config/consensus.go`), which makes an `s1` transaction cost 11 min fees.

## Summary

With the pure Go verifier (`github.com/nullun/go-sqisign`), SQIsign's
verification CPU is worth about 24 extra min fees at the protocol's existing
price for stateless compute. The placeholder of 10 is about 2.4x too low. With
the portable C reference verifier, 10 is about right. Signature bytes add only
0.16.

The fee does not bound block validation time. A block full of `s1` payments
takes about 70 times the CPU of a block full of Ed25519 payments, so `s1`
needs a per-block cap whatever its fee.

All figures were measured on 2026-10-09 on an Apple M2 Pro (8 performance and
4 efficiency cores, arm64).

## Method

### Price of compute

A min fee already buys stateless compute. Each transaction carries a
LogicSig budget of 20,000 units (`LogicSigMaxCost`), and since cost pooling
(`EnableLogicSigCostPooling`), adding a transaction to a group buys 20,000
more. Signature verification is the same kind of work: it is stateless and
runs before evaluation.

The AVM prices signature opcodes in the same units. `ed25519verify` costs 1900
units and `falcon_verify` (Falcon-1024) costs 1700. Measured on this machine,
those come to 21.5 ns and 18.9 ns per unit. One min fee therefore buys about
0.40 ms of stateless CPU.

### Price of bytes

The existing Falcon fees are a byte charge with one consistent rate:

- Falcon-1024 charges 2 min fees for 2988 bytes beyond a classic Ed25519
  payment: 6.69e-4 min fee per byte.
- Falcon-512 charges 1 min fee for 1476 bytes beyond it: 6.77e-4 min fee per
  byte.

That rate is about 6.7x `PerByteTxnSurcharge` (1e-4 min fee per byte). Neither
Falcon fee contains a CPU component, since both verify faster than Ed25519.
The PR that introduced the Falcon fee (#6639) says the same: "No additional
CPU fee component needed".

### Formula

The extra fee for scheme `s`, in min fees, is the CPU it costs beyond an
Ed25519 verification at the LogicSig price, plus its bytes beyond a classic
payment at the Falcon byte price:

    extra(s) = (t_verify(s) - t_verify(ed25519)) / (20000 * t_unit)
             + 6.7e-4 * (bytes(s) - bytes(ed25519 payment))

`t_unit` is the time per AVM cost unit, calibrated from `ed25519verify` (low
end of each range below) or `falcon_verify` (high end).

### Upper bound

The upper bound prices `s1` at parity with the CPU of a classic Ed25519
payment: 58.1 us to verify plus 7.1 us to evaluate, 65.2 us in all. One min
fee buys much more than CPU (gossip, decoding, the transaction pool, storage),
so this bound overstates the fee.

## Measurements

| Scheme | Signed payment (B) | Bare verify (us) | Verify + eval per txn, one core (us) | Current extra fee |
| --- | --- | --- | --- | --- |
| Ed25519 `Sig` | 229 | 40.9 | 65.2 | 0 |
| `ed` (pqsig) | 281 | 40.9 | 65.6 | 0 |
| `f5` Falcon-512 | 1705 | 16.7 | 28.3 | 1 |
| `f1` Falcon-1024 | 3217 | 32.1 | 45.7 | 2 |
| `s1` Go port | 467 | 9707 | 9490 | 10 |
| `s1` C reference, portable | 467 | 4189 | 4203 (estimated) | none |
| `s1` C reference, arm64 asm | 467 | 2968 | 2982 (estimated) | none |

The Go port's bare-verify and per-transaction figures come from separate runs
and differ by about 2%, which is run-to-run noise. Evaluation cost is the
payment-only `BlockEvaluator` benchmark (6.6 to 7.6 us per transaction). The
C per-transaction figures add the 6.5 us non-batched transaction overhead
measured for Falcon to the bare verify time.

## Results

### Extra min fees for `s1`

| Verifier | Compute (LogicSig price) | Bytes | Total extra | Parity with a payment (upper bound) |
| --- | --- | --- | --- | --- |
| Go port (current proof of concept) | 22.5 to 25.6 | 0.16 | about 24 | 145 |
| C reference, portable | 9.6 to 11.0 | 0.16 | about 10 | 63 |
| C reference, arm64 asm | 6.8 to 7.8 | 0.16 | about 7 | 45 |

### Price of verification CPU

At +10 with the Go verifier, `s1` would be the cheapest source of
verification CPU on the network, at about half the LogicSig price:

| Way to buy verification CPU | ALGO per core-second |
| --- | --- |
| Falcon-512 / Falcon-1024 payments | 70.6 / 65.6 |
| Ed25519 payments | 15.3 |
| `s1` at +24, Go verifier | 2.63 |
| LogicSig budget | 2.47 |
| `s1` at +10, Go verifier | 1.16 |

### Block validation time

A 5 MiB block (`MaxTxnBytesPerBlock`) of `s1` payments holds 11,220
transactions. Verifying them takes 106.5 core-seconds with the Go verifier,
against 1.5 for a full block of Ed25519 payments. That measured 12.3 s of wall
time on all 12 cores here. With the arm64 asm verifier it is still 33.5
core-seconds.

A per-block cap on `s1` signatures is the only way to bound this:

| Verifier | Cap at the CPU of a full Ed25519 block (1.5 core-s) | Cap per core-second of budget |
| --- | --- | --- |
| Go port | 157 | 105 |
| C reference, portable | 355 | 238 |
| C reference, arm64 asm | 501 | 335 |

`PaysetGroups` hands out 32 transactions per workset
(`txnPerWorksetThreshold`), so 32 `s1` transactions take about 300 ms on one
core. A block with a few dozen `s1` transactions verifies on only one or two
cores. Worksets should be sized by verify cost for this scheme.

## Limits

- All measurements come from one arm64 machine. The Go port has no assembly
  on any architecture, and nobody has measured it on x86 yet.
- A consensus fee binds every node, so it should follow the slowest verifier
  that nodes actually run, not the fastest one available.
- The upper bound counts only verification and evaluation. The LogicSig price
  does not depend on what else a min fee covers.

## Reproducing

The Go figures come from benchmarks in
`data/transactions/verify/pqcost_bench_test.go`:

    go test ./data/transactions/verify -run '^$' -bench ByScheme -count 5 -benchtime 3x
    go test ./ledger -run '^$' -bench 'BlockEvaluator(RAM|Disk)NoCrypto$' -benchtime 5x -count 3

`BenchmarkTxnGroupByScheme` gives CPU time per transaction,
`BenchmarkPaysetGroupsByScheme` gives wall time with every core in use, and
`BenchmarkSignatureVerifyByScheme` gives bare verification time. The
`BlockEvaluator*Crypto` variants fail on unmodified code ("proposer missing
when payouts enabled"), so only the variants without crypto give evaluation
cost.

The C figures come from the reference implementation
([SQIsign/the-sqisign](https://github.com/SQIsign/the-sqisign), tag
`nist-v3`) built out of tree. macOS cycle counters need root, so the build
defines `NO_CYCLE_COUNTER` to time with `clock_gettime`:

    cmake -S the-sqisign -B build-arm64 -DSQISIGN_BUILD_TYPE=arm64 \
        -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS=-DNO_CYCLE_COUNTER
    cmake --build build-arm64 --target benchmark_p324_3
    build-arm64/apps/benchmark_p324_3 --iterations=100

Use `-DSQISIGN_BUILD_TYPE=ref` for the portable build.
