package main

import (
	"context"
	"database/sql"
	"fmt"
)

func RecordCreditTransaction(ctx context.Context, tx *sql.Tx, userID, exchangeID, montant int, txType string) error {
	return recordTransaction(ctx, tx, userID, sql.NullInt64{Int64: int64(exchangeID), Valid: true}, montant, txType)
}

// recordWelcomeCredit journalise le bonus de bienvenue avec exchange_id NULL (colonne nullable).
func recordWelcomeCredit(ctx context.Context, tx *sql.Tx, userID, montant int) error {
	return recordTransaction(ctx, tx, userID, sql.NullInt64{}, montant, "earn")
}

func recordTransaction(ctx context.Context, tx *sql.Tx, userID int, exchangeID sql.NullInt64, montant int, txType string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_transactions (user_id, exchange_id, montant, type) VALUES ($1,$2,$3,$4)`,
		userID, exchangeID, montant, txType); err != nil {
		return fmt.Errorf("insertion transaction: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE users SET credit_balance = credit_balance + $1 WHERE id = $2 AND credit_balance + $1 >= 0`,
		montant, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("solde insuffisant pour ce mouvement: %w", ErrValidation)
	}
	return nil
}
