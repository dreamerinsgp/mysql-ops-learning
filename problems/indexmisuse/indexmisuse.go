package indexmisuse

import (
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the index-misuse problem tool: reproduce or explain
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "explain":
		explain()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or explain)", action)
	}
}

// reproduce 模拟业务场景：外卖平台订单表，用户按手机号查订单，phone 无索引导致全表扫描、接口超时
func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	_, _ = database.Exec("DROP TABLE IF EXISTS orders")
	_, err = database.Exec(`
		CREATE TABLE orders (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			user_id BIGINT NOT NULL,
			phone VARCHAR(20) NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[业务场景] 外卖平台：用户输入手机号查订单，orders 表 phone 列无索引 → 全表扫描")
	log.Println("插入 50000 行订单...")

	for i := 0; i < 50000; i += 5000 {
		tx, _ := database.Begin()
		for j := 0; j < 5000 && i+j < 50000; j++ {
			tx.Exec("INSERT INTO orders (user_id, phone, amount) VALUES (?, ?, ?)",
				i+j, fmt.Sprintf("138%08d", (i+j)%100000), 29.9)
		}
		tx.Commit()
	}
	// 确保目标手机号有订单
	database.Exec("INSERT INTO orders (user_id, phone, amount) VALUES (99999, '13800138000', 39.9)")
	log.Println("完毕。查询: SELECT * FROM orders WHERE phone='13800138000' ORDER BY created_at DESC LIMIT 20（将全表扫描）")
}

// explain 展示问题 SQL 的执行计划，type=ALL 表示全表扫描
func explain() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	rows, err := database.Query("EXPLAIN SELECT * FROM orders WHERE phone = '13800138000' ORDER BY created_at DESC LIMIT 20")
	if err != nil {
		log.Fatalf("EXPLAIN 失败（请先执行 reproduce）: %v", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	fmt.Println("EXPLAIN 输出（用户按手机号查订单）:")
	for _, c := range cols {
		fmt.Printf("%-20s ", c)
	}
	fmt.Println()
	fmt.Println("--------------------------------------------------------------------------------")

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		for _, v := range vals {
			fmt.Printf("%-20v ", v)
		}
		fmt.Println()
	}
	fmt.Println("\n结论: type=ALL 表示全表扫描。建议: CREATE INDEX idx_phone ON orders(phone);")
}
