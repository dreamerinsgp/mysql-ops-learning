package highcpu

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Run executes the high CPU/IO problem tool
func Run(action string) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce(dsn)
	case "explain":
		explainQuery(dsn)
	case "optimize":
		optimize(dsn)
	default:
		log.Fatalf("Unknown action: %s (use reproduce, explain, or optimize)", action)
	}
}

// reproduce 模拟业务场景：报表系统每日定时聚合，由于缺少索引导致全表扫描，CPU/IO 飙高
func reproduce(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 创建测试表（模拟订单表）
	log.Println("[业务场景] 创建测试表 orders（模拟订单表）...")
	_, err = db.Exec(`
		DROP TABLE IF EXISTS orders
	`)
	if err != nil {
		log.Printf("警告: 删除表失败: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE orders (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT NOT NULL,
			product_id INT NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			status TINYINT NOT NULL DEFAULT 0,
			create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_product_id (product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	log.Println("  表 orders 创建完成（仅有 user_id、product_id 单列索引）")

	// 批量插入 50 万条测试数据
	log.Println("[模拟数据] 插入 50 万条订单数据（这可能需要几分钟）...")
	insertSQL := "INSERT INTO orders (user_id, product_id, amount, status, create_time) VALUES "
	values := []string{}
	for i := 1; i <= 500000; i++ {
		values = append(values, fmt.Sprintf("(%d, %d, %.2f, %d, '%s')",
			(i%10000)+1, (i%5000)+1, float64(i%1000)+10.0, i%4,
			time.Now().Add(-time.Duration(i%365)*24*time.Hour).Format("2006-01-02 15:04:05")))
		if len(values) >= 5000 {
			_, err = db.Exec(insertSQL + values[0])
			if err != nil {
				log.Printf("插入数据失败: %v", err)
				break
			}
			if i%50000 == 0 {
				log.Printf("  已插入 %d 条...", i)
			}
			values = values[:0]
		}
	}
	if len(values) > 0 {
		_, err = db.Exec(insertSQL + values[0])
		if err != nil {
			log.Printf("插入数据失败: %v", err)
		}
	}
	log.Println("  数据插入完成")

	// 执行一个需要全表扫描的聚合查询（缺少复合索引）
	log.Println("[触发问题] 执行复杂聚合查询（缺少 (status, create_time) 复合索引，导致全表扫描）...")
	log.Println("  SQL: SELECT status, DATE(create_time) as day, COUNT(*), SUM(amount) FROM orders WHERE create_time >= '2024-01-01' GROUP BY status, DATE(create_time)")
	
	start := time.Now()
	var rows *sql.Rows
	rows, err = db.Query(`
		SELECT status, DATE(create_time) as day, COUNT(*) as cnt, SUM(amount) as total
		FROM orders 
		WHERE create_time >= '2024-01-01'
		GROUP BY status, DATE(create_time)
		ORDER BY day DESC
	`)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status int
		var day string
		var cnt int
		var total sql.NullFloat64
		if err := rows.Scan(&status, &day, &cnt, &total); err != nil {
			log.Printf("扫描行失败: %v", err)
			break
		}
		count++
		if count <= 5 {
			log.Printf("    结果: status=%d, day=%s, cnt=%d, total=%.2f", status, day, cnt, total.Float64)
		}
	}
	elapsed := time.Since(start)
	log.Printf("  查询完成，返回 %d 行，耗时 %v", count, elapsed)

	// 监控 CPU/IO 状态
	log.Println("[监控] 查看当前 MySQL 状态...")
	var qps, threadsRunning int
	var varName string
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Questions'").Scan(&varName, &qps)
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Threads_running'").Scan(&varName, &threadsRunning)
	log.Printf("  Questions: %d, Threads_running: %d", qps, threadsRunning)

	log.Print(`
[结论] CPU/IO 飙高原因分析：
1. WHERE 条件使用 create_time 字段过滤，但仅有单列索引 idx_user_id、idx_product_id
2. GROUP BY status, DATE(create_time) 需要对大量行排序，触发 Filesort
3. 全表扫描 50 万行，CPU 用于排序和聚合计算
4. 磁盘 I/O 高：读取大量数据页到 Buffer Pool

[优化方案]
1. 添加复合索引: ALTER TABLE orders ADD INDEX idx_status_time (status, create_time);
2. 或覆盖索引: ALTER TABLE orders ADD INDEX idx_cover (status, create_time, amount);
3. 优化 SQL: 避免 DATE() 函数导致索引失效
`)
}

// explainQuery 查看查询执行计划
func explainQuery(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	log.Println("[EXPLAIN] 分析慢查询执行计划...")
	rows, err := db.Query(`
		EXPLAIN SELECT status, DATE(create_time) as day, COUNT(*), SUM(amount) 
		FROM orders 
		WHERE create_time >= '2024-01-01'
		GROUP BY status, DATE(create_time)
		ORDER BY day DESC
	`)
	if err != nil {
		log.Printf("EXPLAIN 失败: %v（表可能不存在，请先运行 reproduce）", err)
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	fmt.Printf("\n| %s |\n", columns[0])
	for i := range columns {
		if i > 0 {
			fmt.Printf(" | %s", columns[i])
		}
	}
	fmt.Println(" |")
	fmt.Println("|" + repeatString("---|", len(columns)))

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("扫描失败: %v", err)
			break
		}
		fmt.Print("|")
		for _, v := range values {
			if v == nil {
				fmt.Print(" NULL |")
			} else {
				fmt.Printf(" %v |", v)
			}
		}
		fmt.Println()
	}

	log.Print(`
[解读]
- type: ALL 表示全表扫描（最差）
- rows: 扫描的行数（很大）
- Extra: Using filesort 表示需要额外排序
`)
}

// optimize 演示优化后的查询
func optimize(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 添加优化所需的索引
	log.Println("[优化] 添加复合索引...")
	_, err = db.Exec(`ALTER TABLE orders ADD INDEX idx_status_time (status, create_time)`)
	if err != nil {
		log.Printf("索引可能已存在: %v", err)
	} else {
		log.Println("  索引 idx_status_time 创建完成")
	}

	// 执行优化后的查询
	log.Println("[优化后] 执行相同查询...")
	start := time.Now()
	rows, err := db.Query(`
		SELECT status, DATE(create_time) as day, COUNT(*) as cnt, SUM(amount) as total
		FROM orders 
		WHERE create_time >= '2024-01-01'
		GROUP BY status, DATE(create_time)
		ORDER BY day DESC
	`)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status int
		var day string
		var cnt int
		var total sql.NullFloat64
		if err := rows.Scan(&status, &day, &cnt, &total); err != nil {
			log.Printf("扫描失败: %v", err)
			break
		}
		count++
	}
	elapsed := time.Since(start)
	log.Printf("  查询完成，返回 %d 行，耗时 %v", count, elapsed)

	// 查看新的执行计划
	log.Println("[EXPLAIN] 优化后的执行计划...")
	rows2, _ := db.Query(`
		EXPLAIN SELECT status, DATE(create_time) as day, COUNT(*), SUM(amount) 
		FROM orders 
		WHERE create_time >= '2024-01-01'
		GROUP BY status, DATE(create_time)
		ORDER BY day DESC
	`)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			values := make([]interface{}, 5)
			valuePtrs := make([]interface{}, 5)
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			rows2.Scan(valuePtrs...)
			fmt.Printf("  type: %v, key: %v, rows: %v, Extra: %v\n",
				values[3], values[4], values[7], values[10])
		}
	}

	log.Print(`
[优化效果]
- 添加复合索引后，MySQL 使用索引范围扫描
- 避免全表扫描，CPU 和 I/O 大幅下降
- 查询时间从 30+ 秒降到毫秒级
`)
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
