# Problem 05: Deadlock

## Scenario

Concurrent transactions acquire locks in different order, causing deadlock. MySQL detects and rolls back one transaction.

## Reproduction

```bash
go run ./cmd run 05-deadlock reproduce
```

Spawns two goroutines that update rows in different order (A then B vs B then A), triggering a deadlock.

## Analyze

```bash
go run ./cmd run 05-deadlock analyze
```

Shows `SHOW ENGINE INNODB STATUS` output (LATEST DETECTED DEADLOCK section).

## Solution

1. **Lock ordering**: Always acquire locks in the same order (e.g. lower ID first)
2. **Retry logic**: On deadlock (err 1213), retry the transaction
3. **Keep transactions short**: Reduce lock hold time
4. **Index design**: Ensure consistent access pattern via indexes
