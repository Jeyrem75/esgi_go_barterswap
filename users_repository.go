package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func insertUser(ctx context.Context, db *sql.DB, u User) (*User, error) {
	const welcomeCredits = 10
	row := db.QueryRowContext(ctx,
		`INSERT INTO users (pseudo, bio, ville, credit_balance)
         VALUES ($1, $2, $3, $4)
         RETURNING id, created_at`,
		u.Pseudo, u.Bio, u.Ville, welcomeCredits)
	if err := row.Scan(&u.ID, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("insertion user: %w", err)
	}
	u.CreditBalance = welcomeCredits
	return &u, nil
}

func selectUser(ctx context.Context, db *sql.DB, id int) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, pseudo, bio, ville, credit_balance, created_at FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Pseudo, &u.Bio, &u.Ville, &u.CreditBalance, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("utilisateur %d: %w", id, ErrNotFound)
	}
	return &u, err
}

// updateUserRow ne touche jamais credit_balance — les mouvements de crédits passent par credits.go.
func updateUserRow(ctx context.Context, db *sql.DB, u User) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users SET pseudo = $1, bio = $2, ville = $3 WHERE id = $4`,
		u.Pseudo, u.Bio, u.Ville, u.ID)
	return err
}
