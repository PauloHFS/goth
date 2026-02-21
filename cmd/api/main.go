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
		cmd.RunServer(assets.FS)
		return
	}

	switch os.Args[1] {
	case "server":
		cmd.RunServer(assets.FS)
	case "seed":
		cmd.RunSeed()
	case "migrate":
		cmd.RunMigrate()
	case "create-user":
		cmd.RunCreateUser()
	case "help":
		showHelp()
	default:
		os.Stderr.WriteString(fmt.Sprintf("Unknown command: %s\n", os.Args[1]))
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	help := `GOTH Stack - Single Binary Console
Usage: ./goth [command] [args]

Available commands:
  server       Start the web server (default)
  migrate      Run database migrations
  seed         Run migrations and seed the database
  create-user  Create a new user (args: <email> <password>)
  help         Show this help message
`
	os.Stdout.WriteString(help)
}
