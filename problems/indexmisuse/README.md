# Problem 07: Index Misuse

## Scenario

Query does full table scan because there is no index or wrong index is used. EXPLAIN shows `type=ALL` or high `rows` scanned.

## Reproduction

```bash
go run ./cmd run 07-index-misuse reproduce
```

Creates table `_ops_learn_index` with data, no index on filtered column. Runs a query that triggers full scan.

## Explain

```bash
go run ./cmd run 07-index-misuse explain
```

Runs `EXPLAIN` on the problematic query to show access type, rows, key (or lack of).

## Solution

1. **Add index**: `CREATE INDEX idx_col ON table(col);`
2. **Avoid SELECT ***: Select only needed columns
3. **Use covering index**: Include all needed columns in index
4. **Analyze EXPLAIN**: type=ref/range good, type=ALL bad; check rows estimate
