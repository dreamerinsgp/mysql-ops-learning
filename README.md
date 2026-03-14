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
| `problem/05-deadlock` | Deadlock |
| `problem/06-lock-wait-timeout` | Lock wait timeout |
| `problem/07-index-misuse` | Index misuse |

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

# Deadlock, lock wait, index misuse
go run ./cmd run 05-deadlock reproduce
go run ./cmd run 05-deadlock analyze
go run ./cmd run 06-lock-wait-timeout reproduce
go run ./cmd run 07-index-misuse reproduce
go run ./cmd run 07-index-misuse explain
```

## Performance Dashboard Integration

The Performance dashboard (DEX Ops Dashboard) can run these tools from its **MySQL Ops** tab:

1. Save Infra config with your Aliyun MySQL (host, port, user, password, database)
2. Ensure `mysql-ops-learning` is at `performance/../mysql-ops-learning` (sibling directory)
3. Ensure Go is installed on the Performance server
4. Open MySQL Ops tab, select problem + action, click Run

The tools run on the Performance server and connect to your MySQL. Add the Performance server IP to your RDS whitelist.

## CI / Testing

GitHub Actions 在 push/PR 到 `mysql-ops-learning/` 时自动运行：

- **build-and-test**：编译、单元测试（无需 MySQL）
- **smoke-test**：MySQL 8.0 服务容器，对每个问题执行核心动作冒烟测试

本地运行：

```bash
make build      # 编译
make test       # 单元测试
make smoke      # 冒烟测试（需 MYSQL_DSN）
```

## Structure

```
mysql-ops-learning/
├── cmd/main.go       # CLI entry, dispatches to problems
├── scripts/smoke_test.sh  # 冒烟测试脚本
├── pkg/db/           # Shared DB connection
└── problems/         # One package per problem (Go: no hyphens in import path)
    ├── conn/         # 01-max-connections
    ├── slowlog/      # 02-slow-log
    ├── largetx/      # 03-large-transaction
    ├── largetable/   # 04-large-table
    ├── deadlock/     # 05-deadlock
    ├── lockwait/     # 06-lock-wait-timeout
    └── indexmisuse/  # 07-index-misuse
```
