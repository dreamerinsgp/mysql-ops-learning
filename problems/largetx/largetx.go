package largetx

import (
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the large-transaction problem tool: reproduce or detect
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "detect":
		detect()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or detect)", action)
	}
}

// reproduce 模拟业务场景：积分商城周年庆，运营批量给 1 万用户发积分，单事务内更新全部导致长时间持锁
func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// 建表：用户积分表
	_, _ = database.Exec(`DROP TABLE IF EXISTS user_points`)
	_, err = database.Exec(`
		CREATE TABLE user_points (
			user_id BIGINT PRIMARY KEY,
			points INT NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[业务场景] 积分商城周年庆：运营批量给用户发 100 积分")

	// 插入 1 万用户
	log.Println("插入 10000 个用户...")
	tx, _ := database.Begin()
	for i := 1; i <= 10000; i++ {
		tx.Exec("INSERT INTO user_points (user_id, points) VALUES (?, 0)", i)
	}
	tx.Commit()
	log.Println("插入完成。模拟错误写法：单事务内更新全部用户积分（持锁数分钟，阻塞用户登录、下单、查余额）...")

	tx, err = database.Begin()
	if err != nil {
		log.Fatal(err)
	}
	res, err := tx.Exec("UPDATE user_points SET points = points + 100")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	n, _ := res.RowsAffected()
	log.Printf("已更新 %d 行。提交中...", n)
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Println("完毕。应拆分为小批次（每批 500~1000 行提交一次），避免长事务持锁。")
}

// detect 查询 INNODB_TRX，定位持有锁的长事务（如批量发积分的脚本）
func detect() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	rows, err := database.Query(`
		SELECT trx_id, trx_state, trx_started, trx_query, trx_rows_modified, trx_rows_locked
		FROM information_schema.INNODB_TRX
	`)
	if err != nil {
		log.Fatalf("Query INNODB_TRX failed (may need MySQL 5.7+): %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var trxID, state, started string
		var query *string
		var rowsModified, rowsLocked *int
		if err := rows.Scan(&trxID, &state, &started, &query, &rowsModified, &rowsLocked); err != nil {
			continue
		}
		count++
		fmt.Printf("--- Transaction %s ---\n", trxID)
		fmt.Printf("  State: %s, Started: %s\n", state, started)
		if rowsModified != nil {
			fmt.Printf("  Rows modified: %d\n", *rowsModified)
		}
		if rowsLocked != nil {
			fmt.Printf("  Rows locked: %d\n", *rowsLocked)
		}
		if query != nil && *query != "" {
			fmt.Printf("  Query: %s\n", *query)
		}
		fmt.Println()
	}
	if count == 0 {
		fmt.Println("No long-running transactions found.")
	}
}
