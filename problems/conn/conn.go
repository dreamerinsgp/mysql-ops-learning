package conn

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the max-connections problem tool: reproduce or monitor
func Run(action string) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce(dsn)
	case "monitor":
		monitor()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or monitor)", action)
	}
}

// reproduce 模拟业务场景：电商秒杀时，每个请求新建连接且不归还连接池，直至耗尽
func reproduce(dsn string) {
	var conns []*sql.DB
	log.Println("[业务场景] 模拟大促秒杀：每个请求新建 MySQL 连接且未归还，直至达到 max_connections 上限...")
	for i := 0; ; i++ {
		c, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("达到上限：在 %d 个连接后新建失败: %v（模拟用户看到「Too many connections」）", len(conns), err)
			break
		}
		c.SetMaxOpenConns(1) // 每个连接池仅1连接，模拟「每请求一连接」
		if err := c.Ping(); err != nil {
			log.Printf("Ping 失败（连接数已满）: %v", err)
			c.Close()
			break
		}
		conns = append(conns, c)
		if i%10 == 0 && i > 0 {
			log.Printf("  已建立 %d 个连接（模拟持续涌入的秒杀请求）", len(conns))
		}
	}
	log.Printf("共耗尽 %d 个连接。清理中...", len(conns))
	for _, c := range conns {
		c.Close()
	}
	log.Println("完毕。所有连接已关闭。结论：应使用连接池，避免每请求一连接。")
}

func monitor() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	var tc, muc, maxConn int
	row := database.QueryRow("SHOW GLOBAL STATUS LIKE 'Threads_connected'")
	var name, val string
	if err := row.Scan(&name, &val); err != nil {
		log.Fatal(err)
	}
	fmt.Sscanf(val, "%d", &tc)
	row = database.QueryRow("SHOW GLOBAL STATUS LIKE 'Max_used_connections'")
	if err := row.Scan(&name, &val); err != nil {
		log.Fatal(err)
	}
	fmt.Sscanf(val, "%d", &muc)
	row = database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'max_connections'")
	if err := row.Scan(&name, &val); err != nil {
		log.Fatal(err)
	}
	fmt.Sscanf(val, "%d", &maxConn)

	fmt.Printf("Threads_connected:     %d\n", tc)
	fmt.Printf("Max_used_connections: %d\n", muc)
	fmt.Printf("max_connections:      %d\n", maxConn)
	if maxConn > 0 {
		fmt.Printf("Usage: %d / %d (%.1f%%)\n", tc, maxConn, 100*float64(tc)/float64(maxConn))
	}
}
