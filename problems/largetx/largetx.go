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

func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Create table
	_, err = database.Exec(`DROP TABLE IF EXISTS _ops_learn_largetx`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = database.Exec(`
		CREATE TABLE _ops_learn_largetx (
			id INT PRIMARY KEY AUTO_INCREMENT,
			v INT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Created _ops_learn_largetx")

	// Insert 10000 rows
	log.Println("Inserting 10000 rows in one transaction...")
	tx, err := database.Begin()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		_, err = tx.Exec("INSERT INTO _ops_learn_largetx (v) VALUES (?)", i)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Println("Insert done. Now updating all 10000 rows in ONE transaction (simulates bad pattern)...")
	tx, err = database.Begin()
	if err != nil {
		log.Fatal(err)
	}
	res, err := tx.Exec("UPDATE _ops_learn_largetx SET v = v + 1")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	n, _ := res.RowsAffected()
	log.Printf("Updated %d rows. Committing...", n)
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Println("Done. Large transaction completed.")
}

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
