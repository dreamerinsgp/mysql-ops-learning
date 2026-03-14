# Problem 06: Lock Wait Timeout

## Scenario

Transaction A holds a row lock. Transaction B tries to update the same row and waits. If wait exceeds `innodb_lock_wait_timeout` (default 50s), B gets "Lock wait timeout exceeded".

## Reproduction

```bash
go run ./cmd run 06-lock-wait-timeout reproduce
```

Spawns two connections: one holds a lock (BEGIN + UPDATE, no commit), the other tries to update the same row and blocks until timeout.

## Solution

1. **Reduce lock hold time**: Commit or rollback quickly
2. **Tune innodb_lock_wait_timeout**: `SET GLOBAL innodb_lock_wait_timeout = 10;` (seconds)
3. **Find blocking transactions**: Query `information_schema.INNODB_LOCK_WAITS`, `INNODB_TRX`, `INNODB_LOCKS`
4. **Kill blocker**: `KILL <trx_mysql_thread_id>` if needed
