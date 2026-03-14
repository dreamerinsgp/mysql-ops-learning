# Problem 03: Large Transaction

## Scenario

A single transaction updates/inserts too many rows. It holds locks for a long time, blocks other queries, and can cause replication lag and undo log bloat.

## Reproduction

```bash
go run ./cmd run 03-large-transaction reproduce
```

Creates table `_ops_learn_largetx` and runs a single transaction that updates 10000 rows. Simulates bad batch pattern.

## Detect

```bash
go run ./cmd run 03-large-transaction detect
```

Queries `information_schema.INNODB_TRX` to show long-running transactions (trx_started, rows modified, etc.).

## Solution

1. **Split into batches**: e.g. UPDATE 1000 rows per transaction, commit, repeat
2. **Avoid long-held locks**: minimize work inside BEGIN...COMMIT
3. **Use chunked processing**: process in smaller chunks with sleep between if needed
4. **Monitor**: detect long-running tx early via INNODB_TRX
