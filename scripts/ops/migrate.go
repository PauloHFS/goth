//go:build ignore

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/PauloHFS/goth/migrations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 2 {
		fmt.Println("Usage: goose <command> <driver> <dbpath>")
		os.Exit(1)
	}

	command := args[0]
	driver := args[1]
	dbpath := args[2]

	// Configurar goose para usar embed.FS
	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect(driver); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting dialect: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	switch command {
	case "up":
		if err := goose.UpContext(ctx, db, "."); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully")
	case "down":
		if err := goose.DownContext(ctx, db, "."); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migration down: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migration rolled back successfully")
	case "status":
		if err := goose.StatusContext(ctx, db, "."); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
			os.Exit(1)
		}
	case "reset":
		if err := goose.ResetContext(ctx, db, "."); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations reset successfully")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
