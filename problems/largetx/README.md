# Problem 03: Large Transaction

## Scenario

积分商城周年庆：运营批量给 10 万用户发积分，单事务内更新全部，持锁数分钟，阻塞用户登录、下单、查余额。

## Reproduction

```bash
go run ./cmd run 03-large-transaction reproduce
```

Creates `user_points` (积分表) and runs a single transaction updating 10000 rows—simulates 运营批量发积分导致长事务持锁.

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
