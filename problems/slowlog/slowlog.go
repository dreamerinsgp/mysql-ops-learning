package slowlog

import (
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the slow-log problem tool: reproduce or enable
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "enable":
		enable()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or enable)", action)
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
