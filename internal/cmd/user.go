package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/PauloHFS/goth/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func RunCreateUser() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: create-user <email> <password>")
		os.Exit(1)
	}
	email := os.Args[2]
	password := os.Args[3]

	dbConn, err := initDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()
	queries := db.New(dbConn)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	fmt.Printf("User %s created successfully\n", email)
	return nil
}
