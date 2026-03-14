package db

import (
	"os"
	"testing"
)

func TestOpenEmptyDSN(t *testing.T) {
	orig := os.Getenv("MYSQL_DSN")
	os.Unsetenv("MYSQL_DSN")
	defer func() {
		if orig != "" {
			os.Setenv("MYSQL_DSN", orig)
		}
	}()

	_, err := Open()
	if err == nil {
		t.Fatal("expected error when MYSQL_DSN is empty")
	}
	if err.Error() != "MYSQL_DSN env var not set" {
		t.Errorf("unexpected error: %v", err)
	}
}
