package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func insertExchangeRow(ctx context.Context, q Querier, serviceID, requesterID, providerID int) (*Exchange, error) {
	var e Exchange
	row := q.QueryRowContext(ctx,
		`INSERT INTO exchanges (service_id, requester_id, owner_id, status)
         VALUES ($1, $2, $3, 'pending')
         RETURNING id, created_at, updated_at`,
		serviceID, requesterID, providerID)
	e.ServiceID, e.RequesterID, e.OwnerID, e.Status = serviceID, requesterID, providerID, "pending"
	if err := row.Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

// selectExchangeForUpdate verrouille la ligne (FOR UPDATE) — utilisé uniquement à
// l'intérieur d'une transaction, avant une transition de statut.
func selectExchangeForUpdate(ctx context.Context, q Querier, id int) (*Exchange, error) {
	var e Exchange
	row := q.QueryRowContext(ctx,
		`SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
         FROM exchanges WHERE id = $1 FOR UPDATE`, id)
	err := row.Scan(&e.ID, &e.ServiceID, &e.RequesterID, &e.OwnerID, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("échange %d: %w", id, ErrNotFound)
	}
	return &e, err
}

func selectExchange(ctx context.Context, db *sql.DB, id int) (*Exchange, error) {
	var e Exchange
	err := db.QueryRowContext(ctx, `
        SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
        FROM exchanges WHERE id = $1`, id).
		Scan(&e.ID, &e.ServiceID, &e.RequesterID, &e.OwnerID, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("échange %d: %w", id, ErrNotFound)
	}
	return &e, err
}

func selectExchangesForUser(ctx context.Context, db *sql.DB, userID int, status string) ([]Exchange, error) {
	query := `SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
              FROM exchanges WHERE (requester_id = $1 OR owner_id = $1)`
	args := []any{userID}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	query += " ORDER BY created_at DESC"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var exchanges []Exchange
	for rows.Next() {
		var e Exchange
		if err := rows.Scan(&e.ID, &e.ServiceID, &e.RequesterID, &e.OwnerID, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		exchanges = append(exchanges, e)
	}
	return exchanges, rows.Err()
}

func updateExchangeStatus(ctx context.Context, tx *sql.Tx, id int, status string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE exchanges SET status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}
