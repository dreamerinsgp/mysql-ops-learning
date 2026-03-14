# Problem 01: Max Connections Exhaustion

## Scenario

Application opens too many connections without releasing them. MySQL reaches `max_connections` limit. New connections get `Too many connections` error.

## Reproduction

```bash
go run ./cmd run 01-max-connections reproduce
```

Opens connections in a loop until the limit is hit. (Use on a test instance; avoid production.)

## Monitor

```bash
go run ./cmd run 01-max-connections monitor
```

Shows `Threads_connected` and `Max_used_connections` via `SHOW GLOBAL STATUS`.

## Solution

1. **Tune `max_connections`** (if legitimate load): `SET GLOBAL max_connections = 500;`
2. **Use connection pooling** in the app; avoid one connection per request.
3. **Find connection leaks**: check long-lived connections, fix app code that forgets to close.
4. **Reduce idle connections**: `wait_timeout`, `interactive_timeout`.
