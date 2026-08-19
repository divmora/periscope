package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/divmora/periscope/internal/store"
)

var (
	username string
	email    string
	password string
	role     string
	status   string
)

var makeUserCmd = &cobra.Command{
	Use:   "make:user",
	Short: "Create a new user in the database",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize the database connection
		if err := store.Connect(context.Background()); err != nil {
			log.Fatalf("Failed to connect to database: %v\n", err)
		}
		defer store.Pool.Close()

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v\n", err)
		}

		_, err = store.Pool.Exec(context.Background(), `
			INSERT INTO users (username, email, password_hash, global_role, status)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (username) DO NOTHING
		`, username, email, string(hashedPassword), role, status)

		if err != nil {
			log.Fatalf("Failed to create user: %v\n", err)
		}

		fmt.Printf("Successfully created user: %s (Role: %s, Status: %s)\n", username, role, status)
	},
}

func init() {
	makeUserCmd.Flags().StringVarP(&username, "username", "u", "admin", "Username")
	makeUserCmd.Flags().StringVarP(&email, "email", "e", "admin@example.com", "Email")
	makeUserCmd.Flags().StringVarP(&password, "password", "p", "password", "Password")
	makeUserCmd.Flags().StringVarP(&role, "role", "r", "admin", "Global Role")
	makeUserCmd.Flags().StringVarP(&status, "status", "s", "active", "Account Status")

	rootCmd.AddCommand(makeUserCmd)
}
