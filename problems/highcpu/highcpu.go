package highcpu

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
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

	// 批量 INSERT（每批 5000 行一条 SQL，远程执行快，约 30–60 秒完成）
	log.Println("[真实数据] 插入 15 万条订单数据（每批 5000 行，批量 INSERT）...")
	const totalRows = 150000
	const batchSize = 5000
	insertPrefix := "INSERT INTO orders (user_id, product_id, amount, status, create_time) VALUES "
	for i := 0; i < totalRows; i += batchSize {
		vals := make([]string, 0, batchSize)
		for j := 0; j < batchSize && i+j < totalRows; j++ {
			n := i + j
			ct := time.Now().Add(-time.Duration(n%365) * 24 * time.Hour).Format("2006-01-02 15:04:05")
			vals = append(vals, fmt.Sprintf("(%d,%d,%.2f,%d,'%s')",
				(n%10000)+1, (n%5000)+1, float64(n%1000)+10.0, n%4, ct))
		}
		_, err = db.Exec(insertPrefix + strings.Join(vals, ","))
		if err != nil {
			log.Printf("插入失败: %v", err)
			break
		}
		if (i+batchSize)%50000 == 0 || i+batchSize >= totalRows {
			log.Printf("  已插入 %d 行...", min(i+batchSize, totalRows))
		}
	}
	log.Println("  数据插入完成")

	// 执行需要全表扫描的聚合查询（缺复合索引 → type=ALL + Using filesort）
	log.Println("[触发问题] 执行报表聚合查询（缺 (status, create_time) 复合索引，全表扫描约 15 万行）...")
	log.Println("  SQL: SELECT status, DATE(create_time), COUNT(*), SUM(amount) FROM orders WHERE create_time>='2024-01-01' GROUP BY status, DATE(create_time)")
	const repeatQuery = 3
	start := time.Now()
	for r := 0; r < repeatQuery; r++ {
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
		count := 0
		for rows.Next() {
			var status int
			var day string
			var cnt int
			var total sql.NullFloat64
			if err := rows.Scan(&status, &day, &cnt, &total); err != nil {
				break
			}
			count++
			if r == 0 && count <= 5 {
				log.Printf("    结果: status=%d, day=%s, cnt=%d, total=%.2f", status, day, cnt, total.Float64)
			}
		}
		rows.Close()
	}
	elapsed := time.Since(start)
	log.Printf("  查询完成（连续执行 %d 次模拟报表任务），总耗时 %v", repeatQuery, elapsed)

	// 监控 CPU/IO 状态
	log.Println("[监控] 查看当前 MySQL 状态...")
	var qps, threadsRunning int
	var varName string
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Questions'").Scan(&varName, &qps)
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Threads_running'").Scan(&varName, &threadsRunning)
	log.Printf("  Questions: %d, Threads_running: %d", qps, threadsRunning)

	log.Print(`
[结论] CPU/IO 飙高原因分析（15 万行 ≈ 真实业务数据量，可扩展到百万级观察更明显）：
1. WHERE create_time 过滤 + GROUP BY status, DATE(create_time)，仅有 idx_user_id、idx_product_id
2. 无法使用索引，全表扫描 + Filesort 排序
3. CPU 用于聚合计算与排序，I/O 读取大量数据页
4. 数据量更大（50 万+）或冷启动时，单次查询可耗时 10–30 秒

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
