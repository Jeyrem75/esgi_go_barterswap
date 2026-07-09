package main

import (
	"context"
	"database/sql"
	"fmt"
)

func insertReview(ctx context.Context, db *sql.DB, r Review) (*Review, error) {
	row := db.QueryRowContext(ctx,
		`INSERT INTO reviews (exchange_id, author_id, target_id, note, commentaire)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id, created_at`,
		r.ExchangeID, r.AuthorID, r.TargetID, r.Note, r.Commentaire)
	if err := row.Scan(&r.ID, &r.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("avis déjà déposé pour l'échange %d: %w", r.ExchangeID, ErrValidation)
		}
		return nil, err
	}
	return &r, nil
}

func selectUserReviews(ctx context.Context, db *sql.DB, userID int) ([]Review, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, exchange_id, author_id, target_id, note, commentaire, created_at
         FROM reviews WHERE target_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reviews []Review
	for rows.Next() {
		var rv Review
		if err := rows.Scan(&rv.ID, &rv.ExchangeID, &rv.AuthorID, &rv.TargetID, &rv.Note, &rv.Commentaire, &rv.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}

func selectServiceReviews(ctx context.Context, db *sql.DB, serviceID int) ([]Review, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT r.id, r.exchange_id, r.author_id, r.target_id, r.note, r.commentaire, r.created_at
        FROM reviews r JOIN exchanges e ON e.id = r.exchange_id
        WHERE e.service_id = $1 ORDER BY r.created_at DESC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reviews []Review
	for rows.Next() {
		var rv Review
		if err := rows.Scan(&rv.ID, &rv.ExchangeID, &rv.AuthorID, &rv.TargetID, &rv.Note, &rv.Commentaire, &rv.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}
