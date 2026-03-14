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

func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	_, _ = database.Exec("DROP TABLE IF EXISTS _ops_learn_index")
	_, err = database.Exec(`
		CREATE TABLE _ops_learn_index (
			id INT PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(100) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Created _ops_learn_index (no index on email). Inserting 50000 rows...")

	for i := 0; i < 50000; i += 5000 {
		tx, _ := database.Begin()
		for j := 0; j < 5000 && i+j < 50000; j++ {
			tx.Exec("INSERT INTO _ops_learn_index (email) VALUES (?)", fmt.Sprintf("user%d@test.com", i+j))
		}
		tx.Commit()
	}
	log.Println("Done. Query: SELECT * FROM _ops_learn_index WHERE email = 'user25000@test.com' (will full scan)")
}

func explain() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	rows, err := database.Query("EXPLAIN SELECT * FROM _ops_learn_index WHERE email = 'user25000@test.com'")
	if err != nil {
		log.Fatalf("EXPLAIN failed (run reproduce first to create table): %v", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	fmt.Println("EXPLAIN output:")
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
	fmt.Println("\nNote: type=ALL means full table scan. Add: CREATE INDEX idx_email ON _ops_learn_index(email);")
}
