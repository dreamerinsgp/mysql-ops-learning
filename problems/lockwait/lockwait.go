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

	// Create table
	_, _ = db1.Exec("DROP TABLE IF EXISTS _ops_learn_lockwait")
	_, err = db1.Exec(`
		CREATE TABLE _ops_learn_lockwait (id INT PRIMARY KEY, v INT)
	`)
	if err != nil {
		log.Fatal(err)
	}
	db1.Exec("INSERT INTO _ops_learn_lockwait VALUES (1, 0)")

	// Reduce timeout for demo (default 50s is long)
	db1.Exec("SET SESSION innodb_lock_wait_timeout = 5")
	db2.Exec("SET SESSION innodb_lock_wait_timeout = 5")

	var wg sync.WaitGroup
	wg.Add(2)

	// Conn1: hold lock
	go func() {
		defer wg.Done()
		tx, _ := db1.BeginTx(context.Background(), nil)
		tx.Exec("UPDATE _ops_learn_lockwait SET v = 1 WHERE id = 1")
		log.Println("Conn1: holding lock for 10s...")
		time.Sleep(10 * time.Second)
		tx.Commit()
		log.Println("Conn1: released")
	}()

	time.Sleep(500 * time.Millisecond) // ensure conn1 gets lock first

	// Conn2: wait for lock (will timeout)
	go func() {
		defer wg.Done()
		log.Println("Conn2: trying to acquire lock (will wait then timeout)...")
		_, err := db2.Exec("UPDATE _ops_learn_lockwait SET v = 2 WHERE id = 1")
		if err != nil {
			log.Printf("Conn2: %v (expected: Lock wait timeout exceeded)", err)
			return
		}
		log.Println("Conn2: acquired (unexpected)")
	}()

	wg.Wait()
	log.Println("Done.")
}
