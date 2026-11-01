# Retention cleanup: investigation and benchmarks

The Hub stores incoming Docker logs in SQLite. Keeping that database within a
configured size limit requires removing older rows, but the original cleanup
path became increasingly expensive as the number of rows grew.

This report documents the bottleneck, the change made to the cleanup path, and
the measurements used to check it. The numbers describe one synthetic benchmark
on one machine; they are not production throughput guarantees.

## What was measured

I investigated commit `02e372a` using a Linux/amd64 Hub binary, Go symbol
and instruction inspection, and a repeatable SQLite cleanup benchmark.

After collecting the baseline, I used the following plan:

1. Replace repeated full-table payload sums and individual deletions with one
   sum, one oldest-first boundary scan, and one indexed prefix deletion.
2. Preserve existing retention semantics, including timestamp/insert-ID ordering,
   SQLite text-length accounting, source/age policy, and transaction rollback.
3. Add regression cases and a repeatable benchmark; compare the same workload
   before and after the change.
4. Run Go tests, race detection, vet, and production-style builds; publish a PR.

## The bottleneck

- **Confirmed: quadratic size-budget cleanup.** For N rows and K deletions,
  `Cleanup` recalculated the payload sum K+1 times, visiting approximately
  `(K+1)*N - K*(K+1)/2` rows. It also executed a selection and deletion per row.
  The store mutex is held throughout, so other operations on that store wait.
- **Follow-up: repeated query parsing.** `MatchesQuery` normalizes, splits, and
  applies a regular expression for each entry. The SQLite store filters entries,
  then `searchStore` applies the filters again. Compiling query predicates once
  could reduce this work, but requires query compatibility and lifecycle tests.
  This candidate has not been benchmarked and is outside this PR.
- **Follow-up: log-ID allocations.** Disassembly of `explorer.lineID` contains
  two `fmt.Sprintf` calls, interface conversions, `time.Time.Format`, and
  `runtime.stringtoslicebyte` before SHA-1. A direct encoding could reduce
  allocations, but must preserve IDs across retries and existing deployments.
  This candidate has not been benchmarked and is outside this PR.

## Baseline environment and profile evidence

The first question was whether the slowdown came from the Go binary itself or
from the amount of SQLite work performed during cleanup.

Environment: Go `go1.27.3-X:nodwarf5`, Linux/amd64, `GOAMD64=v1`, AMD Ryzen 5 PRO
3500U. Benchmarks use one Go CPU and an in-memory database; fixture restoration
is excluded from benchmark time and allocation counts. CPU profiles also include
fixture setup and are interpreted accordingly. These are synthetic maintenance
measurements, not end-to-end request throughput or disk-backed production claims.

The production-style stripped baseline Hub is 12,124,320 bytes. An otherwise
optimized binary retaining symbols is used for inspection; no `-N` or `-l` flags
are used. The largest executable symbol is SQLite's `_sqlite3VdbeExec` (78,311
bytes); symbol size alone is not evidence of a runtime bottleneck. The 32 MiB
`drbg.memory` symbol is BSS, not a 32 MiB on-disk code contribution.

In the baseline `(*Store).Cleanup.func1` disassembly:

- `0x96dfa0` calls `sqlitex.Execute` for the full payload sum.
- `0x96e164` calls `sqlitex.Execute` for the individual deletion.
- `0x96e0d3` branches back to `0x96df3a`, reentering the sum after deletion.

Addresses are specific to this build. A separate 5,000-row / 50%-retained CPU
profile collected 7.64 seconds of samples: `_sqlite3VdbeExec` accounted for 99.61%
including callees and `_lengthFunc` for 25.39% self time. `Cleanup` accounted for
98.17% including callees. This supports removing repeated row scans rather than
changing compiler flags or replacing the SQLite dependency.

## What changed

Instead of recalculating the total after every deletion, the implementation
calculates the total once after age/source cleanup, scans
oldest rows until their removal would satisfy the budget, resets that SELECT,
and deletes the inclusive `(timestamp_ns, insert_id)` prefix in one statement.
The boundary scan retains just one key in Go; it does not collect all IDs.
Payload visits become O(N + K), plus index maintenance for the K deletions.
No schema migration or persistent counters are required.

`EXPLAIN QUERY PLAN` from the bundled SQLite engine confirms:

```text
SCAN logs USING INDEX logs_timestamp_idx
SEARCH logs USING INDEX logs_timestamp_idx ((timestamp_ns,insert_id)<(?,?))
```

There is no temporary sort. SQLite's aggregate VM program contains `Rewind`,
five `length` calls, `AggStep`, and `Next`; this row loop now executes for the
total only once. The optimized machine code has no deletion-to-sum back edge.
The stripped Hub remains 12,124,320 bytes, so the gain comes from less runtime
work rather than a smaller executable.

