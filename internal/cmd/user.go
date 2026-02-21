package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/PauloHFS/goth/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func RunCreateUser() {
	if len(os.Args) < 4 {
		os.Stderr.WriteString("Usage: create-user <email> <password>\n")
		os.Exit(1)
	}
	email := os.Args[2]
	password := os.Args[3]

	dbConn, err := initDB()
	if err != nil {
		panic(err)
	}
	defer dbConn.Close()
	queries := db.New(dbConn)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	_, err = queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("failed to create user: %v\n", err))
		os.Exit(1)
	}
	os.Stdout.WriteString(fmt.Sprintf("User %s created successfully\n", email))
}
