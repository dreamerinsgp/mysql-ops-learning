package replicationlag

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// Run executes the replication lag problem tool: reproduce, monitor, or detect
func Run(action string) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce(dsn)
	case "monitor":
		monitor(dsn)
	case "detect":
		detect(dsn)
	default:
		log.Fatalf("Unknown action: %s (use reproduce, monitor, or detect)", action)
	}
}

// reproduce 模拟业务场景：大促时主库产生大量写入，从库应用 binlog 缓慢导致延迟
func reproduce(dsn string) {
	log.Println("[业务场景] 模拟大促：主库大量写入，从库复制延迟...")
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接主库失败: %v", err)
	}
	defer db.Close()

	// 创建订单表（如果不存在）
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			order_no VARCHAR(64) NOT NULL,
			user_id BIGINT NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB
	`)
	if err != nil {
		log.Printf("建表失败（可忽略）: %v", err)
	}

	// 模拟大量写入：分批插入模拟大促订单
	log.Println("  模拟大事务：分批插入 25000 条订单记录...")
	start := time.Now()
	const totalRows, batchSize = 25000, 5000
	for i := 0; i < totalRows; i += batchSize {
		tx, errTx := db.Begin()
		if errTx != nil {
			log.Printf("Begin 失败: %v", errTx)
			break
		}
		for j := 0; j < batchSize && i+j < totalRows; j++ {
			tx.Exec("INSERT INTO orders (order_no, user_id, amount, status, created_at) VALUES (?, ?, ?, ?, NOW())",
				fmt.Sprintf("ORD%010d", i+j), (i+j)%10000, 100+float64((i+j)%500), "pending")
		}
		if errTx := tx.Commit(); errTx != nil {
			log.Printf("Commit 失败（模拟环境可能无主从）: %v", errTx)
			log.Println("  注意：此问题需要主从环境才能完整复现")
			break
		}
		if (i+batchSize)%10000 == 0 || i+batchSize >= totalRows {
			log.Printf("  已插入 %d 行...", min(i+batchSize, totalRows))
		}
	}
	log.Printf("  完成：耗时 %v", time.Since(start))

	// 记录当前时间点（模拟主库写入完成的时间戳）
	log.Println("  记录主库写入完成时间点...")
	log.Println("")
	log.Println("=== 场景说明 ===")
	log.Println("主库：大促期间瞬间写入 50000 条订单")
	log.Println("从库：使用默认单线程复制，binlog_apply 缓慢")
	log.Println("结果：从库延迟持续增大，报表数据滞后 30+ 分钟")
	log.Println("")
	log.Println("=== 解决方案 ===")
	log.Println("1. 开启并行复制 (slave_parallel_workers)")
	log.Println("2. 拆分为小事务，避免单次过大事务")
	log.Println("3. 调整 slave_parallel_type=LOGICAL_CLOCK")
	log.Println("4. 考虑使用 MGR 或 GTID 提升复制效率")
}

// monitor 监控从库复制延迟状态
func monitor(dsn string) {
	log.Println("[监控] 检查从库复制延迟状态...")
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 检查是否是 slave
	var slaveStatus sql.NullString
	err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
	if err != nil || !slaveStatus.Valid {
		log.Println("当前环境未配置主从复制（非从库）")
		log.Println("在从库上运行此命令查看：")
		log.Println("  SHOW SLAVE STATUS\\G")
		log.Println("  关键指标：")
		log.Println("    - Seconds_Behind_Master: 延迟秒数")
		log.Println("    - Slave_IO_Running: IO 线程状态")
		log.Println("    - Slave_SQL_Running: SQL 线程状态")
		log.Println("    - Relay_Log_Pos: relay log 位置")
		return
	}

	// 获取完整的 slave status
	rows, err := db.Query("SHOW SLAVE STATUS")
	if err != nil {
		log.Printf("查询失败: %v", err)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Printf("获取列失败: %v", err)
		return
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		
		// 打印关键指标
		keyVals := make(map[string]string)
		for i, col := range columns {
			if val, ok := values[i].([]byte); ok {
				keyVals[col] = string(val)
			}
		}
		
		fmt.Println("=== 从库复制状态 ===")
		if sbm, ok := keyVals["Seconds_Behind_Master"]; ok {
			fmt.Printf("延迟 (Seconds_Behind_Master): %s 秒\n", sbm)
		}
		if io, ok := keyVals["Slave_IO_Running"]; ok {
			fmt.Printf("IO 线程状态: %s\n", io)
		}
		if sql, ok := keyVals["Slave_SQL_Running"]; ok {
			fmt.Printf("SQL 线程状态: %s\n", sql)
		}
		if lag, ok := keyVals["Slave_SQL_Relay_Log_Pos"]; ok {
			fmt.Printf("SQL Relay 位置: %s\n", lag)
		}
		if master, ok := keyVals["Master_Host"]; ok {
			fmt.Printf("主库地址: %s\n", master)
		}
	}
}

// detect 检测可能导致复制延迟的配置问题
func detect(dsn string) {
	log.Println("[检测] 检测可能导致复制延迟的配置...")
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 检查并行复制配置
	fmt.Println("=== 复制相关配置 ===")
	
	var parallelWorkers int
	err = db.QueryRow("SHOW VARIABLES LIKE 'slave_parallel_workers'").Scan(nil, &parallelWorkers)
	if err != nil {
		fmt.Println("slave_parallel_workers: 未配置或不支持 (默认 0，单线程)")
	} else {
		fmt.Printf("slave_parallel_workers: %d\n", parallelWorkers)
		if parallelWorkers == 0 {
			fmt.Println("  ⚠️ 警告：未开启并行复制，建议设置为 4-8")
		}
	}

	var parallelType string
	err = db.QueryRow("SHOW VARIABLES LIKE 'slave_parallel_type'").Scan(nil, &parallelType)
	if err == nil {
		fmt.Printf("slave_parallel_type: %s\n", parallelType)
	}

	var logBin string
	err = db.QueryRow("SHOW VARIABLES LIKE 'log_bin'").Scan(nil, &logBin)
	if err == nil {
		fmt.Printf("log_bin: %s (是否开启 binlog)\n", logBin)
	}

	var binlogFormat string
	err = db.QueryRow("SHOW VARIABLES LIKE 'binlog_format'").Scan(nil, &binlogFormat)
	if err == nil {
		fmt.Printf("binlog_format: %s\n", binlogFormat)
	}

	// 检查从库状态
	fmt.Println("")
	fmt.Println("=== 从库状态 ===")
	rows, err := db.Query("SHOW SLAVE STATUS")
	if err != nil {
		fmt.Println("非从库环境，无法检测")
		return
	}
	defer rows.Close()

	hasData := false
	for rows.Next() {
		hasData = true
		columns, _ := rows.Columns()
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		
		keyVals := make(map[string]string)
		for i, col := range columns {
			if val, ok := values[i].([]byte); ok {
				keyVals[col] = string(val)
			}
		}
		
		if sbm, ok := keyVals["Seconds_Behind_Master"]; ok && sbm != "NULL" {
			fmt.Printf("当前延迟: %s 秒\n", sbm)
		}
		if io, ok := keyVals["Slave_IO_Running"]; ok {
			fmt.Printf("IO 线程: %s\n", io)
		}
		if sql, ok := keyVals["Slave_SQL_Running"]; ok {
			fmt.Printf("SQL 线程: %s\n", sql)
		}
	}
	
	if !hasData {
		fmt.Println("未检测到从库状态（非从库环境）")
	}

	fmt.Println("")
	fmt.Println("=== 优化建议 ===")
	fmt.Println("1. 开启并行复制: SET GLOBAL slave_parallel_workers = 4")
	fmt.Println("2. 设置并行类型: SET GLOBAL slave_parallel_type = 'LOGICAL_CLOCK'")
	fmt.Println("3. 大事务拆分: 单次事务不超过 1000 行")
	fmt.Println("4. 启用 GTID: SET GLOBAL gtid_mode = ON")
}
