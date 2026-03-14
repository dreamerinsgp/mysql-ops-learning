# Problem 02: Slow Query Log Monitoring

## Scenario

Some queries are slow but you don't know which. Enable slow query log to capture and analyze them.

## Reproduction

```bash
go run ./cmd run 02-slow-log reproduce
```

Creates `_biz_orders_search`, inserts 150k rows. Runs 用户按手机号搜索订单 SQL (phone 无索引→全表扫描). 若 slow_log=ON 且 long_query_time≤2，该查询会进慢日志。

## Enable

```bash
go run ./cmd run 02-slow-log enable
```

Sets `slow_query_log=1` and `long_query_time=2` (queries > 2 seconds are logged). Checks current settings.

## Solution

1. Enable: `SET GLOBAL slow_query_log = 'ON';`
2. Set threshold: `SET GLOBAL long_query_time = 2;` (seconds)
3. Locate log file: `SHOW GLOBAL VARIABLES LIKE 'slow_query_log_file';`
4. Analyze with `pt-query-digest` or MySQL's built-in tools.
5. Add indexes, rewrite queries based on findings.
