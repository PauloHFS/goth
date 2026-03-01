package main

import (
	"fmt"
	"os"

	"github.com/PauloHFS/goth/internal/cmd"
	"github.com/PauloHFS/goth/web/static/assets"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		if err := cmd.RunServer(assets.FS); err != nil {
			fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "server":
		if err := cmd.RunServer(assets.FS); err != nil {
			fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
			os.Exit(1)
		}
	case "seed":
		if err := cmd.RunSeed(); err != nil {
			fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		if err := cmd.RunMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "migrate failed: %v\n", err)
			os.Exit(1)
		}
	case "create-user":
		if err := cmd.RunCreateUser(); err != nil {
			fmt.Fprintf(os.Stderr, "create-user failed: %v\n", err)
			os.Exit(1)
		}
	case "help":
		showHelp()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("GOTH Stack - Single Binary Console")
	fmt.Println("Usage: ./goth [command] [args]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  server       Start the web server (default)")
	fmt.Println("  migrate      Run database migrations")
	fmt.Println("  seed         Run migrations and seed the database")
	fmt.Println("  create-user  Create a new user (args: <email> <password>)")
	fmt.Println("  help         Show this help message")
}
