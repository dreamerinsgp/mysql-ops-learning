package deadlock

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"mysql-ops-learning/pkg/db"
)

// Run executes the deadlock problem tool: reproduce or analyze
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

// reproduce 模拟业务场景：用户 A 与用户 B 几乎同时互相转账，加锁顺序相反导致死锁
func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// 建表：用户账户表
	_, _ = database.Exec("DROP TABLE IF EXISTS accounts")
	_, err = database.Exec(`
		CREATE TABLE accounts (
			user_id INT PRIMARY KEY,
			balance DECIMAL(14,2) NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	database.Exec("INSERT INTO accounts (user_id, balance) VALUES (1, 1000), (2, 1000)")

	log.Println("[业务场景] 钱包应用：用户 A(1) 转给用户 B(2) 100 元，同时用户 B(2) 转给用户 A(1) 50 元")
	log.Println("两个事务加锁顺序相反（A→B vs B→A），可能触发死锁，一方回滚后用户需重试")

	var wg sync.WaitGroup
	wg.Add(2)

	// 事务1：A 转 B（先锁 A 的账户行，再锁 B 的账户行）
	go func() {
		defer wg.Done()
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			log.Printf("Tx1 开启失败: %v", err)
			return
		}
		tx.Exec("UPDATE accounts SET balance = balance - 100 WHERE user_id = 1")
		tx.Exec("UPDATE accounts SET balance = balance + 100 WHERE user_id = 2")
		if err := tx.Commit(); err != nil {
			log.Printf("Tx1 (A→B): %v （可能被死锁回滚）", err)
		}
	}()

	// 事务2：B 转 A（先锁 B 的账户行，再锁 A 的账户行）—— 加锁顺序相反
	go func() {
		defer wg.Done()
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			log.Printf("Tx2 开启失败: %v", err)
			return
		}
		tx.Exec("UPDATE accounts SET balance = balance - 50 WHERE user_id = 2")
		tx.Exec("UPDATE accounts SET balance = balance + 50 WHERE user_id = 1")
		if err := tx.Commit(); err != nil {
			log.Printf("Tx2 (B→A): %v （可能被死锁回滚）", err)
		}
	}()

	wg.Wait()
	log.Println("完毕。统一按 user_id 升序加锁可避免死锁。")
}

// analyze 输出 SHOW ENGINE INNODB STATUS，查看 LATEST DETECTED DEADLOCK 详情
func analyze() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	var typ, name, status string
	row := database.QueryRow("SHOW ENGINE INNODB STATUS")
	if err := row.Scan(&typ, &name, &status); err != nil {
		log.Fatalf("SHOW ENGINE INNODB STATUS failed: %v", err)
	}
	fmt.Println("--- InnoDB Status (check LATEST DETECTED DEADLOCK if any) ---")
	fmt.Println(status)
}
