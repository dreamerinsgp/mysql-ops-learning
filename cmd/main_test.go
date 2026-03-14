package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd", "run")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected exit 1 when no args")
	}
	if !strings.Contains(string(out), "Usage") {
		t.Errorf("expected usage in output, got: %s", out)
	}
}

func TestMainUnknownProblem(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd", "run", "99-unknown")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "MYSQL_DSN=invalid_dsn_for_test")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected exit 1 for unknown problem")
	}
	if !strings.Contains(string(out), "Unknown problem") {
		t.Errorf("expected unknown problem message, got: %s", out)
	}
}

func TestMainWrongSubcmd(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd", "build", "01-max-connections")
	cmd.Dir = ".."
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "Usage") {
		t.Errorf("expected usage for wrong subcmd, got: %s", out)
	}
}
