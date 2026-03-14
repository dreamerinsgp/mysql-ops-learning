# MySQL Ops Learning Project

A Go-based project for learning common MySQL operations issues. Each problem is represented in a Git branch with tools to reproduce, monitor, and fix it.

## Prerequisites

- Go 1.21+
- MySQL (e.g. Aliyun RDS) with network access

## Setup

1. Copy `.env.example` to `.env`
2. Fill `MYSQL_DSN` with your Aliyun MySQL connection string
3. Load env: `source .env` or `export $(cat .env | xargs)`
4. Add your IP to RDS whitelist

## Branch Map

| Branch | Problem |
|--------|---------|
| `main` | Full project (all problems) |
| `problem/01-max-connections` | Max connection exhaustion |
| `problem/02-slow-log` | Slow query log monitoring |
| `problem/03-large-transaction` | Large transaction |
| `problem/04-large-table` | Large table issues |

## Usage

```bash
# Max connections: reproduce exhaustion, or monitor status
go run ./cmd run 01-max-connections reproduce
go run ./cmd run 01-max-connections monitor

# Slow log: reproduce slow query, or enable slow log
go run ./cmd run 02-slow-log reproduce
go run ./cmd run 02-slow-log enable

# Large transaction: reproduce large tx, or detect long-running
go run ./cmd run 03-large-transaction reproduce
go run ./cmd run 03-large-transaction detect

# Large table: reproduce, or analyze table sizes
go run ./cmd run 04-large-table reproduce
go run ./cmd run 04-large-table analyze
```

## Structure

```
mysql-ops-learning/
├── cmd/main.go       # CLI entry, dispatches to problems
├── pkg/db/           # Shared DB connection
└── problems/         # One dir per problem
    ├── 01-max-connections/
    ├── 02-slow-log/
    ├── 03-large-transaction/
    └── 04-large-table/
```
