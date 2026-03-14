# Problem 02: Slow Query Log Monitoring

## Scenario

Some queries are slow but you don't know which. Enable slow query log to capture and analyze them.

## Reproduction

```bash
go run ./cmd run 02-slow-log reproduce
```

Runs `SELECT SLEEP(5)` to produce an intentionally slow query. Ensure slow_log is enabled and long_query_time is set appropriately first.

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
