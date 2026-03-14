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

func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Create test tables
	_, _ = database.Exec("DROP TABLE IF EXISTS _ops_learn_deadlock_a")
	_, _ = database.Exec("DROP TABLE IF EXISTS _ops_learn_deadlock_b")
	_, err = database.Exec(`
		CREATE TABLE _ops_learn_deadlock_a (id INT PRIMARY KEY, v INT)
	`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = database.Exec(`
		CREATE TABLE _ops_learn_deadlock_b (id INT PRIMARY KEY, v INT)
	`)
	if err != nil {
		log.Fatal(err)
	}
	database.Exec("INSERT INTO _ops_learn_deadlock_a VALUES (1, 0)")
	database.Exec("INSERT INTO _ops_learn_deadlock_a VALUES (2, 0)")
	database.Exec("INSERT INTO _ops_learn_deadlock_b VALUES (1, 0)")
	database.Exec("INSERT INTO _ops_learn_deadlock_b VALUES (2, 0)")

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: lock A(1) then B(2)
	go func() {
		defer wg.Done()
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			log.Printf("Tx1 begin: %v", err)
			return
		}
		tx.Exec("UPDATE _ops_learn_deadlock_a SET v = v + 1 WHERE id = 1")
		tx.Exec("UPDATE _ops_learn_deadlock_b SET v = v + 1 WHERE id = 2")
		if err := tx.Commit(); err != nil {
			log.Printf("Tx1: %v", err)
		}
	}()

	// Goroutine 2: lock B(1) then A(2) - opposite order, can deadlock
	go func() {
		defer wg.Done()
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			log.Printf("Tx2 begin: %v", err)
			return
		}
		tx.Exec("UPDATE _ops_learn_deadlock_b SET v = v + 1 WHERE id = 1")
		tx.Exec("UPDATE _ops_learn_deadlock_a SET v = v + 1 WHERE id = 2")
		if err := tx.Commit(); err != nil {
			log.Printf("Tx2: %v", err)
		}
	}()

	wg.Wait()
	log.Println("Done. Check for deadlock in tx logs above. One may have been rolled back.")
}

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
