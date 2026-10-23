# Retention cleanup performance investigation

## Scope and plan

Investigated commit `02e372a` using a Linux/amd64 Hub binary, Go symbol and
instruction inspection, and a repeatable SQLite cleanup benchmark. The working
plan, established after collecting the baseline, is:

1. Replace repeated full-table payload sums and individual deletions with one
   sum, one oldest-first boundary scan, and one indexed prefix deletion.
2. Preserve existing retention semantics, including timestamp/insert-ID ordering,
   SQLite text-length accounting, source/age policy, and transaction rollback.
3. Add regression cases and a repeatable benchmark; compare the same workload
   before and after the change.
4. Run Go tests, race detection, vet, and production-style builds; publish a PR.

## Findings

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

## Baseline binary and profile evidence

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

## Results

Pending implementation and comparison.

## Reproduction

Build each revision with the same benchmark file and toolchain. Keep artifacts
outside the repository:

```sh
mkdir -p /tmp/caroline-performance
CGO_ENABLED=0 go build -trimpath -o /tmp/caroline-performance/caroline ./cmd/caroline
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /tmp/caroline-performance/caroline-release ./cmd/caroline
go tool nm -size -sort size /tmp/caroline-performance/caroline
go tool objdump -s 'caroline/internal/storage/sqlite.\(\*Store\).Cleanup.func1' /tmp/caroline-performance/caroline
go tool objdump -s 'caroline/internal/explorer.lineID' /tmp/caroline-performance/caroline
go test -c -o /tmp/caroline-performance/sqlite.test ./internal/storage/sqlite
/tmp/caroline-performance/sqlite.test -test.run='^$' -test.bench=BenchmarkCleanupBudget -test.benchtime=1x -test.count=3 -test.cpu=1
/tmp/caroline-performance/sqlite.test -test.run='^$' -test.bench='BenchmarkCleanupBudget/rows=5000/retain=50%' -test.benchtime=1x -test.cpu=1 -test.cpuprofile=/tmp/caroline-performance/cleanup.cpu
go tool pprof -top /tmp/caroline-performance/sqlite.test /tmp/caroline-performance/cleanup.cpu
```
