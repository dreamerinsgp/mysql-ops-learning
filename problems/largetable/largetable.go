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

func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	_, _ = database.Exec("DROP TABLE IF EXISTS _ops_learn_largetable")
	_, err = database.Exec(`
		CREATE TABLE _ops_learn_largetable (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			data VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Created _ops_learn_largetable. Inserting 100000 rows...")

	// Batch insert
	const batch = 5000
	for i := 0; i < 100000; i += batch {
		tx, _ := database.Begin()
		for j := 0; j < batch && i+j < 100000; j++ {
			tx.Exec("INSERT INTO _ops_learn_largetable (data) VALUES (?)", fmt.Sprintf("row_%d", i+j))
		}
		tx.Commit()
		if (i/batch)%5 == 0 {
			log.Printf("  Inserted %d rows", i+batch)
		}
	}
	log.Println("Done. 100000 rows inserted.")
}

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
