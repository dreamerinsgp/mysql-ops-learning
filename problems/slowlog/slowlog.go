package slowlog

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"mysql-ops-learning/pkg/db"
)

// Run executes the slow-log problem tool: reproduce, enable, or view
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "enable":
		enable()
	case "view":
		view()
	default:
		log.Fatalf("Unknown action: %s (use reproduce, enable, or view)", action)
	}
}

// reproduce 模拟业务场景：O2O 用户按手机号搜索历史订单，因 phone 无索引导致全表扫描变慢
func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// 建表：模拟订单表（无 phone 索引时搜索会全表扫描）
	_, _ = database.Exec("DROP TABLE IF EXISTS _biz_orders_search")
	_, err = database.Exec(`
		CREATE TABLE _biz_orders_search (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			order_no VARCHAR(32) NOT NULL,
			user_phone VARCHAR(20) NOT NULL,
			amount DECIMAL(10,2),
			status TINYINT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 插入约 3 万行（控制耗时 <30s，仍足以触发慢日志）
	log.Println("[业务场景] 用户在「我的订单」按手机号搜索，接口执行以下 SQL（phone 无索引 → 全表扫描）")
	log.Println("  SELECT * FROM _biz_orders_search WHERE user_phone='13800138000' ORDER BY created_at DESC LIMIT 20")
	const totalRows = 30000
	const batch = 5000
	targetPhone := "13800138000"
	for i := 0; i < totalRows; i += batch {
		tx, _ := database.Begin()
		for j := 0; j < batch && i+j < totalRows; j++ {
			phone := fmt.Sprintf("138%08d", (i+j)%100000)
			tx.Exec("INSERT INTO _biz_orders_search (order_no, user_phone, amount, status) VALUES (?, ?, ?, ?)",
				fmt.Sprintf("ORD%010d", i+j), phone, 99.9, 1)
		}
		tx.Commit()
		if (i+batch)%10000 == 0 || i+batch >= totalRows {
			log.Printf("  已插入 %d 行...", min(i+batch, totalRows))
		}
	}
	database.Exec("INSERT INTO _biz_orders_search (order_no, user_phone, amount, status) VALUES (?, ?, ?, ?)",
		"ORD13800138000", targetPhone, 199, 1)
	log.Printf("已插入 %d 行。执行慢查询...", totalRows+1)

	_, err = database.Exec("SELECT * FROM _biz_orders_search WHERE user_phone = ? ORDER BY created_at DESC LIMIT 20", targetPhone)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("完毕。若 slow_query_log=ON 且 long_query_time<=2，该查询应出现在慢日志中。")
}

func enable() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// 尝试 SET GLOBAL，云 RDS / 受限账户通常无 SUPER 权限
	setFailed := false
	if _, err := database.Exec("SET GLOBAL slow_query_log = 'ON'"); err != nil {
		setFailed = true
		log.Printf("SET GLOBAL slow_query_log 失败: %v", err)
	} else {
		log.Println("slow_query_log = ON")
	}
	if _, err := database.Exec("SET GLOBAL long_query_time = 2"); err != nil {
		setFailed = true
		log.Printf("SET GLOBAL long_query_time 失败: %v", err)
	} else {
		log.Println("long_query_time = 2 seconds")
	}
	if setFailed {
		fmt.Println()
		log.Println("提示：当前账户无 SUPER/SYSTEM_VARIABLES_ADMIN 权限，无法 SET GLOBAL。")
		log.Println("云 RDS（阿里云/腾讯云/AWS 等）需在控制台修改参数组，或联系 DBA 用高权限账号设置。")
		log.Println("若 slow_query_log 已为 ON，可直接分析慢日志文件。")
		fmt.Println()
	}

	// 显示当前配置（SHOW 通常可读）
	var name, val string
	row := database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'slow_query_log'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("slow_query_log: %s\n", val)
	}
	row = database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'long_query_time'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("long_query_time: %s\n", val)
	}
	row = database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'slow_query_log_file'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("slow_query_log_file: %s\n", val)
	}
}

// view 查看慢日志：优先从 mysql.slow_log 表读取（log_output=TABLE），否则尝试读文件或输出路径
func view() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	var logOutput, logFile string
	var n, v string
	if database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'log_output'").Scan(&n, &v) == nil {
		logOutput = v
	}
	if database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'slow_query_log_file'").Scan(&n, &v) == nil {
		logFile = v
	}

	// log_output 可为 TABLE, FILE, 或 TABLE,FILE
	if strings.Contains(strings.ToUpper(logOutput), "TABLE") {
		rows, err := database.Query(`
			SELECT start_time, user_host, query_time, lock_time, rows_sent, rows_examined, db, LEFT(sql_text, 500) as sql_text
			FROM mysql.slow_log
			ORDER BY start_time DESC
			LIMIT 50
		`)
		if err != nil {
			log.Printf("查询 mysql.slow_log 失败（可能未启用 TABLE 输出）: %v", err)
			fmt.Printf("\n慢日志文件路径: %s\n", logFile)
			log.Println("请在 MySQL 服务器上执行: tail -100 " + logFile)
			return
		}
		defer rows.Close()
		fmt.Println("=== mysql.slow_log 最近 50 条 ===")
		for rows.Next() {
			var startTime, userHost, queryTime, lockTime, rowsSent, rowsExamined, db, sqlText sql.NullString
			if rows.Scan(&startTime, &userHost, &queryTime, &lockTime, &rowsSent, &rowsExamined, &db, &sqlText) != nil {
				continue
			}
			fmt.Printf("\n--- %s | query_time=%s lock_time=%s rows=%s/%s | db=%s ---\n",
				nullStr(startTime), nullStr(queryTime), nullStr(lockTime), nullStr(rowsSent), nullStr(rowsExamined), nullStr(db))
			fmt.Println(nullStr(sqlText))
		}
		return
	}

	// FILE 模式：尝试在本机读取（仅当 MySQL 与应用同机时可行）
	if logFile != "" {
		if data, err := os.ReadFile(logFile); err == nil {
			lines := strings.Split(string(data), "\n")
			n := len(lines)
			if n > 100 {
				lines = lines[n-100:]
			}
			fmt.Println("=== slow_query_log_file 末尾内容 ===")
			fmt.Println(strings.Join(lines, "\n"))
			return
		}
		fmt.Printf("慢日志文件路径: %s\n", logFile)
		log.Println("无法直接读取（MySQL 可能在其他主机）。请在 MySQL 服务器上执行:")
		fmt.Printf("  tail -100 %s\n", logFile)
	} else {
		log.Println("slow_query_log_file 为空，请先开启慢日志。")
	}
}

func nullStr(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}
