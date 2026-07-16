package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func insertUser(ctx context.Context, db *sql.DB, u User) (*User, error) {
	const welcomeCredits = 10
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`INSERT INTO users (pseudo, bio, ville)
         VALUES ($1, $2, $3)
         RETURNING id, created_at`,
		u.Pseudo, u.Bio, u.Ville)
	if err := row.Scan(&u.ID, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("insertion user: %w", err)
	}
	if err := recordWelcomeCredit(ctx, tx, u.ID, welcomeCredits); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
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

func replaceSkillsRows(ctx context.Context, db *sql.DB, userID int, skills []Skill) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM skills WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, s := range skills {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO skills (user_id, nom, niveau) VALUES ($1, $2, $3)`,
			userID, s.Nom, s.Niveau); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func selectSkills(ctx context.Context, db *sql.DB, userID int) ([]Skill, error) {
	rows, err := db.QueryContext(ctx, `SELECT nom, niveau FROM skills WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skills []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.Nom, &s.Niveau); err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}
