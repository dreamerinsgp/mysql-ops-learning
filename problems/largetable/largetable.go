package largetable

import (
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the large-table problem tool: reproduce or analyze
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "analyze":
		analyze()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or analyze)", action)
	}
}

// reproduce 模拟业务场景：电商订单表随业务增长至 10 万行，后续 ALTER 加字段会长时间锁表
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
			amount DECIMAL(10,2) NOT NULL,
			status TINYINT DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[业务场景] 电商订单表 orders，模拟运营一年后数据量增长...")
	log.Println("插入 100000 行订单（实际可能达千万级，ALTER ADD COLUMN 会锁表数小时）")

	const batch = 5000
	for i := 0; i < 100000; i += batch {
		tx, _ := database.Begin()
		for j := 0; j < batch && i+j < 100000; j++ {
			tx.Exec("INSERT INTO orders (user_id, amount, status) VALUES (?, ?, ?)",
				(i+j)%10000, 99.9+float64(j%100), 1)
		}
		tx.Commit()
		if (i/batch)%5 == 0 {
			log.Printf("  已插入 %d 行", i+batch)
		}
	}
	log.Println("完毕。大表 ALTER 需考虑在线 DDL 或 pt-osc，避免停服。")
}

// analyze 查询 information_schema.TABLES，查看各表数据量、索引占用（如 orders 表体积）
func analyze() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	rows, err := database.Query(`
		SELECT table_schema, table_name, table_rows, 
		       data_length, index_length, 
		       data_length + index_length as total_length,
		       engine
		FROM information_schema.TABLES
		WHERE table_schema = DATABASE()
		ORDER BY (data_length + index_length) DESC
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Table sizes (current database):")
	fmt.Printf("%-30s %12s %12s %12s %10s\n", "Table", "Rows", "Data", "Index", "Engine")
	fmt.Println("--------------------------------------------------------------------------------")

	for rows.Next() {
		var schema, name, engine string
		var tableRows, dataLen, indexLen, totalLen *uint64
		if err := rows.Scan(&schema, &name, &tableRows, &dataLen, &indexLen, &totalLen, &engine); err != nil {
			continue
		}
		rowsVal := uint64(0)
		if tableRows != nil {
			rowsVal = *tableRows
		}
		dataVal := uint64(0)
		if dataLen != nil {
			dataVal = *dataLen
		}
		idxVal := uint64(0)
		if indexLen != nil {
			idxVal = *indexLen
		}
		totalVal := dataVal + idxVal
		fmt.Printf("%-30s %12d %12s %12s %10s\n",
			name, rowsVal,
			formatBytes(dataVal), formatBytes(idxVal), engine)
		_ = totalVal
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
