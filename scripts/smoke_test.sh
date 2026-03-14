#!/usr/bin/env bash
# mysql-ops-learning 冒烟测试：在 MySQL 可用时运行每个问题的核心动作
set -e

BIN="${BIN:-./mysql-ops}"
if [[ ! -x "$BIN" ]]; then
  echo "Error: $BIN not found or not executable. Run 'go build -o mysql-ops ./cmd/main.go' first."
  exit 1
fi

if [[ -z "$MYSQL_DSN" ]]; then
  echo "Error: MYSQL_DSN not set"
  exit 1
fi

export MYSQL_DSN
export MYSQL_OPS_CI=1  # 缩短 lockwait 等耗时操作

run() {
  local problem=$1
  local action=$2
  echo "=== $problem $action ==="
  $BIN run "$problem" "$action" || { echo "FAIL: $problem $action"; exit 1; }
  echo "OK"
}

echo "Starting smoke tests..."

# 01: monitor 只读状态，不耗连接
run "01-max-connections" "monitor"

# 02: enable 设置慢日志
run "02-slow-log" "enable"

# 03: 先 reproduce 创建表，再 detect
run "03-large-transaction" "reproduce"
run "03-large-transaction" "detect"

# 04: 先 reproduce 创建表，再 analyze
run "04-large-table" "reproduce"
run "04-large-table" "analyze"

# 05: reproduce 可能触发死锁（一方回滚），analyze 查看状态
run "05-deadlock" "reproduce"
run "05-deadlock" "analyze"

# 06: reproduce 持锁 2s（CI 模式），另一连接超时
run "06-lock-wait-timeout" "reproduce"

# 07: 先 reproduce 建表，再 explain
run "07-index-misuse" "reproduce"
run "07-index-misuse" "explain"

echo ""
echo "All smoke tests passed."
