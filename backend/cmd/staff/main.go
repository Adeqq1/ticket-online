package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/platform"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(os.Args[1:]); err != nil {
		logger.Error("staff command failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "bootstrap-admin" {
		return fmt.Errorf("usage: ticket-staff bootstrap-admin --name NAME --email EMAIL < password-stdin")
	}
	flags := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
	name := flags.String("name", "", "administrator name")
	email := flags.String("email", "", "administrator email")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	password, err := io.ReadAll(io.LimitReader(os.Stdin, 131))
	if err != nil {
		return fmt.Errorf("read password from stdin: %w", err)
	}
	if len(password) == 131 {
		return fmt.Errorf("password input is too long")
	}
	password = []byte(strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r"))
	if err := staffauth.ValidatePassword(string(password)); err != nil {
		return err
	}
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return fmt.Errorf("MYSQL_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := platform.OpenDatabase(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrations.Run(ctx, db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if err := staffauth.New(db).BootstrapAdmin(ctx, *name, *email, string(password)); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Administrator created.")
	return nil
}
