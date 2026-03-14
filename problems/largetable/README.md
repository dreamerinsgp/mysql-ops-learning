# Problem 04: Large Table

## Scenario

Table grows too large. Full table scans, DDL (ALTER) takes forever or locks the table. Need to understand table sizes and consider partitioning or online DDL tools.

## Reproduction

```bash
go run ./cmd run 04-large-table reproduce
```

Creates table `_ops_learn_largetable` and inserts 100000 rows. Simulates a table that has grown large.

## Analyze

```bash
go run ./cmd run 04-large-table analyze
```

Queries `information_schema.TABLES` to show table sizes (data_length, index_length), row count, engine.

## Solution

1. **Partitioning**: split by range/hash for large tables
2. **Online DDL**: MySQL 8.0 ALTER with ALGORITHM=INPLACE, or use pt-online-schema-change, gh-ost
3. **Archiving**: move old data to archive tables
4. **Indexing**: add proper indexes to avoid full scans
5. **Monitor**: track table growth over time
