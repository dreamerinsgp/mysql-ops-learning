package main

import (
	"fmt"
	"os"

	"mysql-ops-learning/problems/conn"
	"mysql-ops-learning/problems/deadlock"
	"mysql-ops-learning/problems/highcpu"
	"mysql-ops-learning/problems/indexmisuse"
	"mysql-ops-learning/problems/largetable"
	"mysql-ops-learning/problems/largetx"
	"mysql-ops-learning/problems/lockwait"
	"mysql-ops-learning/problems/replicationlag"
	"mysql-ops-learning/problems/slowlog"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}
	subcmd := os.Args[1] // run
	problem := os.Args[2] // 01-max-connections, etc.
	action := ""
	if len(os.Args) >= 4 {
		action = os.Args[3]
	}

	if subcmd != "run" {
		printUsage()
		os.Exit(1)
	}

	switch problem {
	case "01-max-connections":
		conn.Run(action)
	case "02-slow-log":
		slowlog.Run(action)
	case "03-large-transaction":
		largetx.Run(action)
	case "04-large-table":
		largetable.Run(action)
	case "05-deadlock":
		deadlock.Run(action)
	case "06-lock-wait-timeout":
		lockwait.Run(action)
	case "07-index-misuse":
		indexmisuse.Run(action)
	case "08-replication-lag":
		replicationlag.Run(action)
	case "09-cpu-io-high":
		highcpu.Run(action)
	default:
		fmt.Printf("Unknown problem: %s\n", problem)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: go run ./cmd run <problem> [action]

Problems and actions:
  01-max-connections   reproduce | monitor
  02-slow-log          reproduce | enable | view
  03-large-transaction reproduce | detect
  04-large-table       reproduce | analyze
  05-deadlock          reproduce | analyze
  06-lock-wait-timeout reproduce
  07-index-misuse      reproduce | explain
  08-replication-lag   reproduce | monitor | detect
  09-cpu-io-high       reproduce | explain | optimize

Set MYSQL_DSN env (e.g. from .env): user:pass@tcp(host:3306)/dbname`)
}
