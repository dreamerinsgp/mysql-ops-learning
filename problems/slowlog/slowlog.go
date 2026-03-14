package slowlog

import (
	"fmt"
	"log"
	"os"

	"mysql-ops-learning/pkg/db"
)

// Run executes the slow-log problem tool: reproduce or enable
func Run(action string) {
	if os.Getenv("MYSQL_DSN") == "" {
		log.Fatal("MYSQL_DSN not set")
	}

	switch action {
	case "reproduce":
		reproduce()
	case "enable":
		enable()
	default:
		log.Fatalf("Unknown action: %s (use reproduce or enable)", action)
	}
}

func reproduce() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	log.Println("Running slow query: SELECT SLEEP(5)...")
	_, err = database.Exec("SELECT SLEEP(5)")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Done. If slow_query_log is ON and long_query_time <= 5, this should appear in slow log.")
}

func enable() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	_, err = database.Exec("SET GLOBAL slow_query_log = 'ON'")
	if err != nil {
		log.Printf("Warning: SET GLOBAL slow_query_log failed (may need SUPER): %v", err)
	} else {
		log.Println("slow_query_log = ON")
	}
	_, err = database.Exec("SET GLOBAL long_query_time = 2")
	if err != nil {
		log.Printf("Warning: SET GLOBAL long_query_time failed: %v", err)
	} else {
		log.Println("long_query_time = 2 seconds")
	}

	// Show current values
	var name, val string
	row := database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'slow_query_log'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("slow_query_log: %s\n", val)
	}
	row = database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'long_query_time'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("long_query_time: %s\n", val)
	}
	row = database.QueryRow("SHOW GLOBAL VARIABLES LIKE 'slow_query_log_file'")
	if err := row.Scan(&name, &val); err == nil {
		fmt.Printf("slow_query_log_file: %s\n", val)
	}
}
