# Problem 07: Index Misuse

## Scenario

Query does full table scan because there is no index or wrong index is used. EXPLAIN shows `type=ALL` or high `rows` scanned.

## Reproduction

```bash
go run ./cmd run 07-index-misuse reproduce
```

Creates `orders` (外卖订单表)，phone 列无索引。用户按手机号查订单触发全表扫描，接口超时。

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