## Benchmark results

I compared five separate process launches per revision, alternating
before/after, with one timed cleanup per sub-benchmark and no concurrent builds
or tests. Each fixture has 256 ASCII payload characters, a nine-character
summary, three JSON `null` strings, and monotonically increasing timestamps.
The 50% cases remove exactly half of the rows. Single-iteration runs include
cold statement preparation; the table reports medians rather than claiming
statistical significance for small differences.

| Rows | Retained | Before (median) | After (median) | Speedup |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 50% | 3.384 ms | 0.643 ms | 5.27x |
| 100 | 100% | 0.165 ms | 0.147 ms | 1.12x |
| 1,000 | 50% | 251.202 ms | 6.007 ms | 41.82x |
| 1,000 | 100% | 0.820 ms | 0.855 ms | 0.96x |
| 5,000 | 50% | 6381.351 ms | 33.602 ms | 189.91x |
| 5,000 | 100% | 4.776 ms | 4.728 ms | 1.01x |

For 5,000 rows / 50% retained, median Go allocations fall from 12,523 to 31 per
cleanup and allocated bytes from 184,888 to 5,176. The no-deletion path still
performs the same full-table sum; its small timing variations do not establish
a performance improvement or regression.

Raw samples: [before](retention-cleanup-before.txt) and
[after](retention-cleanup-after.txt).

## Compatibility and validation

The optimization changes how cleanup selects rows to delete. It should not
change which rows are retained, the order in which ties are resolved, or what
happens when a transaction fails.

The regression tests cover empty/disabled/exact budgets, complete deletion,
out-of-insertion-order timestamps and tied IDs, zero-length rows, all five
payload columns, NULLs, Unicode, embedded NUL, and combined age/size cleanup.
A deliberately failing DELETE trigger verifies rollback of both age and size
deletions and successful reuse of the connection afterward. Existing tests
continue to cover source/min retention boundaries.

SQLite `length(TEXT)` counts characters and stops at embedded NUL. This PR
preserves that existing accounting; it does not convert the budget to UTF-8 byte
length. Cleanup still holds the store mutex and performs a full payload scan
even when no deletion is needed. Eliminating that scan would require separately
maintained accounting and migration/reconciliation logic.

Passed: `go test ./...`, `go vet ./...`, `go test -race -p 1 ./...`, Go
formatting, `git diff --check`, `npm ci`, `npm run lint`, `npm run typecheck`,
`npm run build`, and `CGO_ENABLED=0` stripped Hub/Agent builds. Race testing used
`TMPDIR=/home/haturatu/.cache/caroline-performance-tests` after the default
`/tmp` tmpfs ran out of linker space; the complete rerun passed.

## Reproducing the measurements

Keep the Go toolchain, benchmark fixture, and build flags the same for both
revisions. Running the two benchmarks in parallel would make the results harder
to compare.

The benchmark-only commit `817b6ac` preserves the baseline implementation.
Build it and the PR head with the same benchmark file and toolchain. Keep artifacts
outside the repository:

```sh
mkdir -p /tmp/caroline-performance
# On 817b6ac use revision=before; on the PR head use revision=after.
revision=before
CGO_ENABLED=0 go build -trimpath -o /tmp/caroline-performance/caroline-$revision ./cmd/caroline
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /tmp/caroline-performance/caroline-release-$revision ./cmd/caroline
go tool nm -size -sort size /tmp/caroline-performance/caroline-$revision
go tool objdump -s 'caroline/internal/storage/sqlite.\(\*Store\).Cleanup.func1' /tmp/caroline-performance/caroline-$revision
go tool objdump -s 'caroline/internal/explorer.lineID' /tmp/caroline-performance/caroline-$revision
go test -c -o /tmp/caroline-performance/sqlite-$revision.test ./internal/storage/sqlite

# After building both revisions, run without concurrent builds or tests.
for sample in 1 2 3 4 5; do
  /tmp/caroline-performance/sqlite-before.test -test.run='^$' -test.bench=BenchmarkCleanupBudget -test.benchtime=1x -test.cpu=1
  /tmp/caroline-performance/sqlite-after.test -test.run='^$' -test.bench=BenchmarkCleanupBudget -test.benchtime=1x -test.cpu=1
done
/tmp/caroline-performance/sqlite-before.test -test.run='^$' -test.bench='BenchmarkCleanupBudget/rows=5000/retain=50%' -test.benchtime=1x -test.cpu=1 -test.cpuprofile=/tmp/caroline-performance/cleanup.cpu
go tool pprof -top /tmp/caroline-performance/sqlite-before.test /tmp/caroline-performance/cleanup.cpu
```
