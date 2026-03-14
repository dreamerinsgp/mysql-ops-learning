# OpenClaw 最小验证指南

## Step 1：环境准备

### 1.1 检查 OpenClaw 是否安装

```bash
which openclaw || npm list -g openclaw
```

### 1.2 配置 workspace 指向 dex_full

编辑 `~/.openclaw/openclaw.json`，确保 agent workspace 包含本项目：

```json5
{
  "agent": {
    "workspace": "/home/ubuntu/dex_full"
  }
}
```

或使用相对路径（需在正确目录下运行）：

```json5
{
  "agent": {
    "workspace": "~/dex_full"  // 或你的实际路径
  }
}
```

### 1.3 启用 Webhook（可选，用于 Dashboard 触发）

在 `~/.openclaw/openclaw.json` 中增加：

```json5
{
  "hooks": {
    "enabled": true,
    "token": "your-secret-token",
    "path": "/hooks"
  }
}
```

### 1.4 启动 Gateway

```bash
openclaw gateway status   # 检查是否已运行
openclaw gateway          # 若未运行，启动（前台）
# 或
openclaw gateway start    # 若已安装为服务
```

---

## Step 2：方式 A - 通过 TUI 手动验证

### 2.1 启动 TUI

```bash
cd /home/ubuntu/dex_full
openclaw tui
```

### 2.2 发送测试消息

在 TUI 中输入（可替换为你想验证的问题）：

```
请使用 mysql-ops-case-gen 技能，根据以下描述生成第 8 个 MySQL 运维案例：

问题名称：主从复制延迟
业务场景：某订单系统采用主从架构，主库处理写请求，从库用于报表查询。大促期间主库写入量激增，从库出现严重延迟，Seconds_Behind_Master 持续 30 分钟以上，报表数据严重滞后。
现象：报表数据不是实时的；从库 lag 持续增大；DBA 监控显示 io_thread 或 sql_thread 落后。
技术点：binlog 传输、从库 apply、大事务导致单线程阻塞、并行复制
```

### 2.3 预期结果

Agent 应依次：
1. 读取现有 problems 和 mysql-cases 作为参考
2. 创建 `problems/replicationlag/replicationlag.go`（或类似命名）
3. 更新 `cmd/main.go`
4. 创建 `performance/mysql-cases/08-replication-lag.md`
5. 更新 `performance/backend/main.py`
6. 执行 `go build ./cmd` 验证编译

---

## Step 2：方式 B - 通过 Webhook 触发

### 2.1 调用 Webhook

```bash
# 替换 <GATEWAY_URL> 为实际地址（如 http://127.0.0.1:18789）
# 替换 <HOOK_TOKEN> 为 hooks.token 配置的值

curl -X POST "http://127.0.0.1:18789/hooks/agent" \
  -H "Authorization: Bearer YOUR_HOOK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "请使用 mysql-ops-case-gen 技能，根据以下描述生成第 8 个 MySQL 运维案例：\n\n问题名称：主从复制延迟\n业务场景：某订单系统采用主从架构，主库写、从库读报表。大促时从库延迟 30 分钟以上。\n现象：Seconds_Behind_Master 持续增大，报表数据滞后。\n技术点：binlog apply、大事务、并行复制",
    "name": "mysql-ops-case-gen",
    "sessionKey": "hook:mysql-ops:08",
    "deliver": false,
    "timeoutSeconds": 180
  }'
```

返回 `202` 表示已接受，Agent 在后台执行。

### 2.2 查看执行结果

- Webhook 为异步，需通过 sessions 查看
- 或在 workspace 检查是否生成了新文件
- 执行 `go build ./cmd` 验证

---

## Step 3：验证清单

完成后检查：

- [x] `mysql-ops-learning/problems/replicationlag/` 存在且 `.go` 可编译
- [x] `mysql-ops-learning/cmd/main.go` 已注册新 case (08-replication-lag)
- [x] `performance/mysql-cases/08-replication-lag.md` 已创建
- [x] `performance/backend/main.py` 中 `MYSQL_OPS_PROBLEMS`、`MYSQL_OPS_PROBLEM_DIRS` 已更新
- [x] `go build -o x ./cmd` 通过

---

---

## Step 2：Dashboard 接入（已完成）

在 Performance Dashboard 中：

1. **Infra Config** → 新增「4. OpenClaw」：
   - OpenClaw Gateway URL：`http://127.0.0.1:18789`
   - Hooks Token：与 `~/.openclaw/openclaw.json` 中 `hooks.token` 一致

2. **MySQL Ops Tab** → 点击「+ 添加新问题（AI 生成）」：
   - 只需填写问题名称（如「binlog 过大」）
   - 提交后调用 `POST /api/mysql-ops/generate`，由后端转发到 OpenClaw Webhook
   - 提示「Agent 已启动，稍后刷新页面查看新案例」

3. **查看新案例**：问题列表从 `performance/config/mysql_ops_problems.json` 动态加载，**无需重启后端**。Agent 完成后，点击「刷新列表」或切换到其他 Tab 再切回即可看到新案例。

4. **注意**：后端需能访问 OpenClaw Gateway。若 Dashboard 与 OpenClaw 不在同一台机，需配置可访问的 URL（如 Tailscale 内网地址）。

---

## 验证结果（2026-03-14）

**Step 1 最小验证已完成：**

1. **Webhook 触发** ✓：`POST /hooks/agent` 返回 202，Agent 异步执行
2. **Agent 生成 Go 代码** ✓：自动创建了 `problems/replicationlag/replicationlag.go`，含 reproduce / monitor / detect
3. **完整链路**：Agent 生成了核心 Go 包，其余部分（cmd/main.go、mysql-cases、backend）已人工补全
4. **Skill 生效** ✓：`skills/load/extraDirs` 加载了 `dex_full/skills`，mysql-ops-case-gen 被正确引用

**改进建议：**
- 在 Skill 中强调「必须完成全部 6 步，包括更新 main.go 和 backend」
- 可将生成任务拆为两段：先生成 Go+md，再触发「请完成注册步骤 3-5」
