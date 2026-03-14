# Problem 05: Deadlock

## Scenario

钱包应用：用户 A 转给 B 100 元，同时用户 B 转给 A 50 元。两事务加锁顺序相反（A→B vs B→A），触发死锁。

## Reproduction

```bash
go run ./cmd run 05-deadlock reproduce
```

Creates `accounts`, two goroutines simulate A→B and B→A transfers with opposite lock order, triggering deadlock.

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
