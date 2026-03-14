package lockwait

import (
	"context"
	"database/sql"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"mysql-ops-learning/pkg/db"
)

// Run executes the lock-wait problem tool: reproduce
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	default:
		log.Fatalf("Unknown action: %s (use reproduce)", action)
	}
}

// reproduce 模拟业务场景：后台导出用户报表持锁不释放，前台用户修改头像/昵称被阻塞直至超时
func reproduce() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN not set")
	}
	db1, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer db1.Close()

	db2, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db2.Close()
	if err := db2.Ping(); err != nil {
		log.Fatal(err)
	}

	// 建表：用户表
	_, _ = db1.Exec("DROP TABLE IF EXISTS users")
	_, err = db1.Exec(`
		CREATE TABLE users (
			id INT PRIMARY KEY,
			nickname VARCHAR(50),
			avatar VARCHAR(200)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	db1.Exec("INSERT INTO users (id, nickname, avatar) VALUES (1, 'test_user', 'default.png')")

	db1.Exec("SET SESSION innodb_lock_wait_timeout = 5")
	db2.Exec("SET SESSION innodb_lock_wait_timeout = 5")

	log.Println("[业务场景] SaaS 平台：后台导出全部用户报表，持锁 10 秒不提交；前台用户同时修改头像")

	var wg sync.WaitGroup
	wg.Add(2)

	// 连接1：模拟后台导出脚本，开启事务并持锁（模拟导出时对 users 加锁处理）
	go func() {
		defer wg.Done()
		tx, _ := db1.BeginTx(context.Background(), nil)
		tx.Exec("UPDATE users SET nickname = nickname WHERE id = 1") // 持排他锁
		holdDur := 10 * time.Second
		if os.Getenv("MYSQL_OPS_CI") == "1" {
			holdDur = 2 * time.Second // CI 冒烟测试缩短持锁时间
		}
		log.Println("后台：持锁中（模拟导出处理）...")
		time.Sleep(holdDur)
		tx.Commit()
		log.Println("后台：释放锁")
	}()

	time.Sleep(500 * time.Millisecond)

	// 连接2：模拟前台用户修改头像
	go func() {
		defer wg.Done()
		log.Println("前台用户：尝试更新头像（需等待后台释放锁）...")
		_, err := db2.Exec("UPDATE users SET avatar = 'new.png' WHERE id = 1")
		if err != nil {
			log.Printf("前台用户：%v（Lock wait timeout exceeded，用户看到「修改失败，请重试」）", err)
			return
		}
		log.Println("前台用户：更新成功")
	}()

	wg.Wait()
	log.Println("完毕。应缩短持锁时间，或导出使用只读从库，避免阻塞前台写入。")
}
