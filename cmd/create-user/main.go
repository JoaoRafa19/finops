package main

import (
	"context"
	"errors"
	"finops/internal/app"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	email := flag.String("email", "", "user email")
	password := flag.String("password", "", "user password")
	isAdmin := flag.Bool("admin", false, "create user as admin")
	flag.Parse()

	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: -email")
		os.Exit(1)
	}
	if strings.TrimSpace(*password) == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: -password")
		os.Exit(1)
	}
	if len(*password) < 6 {
		fmt.Fprintln(os.Stderr, "password must have at least 8 characters")
		os.Exit(1)
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	cfg := app.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := app.NewDB(ctx, cfg.DbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}

	const query = `
		INSERT INTO users (email, password_hash, password_algo, is_admin)
		VALUES ($1, $2, 'bcrypt', $3)
		RETURNING id
	`

	var userID int64
	err = db.QueryRowContext(ctx, query, normalizedEmail, string(hash), *isAdmin).Scan(&userID)
	if err != nil {
		if isDuplicateEmail(err) {
			fmt.Fprintf(os.Stderr, "user with email %q already exists\n", normalizedEmail)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("user created successfully: id=%d email=%s admin=%t\n", userID, normalizedEmail, *isAdmin)
}

func isDuplicateEmail(err error) bool {
	var sqlErr interface{ SQLState() string }
	if errors.As(err, &sqlErr) {
		return sqlErr.SQLState() == "23505"
	}

	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
