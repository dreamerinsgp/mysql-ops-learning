# Problem 06: Lock Wait Timeout

## Scenario

SaaS 平台：后台导出用户报表持锁 10 秒不提交，前台用户修改头像被阻塞，等待超 5 秒后报 Lock wait timeout exceeded。

## Reproduction

```bash
go run ./cmd run 06-lock-wait-timeout reproduce
```

Creates `users`, Conn1 持锁 10s，Conn2 模拟前台用户更新头像，等待超时。

## Solution

1. **Reduce lock hold time**: Commit or rollback quickly
2. **Tune innodb_lock_wait_timeout**: `SET GLOBAL innodb_lock_wait_timeout = 10;` (seconds)
3. **Find blocking transactions**: Query `information_schema.INNODB_LOCK_WAITS`, `INNODB_TRX`, `INNODB_LOCKS`
4. **Kill blocker**: `KILL <trx_mysql_thread_id>` if needed
