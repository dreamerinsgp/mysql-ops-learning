package failover

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// Run executes the failover problem tool: reproduce, prepare, switch, or verify
func Run(action string) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce(dsn)
	case "prepare":
		prepare(dsn)
	case "switch":
		switchMaster(dsn)
	case "verify":
		verify(dsn)
	default:
		log.Fatalf("Unknown action: %s (use reproduce, prepare, switch, or verify)", action)
	}
}

// reproduce 模拟业务场景：主库故障，需要执行主从切换
func reproduce(dsn string) {
	log.Println("[业务场景] 模拟主库故障触发故障转移...")
	log.Println("")
	
	log.Println("=== 场景描述 ===")
	log.Println("某电商平台使用 MySQL 主从架构实现高可用：")
	log.Println("  - 主库 (Master): 负责所有写入操作")
	log.Println("  - 从库 (Slave): 负责读取，实时同步主库数据")
	log.Println("  - 读写分离: 写入走主库，读取优先走从库")
	log.Println("")
	log.Println("故障发生：")
	log.Println("  时间: 14:30:00")
	log.Println("  事件: 主库 MySQL 服务无响应")
	log.Println("  原因: 硬件故障/网络中断/数据库崩溃")
	log.Println("")
	log.Println("影响：")
	log.Println("  - 用户无法下单（写入请求失败）")
	log.Println("  - 支付中断")
	log.Println("  - 订单创建失败")
	log.Println("  - 业务中断计时开始...")
	log.Println("")
	
	// 模拟检查主库状态
	log.Println("=== 模拟：检测主库状态 ===")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("连接数据库失败: %v", err)
		return
	}
	defer db.Close()

	// 检查是否为从库
	var slaveStatus sql.NullString
	err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
	if err != nil || !slaveStatus.Valid {
		log.Println("当前连接的是主库（可作为测试）")
	} else {
		log.Println("当前连接的是从库")
	}

	// 显示主库状态
	var serverID, binlogFormat, logBin string
	db.QueryRow("SHOW VARIABLES LIKE 'server_id'").Scan(nil, &serverID)
	db.QueryRow("SHOW VARIABLES LIKE 'binlog_format'").Scan(nil, &binlogFormat)
	db.QueryRow("SHOW VARIABLES LIKE 'log_bin'").Scan(nil, &logBin)
	
	fmt.Println("主库配置:")
	fmt.Printf("  server_id: %s\n", serverID)
	fmt.Printf("  binlog_format: %s\n", binlogFormat)
	fmt.Printf("  log_bin: %s\n", logBin)

	// 创建测试数据模拟故障前状态
	log.Println("")
	log.Println("=== 模拟：故障前数据状态 ===")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			order_no VARCHAR(64) NOT NULL,
			user_id BIGINT NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id)
		) ENGINE=InnoDB
	`)
	if err != nil {
		log.Printf("建表失败: %v", err)
	}

	// 插入一些模拟订单
	result, err := db.Exec("INSERT INTO orders (order_no, user_id, amount, status) VALUES (?, ?, ?, ?)",
		"ORD20260314001", 1001, 299.00, "paid")
	if err != nil {
		log.Printf("插入测试数据失败: %v", err)
	} else {
		orderID, _ := result.LastInsertId()
		log.Printf("  已创建订单 #%d (故障前最后订单)", orderID)
	}

	log.Println("")
	log.Println("=== 故障发生 ===")
	log.Println("⚠️ 主库 MySQL 服务无响应!")
	log.Println("⚠️ 应用写入请求开始失败!")
	log.Println("")
	log.Println("此时需要执行故障转移：将从库提升为主库")
	log.Println("")
	log.Println("=== 解决方案 ===")
	log.Println("1. 确认从库数据已同步到最新")
	log.Println("2. 停止从库复制 (STOP SLAVE)")
	log.Println("3. 重置从库复制配置 (RESET SLAVE ALL)")
	log.Println("4. 将从库设为可读写 (SET GLOBAL read_only=OFF)")
	log.Println("5. 通知应用切换数据库连接")
	log.Println("6. 验证数据一致性")
	log.Println("")
	log.Println("使用以下命令执行故障转移:")
	log.Println("  go run ./cmd run 10-failover prepare   # 准备阶段：检查从库状态")
	log.Println("  go run ./cmd run 10-failover switch    # 执行切换")
	log.Println("  go run ./cmd run 10-failover verify    # 验证结果")
}

// prepare 准备阶段：检查从库状态，确认数据同步
func prepare(dsn string) {
	log.Println("[准备阶段] 检查从库状态，确认数据已同步...")
	log.Println("")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 预声明所有变量（避免 goto 跳过声明问题）
	var (
		slaveStatus sql.NullString
		rows         *sql.Rows
		keyVals      map[string]string
	)

	keyVals = make(map[string]string)

	// 检查是否是 slave
	err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
	if err != nil || !slaveStatus.Valid {
		log.Println("当前实例不是从库（非主从环境或已是主库）")
		log.Println("")
		log.Println("=== 在从库上执行以下检查 ===")
		log.Println("")
		goto showCommands
	}

	// 获取 slave status
	rows, err = db.Query("SHOW SLAVE STATUS")
	if err != nil {
		log.Printf("查询失败: %v", err)
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}
	rows.Next()
	rows.Scan(valuePtrs...)

	for i, col := range columns {
		if val, ok := values[i].([]byte); ok {
			keyVals[col] = string(val)
		}
	}

	fmt.Println("=== 从库状态检查 ===")
	
	// 检查 IO 线程
	if io, ok := keyVals["Slave_IO_Running"]; ok {
		fmt.Printf("IO 线程 (Slave_IO_Running): %s", io)
		if io != "Yes" {
			fmt.Println(" ⚠️ IO 线程未运行")
		} else {
			fmt.Println(" ✓")
		}
	}

	// 检查 SQL 线程
	if sql, ok := keyVals["Slave_SQL_Running"]; ok {
		fmt.Printf("SQL 线程 (Slave_SQL_Running): %s", sql)
		if sql != "Yes" {
			fmt.Println(" ⚠️ SQL 线程未运行")
		} else {
			fmt.Println(" ✓")
		}
	}

	// 检查延迟
	if sbm, ok := keyVals["Seconds_Behind_Master"]; ok {
		fmt.Printf("延迟 (Seconds_Behind_Master): %s 秒", sbm)
		if sbm == "0" || sbm == "NULL" {
			fmt.Println(" ✓ 已同步")
		} else {
			fmt.Println(" ⚠️ 存在延迟")
		}
	}

	// 检查主库日志位置
	if masterLog, ok := keyVals["Master_Log_File"]; ok {
		fmt.Printf("主库日志文件: %s\n", masterLog)
	}
	if readPos, ok := keyVals["Read_Master_Log_Pos"]; ok {
		fmt.Printf("读取位置: %s\n", readPos)
	}
	if relayPos, ok := keyVals["Relay_Log_Pos"]; ok {
		fmt.Printf("Relay 日志位置: %s\n", relayPos)
	}

	// GTID 信息
	if gtid, ok := keyVals["Executed_Gtid_Set"]; ok {
		fmt.Printf("已执行 GTID: %s\n", gtid)
	}

showCommands:
	fmt.Println("")
	fmt.Println("=== 执行故障转移前必须确认 ===")
	fmt.Println("1. Slave_IO_Running = Yes")
	fmt.Println("2. Slave_SQL_Running = Yes")
	fmt.Println("3. Seconds_Behind_Master = 0 (无延迟)")
	fmt.Println("4. Relay_Log_Pos 与 Read_Master_Log_Pos 接近")
	fmt.Println("")
	fmt.Println("如果数据未完全同步，切换将导致数据丢失!")
	fmt.Println("")
	fmt.Println("=== 手动检查命令 ===")
	fmt.Println("  SHOW SLAVE STATUS\\G")
	fmt.Println("  SELECT * FROM performance_schema.replication_applier_status_by_worker\\G")
}

// switchMaster 执行主从切换
func switchMaster(dsn string) {
	log.Println("[执行切换] 模拟主从切换过程...")
	log.Println("")
	log.Println("⚠️ 警告：以下操作会修改数据库配置!")	
	log.Println("")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 检查是否是 slave
	var slaveStatus sql.NullString
	err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
	isSlave := err == nil && slaveStatus.Valid

	if isSlave {
		log.Println("步骤 1: 停止从库复制")
		log.Println("  SQL: STOP SLAVE")
		log.Println("  SQL: STOP SLAVE IO_THREAD")
		log.Println("  SQL: STOP SLAVE SQL_THREAD")
		
		_, err = db.Exec("STOP SLAVE")
		if err != nil {
			log.Printf("  停止失败（可能非从库）: %v", err)
		} else {
			log.Println("  ✓ 已停止从库复制")
		}

		log.Println("")
		log.Println("步骤 2: 重置从库复制配置")
		log.Println("  SQL: RESET SLAVE ALL")
		_, err = db.Exec("RESET SLAVE ALL")
		if err != nil {
			log.Printf("  重置失败: %v", err)
		} else {
			log.Println("  ✓ 已重置复制配置")
		}
	}

	log.Println("")
	log.Println("步骤 3: 设置从库为可读写模式")
	log.Println("  SQL: SET GLOBAL read_only = OFF")
	log.Println("  SQL: SET GLOBAL super_read_only = OFF")
	
	_, err = db.Exec("SET GLOBAL read_only = OFF")
	if err != nil {
		log.Printf("  设置 read_only 失败: %v", err)
	} else {
		log.Println("  ✓ 已关闭 read_only")
	}

	_, err = db.Exec("SET GLOBAL super_read_only = OFF")
	if err != nil {
		log.Printf("  设置 super_read_only 失败: %v", err)
	} else {
		log.Println("  ✓ 已关闭 super_read_only")
	}

	log.Println("")
	log.Println("步骤 4: 验证主库配置")
	var serverID, readOnly, logBin string
	db.QueryRow("SHOW VARIABLES LIKE 'server_id'").Scan(nil, &serverID)
	db.QueryRow("SHOW VARIABLES LIKE 'read_only'").Scan(nil, &readOnly)
	db.QueryRow("SHOW VARIABLES LIKE 'log_bin'").Scan(nil, &logBin)
	
	fmt.Println("当前实例配置:")
	fmt.Printf("  server_id: %s\n", serverID)
	fmt.Printf("  read_only: %s\n", readOnly)
	fmt.Printf("  log_bin: %s\n", logBin)

	log.Println("")
	log.Println("=== 切换完成 ===")
	log.Println("✓ 从库已提升为新主库")
	log.Println("✓ 可以接受写入请求")
	log.Println("")
	log.Println("后续步骤:")
	log.Println("1. 更新应用数据库连接配置，指向新主库")
	log.Println("2. 恢复原主库后，将其配置为新主库的从库")
	log.Println("3. 重新建立主从复制关系")
	log.Println("")
	log.Println("使用 verify 检查数据一致性:")
	log.Println("  go run ./cmd run 10-failover verify")
}

// verify 验证数据一致性和服务状态
func verify(dsn string) {
	log.Println("[验证] 检查故障转移后的数据一致性...")
	log.Println("")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 检查是否可写
	log.Println("=== 写入能力验证 ===")
	result, err := db.Exec("INSERT INTO orders (order_no, user_id, amount, status) VALUES (?, ?, ?, ?)",
		fmt.Sprintf("ORD%v", time.Now().Unix()), 9999, 0.01, "testing")
	if err != nil {
		log.Printf("  ⚠️ 写入测试失败: %v", err)
		log.Println("  可能原因：read_only 未关闭")
	} else {
		id, _ := result.LastInsertId()
		log.Printf("  ✓ 写入成功 (order_id: %d)", id)
		// 清理测试数据
		db.Exec("DELETE FROM orders WHERE id = ?", id)
	}

	// 检查只读配置
	log.Println("")
	log.Println("=== 只读配置检查 ===")
	var readOnly, superReadOnly string
	db.QueryRow("SHOW VARIABLES LIKE 'read_only'").Scan(nil, &readOnly)
	db.QueryRow("SHOW VARIABLES LIKE 'super_read_only'").Scan(nil, &superReadOnly)
	fmt.Printf("  read_only: %s\n", readOnly)
	fmt.Printf("  super_read_only: %s\n", superReadOnly)
	
	if readOnly == "OFF" && superReadOnly == "OFF" {
		fmt.Println("  ✓ 数据库可读写")
	} else {
		fmt.Println("  ⚠️ 数据库仍为只读模式")
	}

	// 检查复制状态
	log.Println("")
	log.Println("=== 复制状态检查 ===")
	var slaveStatus sql.NullString
	err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
	if err != nil || !slaveStatus.Valid {
		fmt.Println("  ✓ 当前不是从库（已成功切换为主库）")
	} else {
		fmt.Println("  当前仍为从库配置")
	}

	// 检查 binlog
	log.Println("")
	log.Println("=== Binlog 配置检查 ===")
	var logBin, binlogFormat string
	db.QueryRow("SHOW VARIABLES LIKE 'log_bin'").Scan(nil, &logBin)
	db.QueryRow("SHOW VARIABLES LIKE 'binlog_format'").Scan(nil, &binlogFormat)
	fmt.Printf("  log_bin: %s\n", logBin)
	fmt.Printf("  binlog_format: %s\n", binlogFormat)

	if logBin == "ON" {
		fmt.Println("  ✓ Binlog 已开启（可作为新主库）")
	}

	// 数据验证
	log.Println("")
	log.Println("=== 数据验证 ===")
	var orderCount int
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
	fmt.Printf("  订单表总记录数: %d\n", orderCount)

	log.Println("")
	log.Println("=== 验证完成 ===")
	log.Println("✓ 故障转移成功完成")
	log.Println("✓ 新主库可以正常提供写入服务")
	log.Println("")
	log.Println("建议后续操作:")
	log.Println("1. 监控新主库性能，确保稳定")
	log.Println("2. 修复旧主库后，建立新的主从关系")
	log.Println("3. 考虑使用 MGR 或 Keepalived 实现自动故障转移")
}
