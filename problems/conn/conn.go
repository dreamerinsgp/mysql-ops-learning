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

func reproduce(dsn string) {
	var conns []*sql.DB
	log.Println("Opening connections until limit...")
	for i := 0; ; i++ {
		c, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("Failed after %d connections: %v", len(conns), err)
			break
		}
		c.SetMaxOpenConns(1) // each pool = 1 connection to exhaust limit
		if err := c.Ping(); err != nil {
			log.Printf("Ping failed after %d connections: %v", len(conns), err)
			c.Close()
			break
		}
		conns = append(conns, c)
		if i%10 == 0 && i > 0 {
			log.Printf("  %d connections open", len(conns))
		}
	}
	log.Printf("Total: %d connections. Cleaning up...", len(conns))
	for _, c := range conns {
		c.Close()
	}
	log.Println("Done. All connections closed.")
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
