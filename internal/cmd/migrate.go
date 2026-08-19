package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // Import the pgx driver
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/divmora/periscope/internal/store"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [up|down|status]",
	Short: "Run database migrations",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		command := args[0]

		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			host,
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		)

		db, err := sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v\n", err)
		}
		defer db.Close()

		goose.SetBaseFS(store.EmbedMigrations)

		if err := goose.SetDialect("postgres"); err != nil {
			log.Fatalf("Failed to set goose dialect: %v\n", err)
		}

		switch command {
		case "up":
			if err := goose.Up(db, "migrations"); err != nil {
				log.Fatalf("goose up failed: %v\n", err)
			}
		case "down":
			if err := goose.Down(db, "migrations"); err != nil {
				log.Fatalf("goose down failed: %v\n", err)
			}
		case "status":
			if err := goose.Status(db, "migrations"); err != nil {
				log.Fatalf("goose status failed: %v\n", err)
			}
		default:
			log.Fatalf("Unknown command %q. Allowed: up, down, status\n", command)
		}

		fmt.Println("Migration command completed.")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
